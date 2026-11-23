package store

import (
	"context"
	"database/sql"
	"errors"
	"time"

	sqlcstore "dragz-admin/backend/internal/store/sqlc"
)

const (
	// SubscriptionTrial is a subscription that has not been paid yet.
	SubscriptionTrial = "trial"
	// SubscriptionActive is a paid subscription inside its term.
	SubscriptionActive = "active"
	// SubscriptionOverdue is past trial_ends_at or ended_at and still inside grace.
	SubscriptionOverdue = "overdue"
	// SubscriptionCancelled is terminal. A later plan needs a new row.
	SubscriptionCancelled = "cancelled"
)

// ValidSubscriptionStatus reports whether status is one of the four lifecycle values.
func ValidSubscriptionStatus(status string) bool {
	switch status {
	case SubscriptionTrial, SubscriptionActive, SubscriptionOverdue, SubscriptionCancelled:
		return true
	default:
		return false
	}
}

// ValidSubscriptionTransition reports whether from → to is legal.
//
//	trial    → active | overdue | cancelled
//	active   → overdue | cancelled
//	overdue  → active | cancelled
//	cancelled → (none)
//
// Staying on trial, active, or overdue is allowed so a plan change can keep
// the current status. Cancelled cannot stay put: that needs a new row.
func ValidSubscriptionTransition(from, to string) bool {
	if !ValidSubscriptionStatus(from) || !ValidSubscriptionStatus(to) {
		return false
	}
	if from == to {
		return from != SubscriptionCancelled
	}
	switch from {
	case SubscriptionTrial:
		return to == SubscriptionActive || to == SubscriptionOverdue || to == SubscriptionCancelled
	case SubscriptionActive:
		return to == SubscriptionOverdue || to == SubscriptionCancelled
	case SubscriptionOverdue:
		return to == SubscriptionActive || to == SubscriptionCancelled
	default:
		return false
	}
}

// ReplaceTenantSubscription points the tenant at planID with status.
// An open trial, active, or overdue row is updated in place when the move is
// legal. After cancelled, or when the tenant has no open row, a new row is
// inserted. A missing tenant or plan is ErrNotFound. An illegal move is
// ErrInvalidSubscriptionTransition.
func (r *TenantRepository) ReplaceTenantSubscription(ctx context.Context, tenantID, planID uint64, status string) (uint64, error) {
	if status == "" {
		status = SubscriptionActive
	}
	if !ValidSubscriptionStatus(status) {
		return 0, ErrInvalidSubscriptionTransition
	}
	var id uint64
	err := r.platformTx(ctx, func(ctx context.Context, tx *Tx) error {
		if err := requireTenantAndPlan(ctx, tx, tenantID, planID); err != nil {
			return err
		}
		versionID, snapshot, err := planSnapshotFor(ctx, tx, planID)
		if err != nil {
			return err
		}
		open, err := loadOpenSubscription(ctx, tx, tenantID)
		if errors.Is(err, sql.ErrNoRows) {
			if err := retireOtherActive(ctx, tx, tenantID, 0, status); err != nil {
				return err
			}
			if err := tx.QueryRowContext(ctx, `
INSERT INTO sys_tenant_subscription (
  tenant_id, plan_id, status, grace_ends_at, ended_at,
  plan_version_id, plan_snapshot
) VALUES (
  $1, $2, $3,
  CASE WHEN $3 = 'overdue' THEN NOW() + INTERVAL '7 days' ELSE NULL END,
  CASE WHEN $3 = 'cancelled' THEN NOW() ELSE NULL END,
  $4, $5::jsonb
)
RETURNING id`, tenantID, planID, status, versionID, string(snapshot)).Scan(&id); err != nil {
				return err
			}
			return applySnapshotQuotas(ctx, tx, tenantID, snapshot)
		}
		if err != nil {
			return err
		}
		if !ValidSubscriptionTransition(open.Status, status) {
			return ErrInvalidSubscriptionTransition
		}
		if err := retireOtherActive(ctx, tx, tenantID, open.ID, status); err != nil {
			return err
		}
		if open.PlanID == planID {
			// Same plan: a status change must not re-pin the subscription to
			// a newer catalog version. The tenant keeps what it bought.
			if err := updateSubscriptionRow(ctx, tx, open.ID, planID, status); err != nil {
				return err
			}
			id = open.ID
			return applyPlanQuotas(ctx, sqlcstore.New(tx), tenantID, planID)
		}
		if err := updateSubscriptionRowWithPlan(ctx, tx, open.ID, planID, status, versionID, snapshot); err != nil {
			return err
		}
		id = open.ID
		return applySnapshotQuotas(ctx, tx, tenantID, snapshot)
	})
	return id, err
}

// TransitionSubscription moves the open trial, active, or overdue row to
// status without changing the plan. No open row is ErrNotFound. Cancelled is
// not updated here; ReplaceTenantSubscription inserts the next row.
func (r *TenantRepository) TransitionSubscription(ctx context.Context, tenantID uint64, status string) (uint64, error) {
	if !ValidSubscriptionStatus(status) {
		return 0, ErrInvalidSubscriptionTransition
	}
	var id uint64
	err := r.platformTx(ctx, func(ctx context.Context, tx *Tx) error {
		open, err := loadOpenSubscription(ctx, tx, tenantID)
		if errors.Is(err, sql.ErrNoRows) {
			return ErrNotFound
		}
		if err != nil {
			return err
		}
		if !ValidSubscriptionTransition(open.Status, status) {
			return ErrInvalidSubscriptionTransition
		}
		if err := retireOtherActive(ctx, tx, tenantID, open.ID, status); err != nil {
			return err
		}
		if err := updateSubscriptionRow(ctx, tx, open.ID, open.PlanID, status); err != nil {
			return err
		}
		id = open.ID
		return nil
	})
	return id, err
}

type openSubscription struct {
	ID     uint64
	PlanID uint64
	Status string
}

func requireTenantAndPlan(ctx context.Context, tx *Tx, tenantID, planID uint64) error {
	var tenantExists, planExists bool
	if err := tx.QueryRowContext(ctx, `SELECT EXISTS (SELECT 1 FROM sys_tenant WHERE id = $1 AND deleted_at IS NULL)`, tenantID).Scan(&tenantExists); err != nil {
		return err
	}
	if err := tx.QueryRowContext(ctx, `SELECT EXISTS (SELECT 1 FROM sys_plan WHERE id = $1)`, planID).Scan(&planExists); err != nil {
		return err
	}
	if !tenantExists || !planExists {
		return ErrNotFound
	}
	return nil
}

func loadOpenSubscription(ctx context.Context, tx *Tx, tenantID uint64) (openSubscription, error) {
	var row openSubscription
	err := tx.QueryRowContext(ctx, `
SELECT id, plan_id, status
FROM sys_tenant_subscription
WHERE tenant_id = $1 AND status IN ('trial', 'active', 'overdue')
ORDER BY id DESC
LIMIT 1`, tenantID).Scan(&row.ID, &row.PlanID, &row.Status)
	return row, err
}

// retireOtherActive cancels a different active row before this one becomes
// active, so the partial unique index still holds.
func retireOtherActive(ctx context.Context, tx *Tx, tenantID, keepID uint64, status string) error {
	if status != SubscriptionActive {
		return nil
	}
	_, err := tx.ExecContext(ctx, `
UPDATE sys_tenant_subscription
SET status = 'cancelled', ended_at = COALESCE(ended_at, NOW()), updated_at = NOW()
WHERE tenant_id = $1 AND status = 'active' AND id <> $2`, tenantID, keepID)
	return err
}

// updateSubscriptionRowWithPlan is updateSubscriptionRow for a plan change: it
// also replaces the pinned version and snapshot.
func updateSubscriptionRowWithPlan(ctx context.Context, tx *Tx, id, planID uint64, status string, versionID uint64, snapshot []byte) error {
	if err := updateSubscriptionRow(ctx, tx, id, planID, status); err != nil {
		return err
	}
	_, err := tx.ExecContext(ctx, `
UPDATE sys_tenant_subscription
SET plan_version_id = $2, plan_snapshot = $3::jsonb
WHERE id = $1`, id, versionID, string(snapshot))
	return err
}

// updateSubscriptionRow writes status onto an existing row.
// Becoming overdue starts a 7-day grace when grace_ends_at is still null.
// Returning to active or trial clears ended_at and grace_ends_at so the
// hourly job does not immediately send the row back to overdue or cancelled.
func updateSubscriptionRow(ctx context.Context, tx *Tx, id, planID uint64, status string) error {
	_, err := tx.ExecContext(ctx, `
UPDATE sys_tenant_subscription
SET plan_id = $2,
    status = $3,
    grace_ends_at = CASE
      WHEN $3 = 'overdue' THEN COALESCE(grace_ends_at, NOW() + INTERVAL '7 days')
      WHEN $3 IN ('active', 'trial') THEN NULL
      ELSE grace_ends_at
    END,
    ended_at = CASE
      WHEN $3 = 'cancelled' THEN COALESCE(ended_at, NOW())
      WHEN $3 IN ('active', 'trial') THEN NULL
      ELSE ended_at
    END,
    updated_at = NOW()
WHERE id = $1`, id, planID, status)
	return err
}

// SubscriptionReconcileStats counts the lifecycle moves one reconcile pass made.
type SubscriptionReconcileStats struct {
	Overdue   int `json:"overdue"`
	Cancelled int `json:"cancelled"`
}

// ReconcileSubscriptions applies due lifecycle moves with row-level security
// bypassed.
//
//  1. trial whose trial_ends_at is past → overdue, grace_ends_at = NOW()+7 days if null
//  2. active whose ended_at is past → overdue, same grace
//  3. overdue whose grace_ends_at is past → cancelled, and the tenant is suspended
//
// Each move to overdue or cancelled inserts a draft subscription invoice.
// Rows that just entered overdue are not cancelled in the same pass.
func (r *TenantRepository) ReconcileSubscriptions(ctx context.Context) (SubscriptionReconcileStats, error) {
	var stats SubscriptionReconcileStats
	err := r.platformTx(ctx, func(ctx context.Context, tx *Tx) error {
		overdue, err := reconcileMove(ctx, tx, SubscriptionTrial, SubscriptionOverdue,
			`status = 'trial' AND trial_ends_at IS NOT NULL AND trial_ends_at < NOW()`,
			"trial ended; grace started", false)
		if err != nil {
			return err
		}
		activeOverdue, err := reconcileMove(ctx, tx, SubscriptionActive, SubscriptionOverdue,
			`status = 'active' AND ended_at IS NOT NULL AND ended_at < NOW()`,
			"subscription term ended; grace started", false)
		if err != nil {
			return err
		}
		cancelled, err := reconcileMove(ctx, tx, SubscriptionOverdue, SubscriptionCancelled,
			`status = 'overdue' AND grace_ends_at IS NOT NULL AND grace_ends_at < NOW()`,
			"grace ended; subscription cancelled", true)
		if err != nil {
			return err
		}
		stats.Overdue = overdue + activeOverdue
		stats.Cancelled = cancelled
		return nil
	})
	return stats, err
}

func reconcileMove(ctx context.Context, tx *Tx, from, to, where, memo string, suspend bool) (int, error) {
	if !ValidSubscriptionTransition(from, to) {
		return 0, ErrInvalidSubscriptionTransition
	}
	rows, err := tx.QueryContext(ctx, `
SELECT id, tenant_id, started_at, trial_ends_at, ended_at, grace_ends_at
FROM sys_tenant_subscription
WHERE `+where+`
ORDER BY id
FOR UPDATE`)
	if err != nil {
		return 0, err
	}
	defer rows.Close()
	type dueRow struct {
		id          uint64
		tenantID    uint64
		startedAt   time.Time
		trialEndsAt sql.NullTime
		endedAt     sql.NullTime
		graceEndsAt sql.NullTime
	}
	var due []dueRow
	for rows.Next() {
		var row dueRow
		if err := rows.Scan(&row.id, &row.tenantID, &row.startedAt, &row.trialEndsAt, &row.endedAt, &row.graceEndsAt); err != nil {
			return 0, err
		}
		due = append(due, row)
	}
	if err := rows.Err(); err != nil {
		return 0, err
	}
	if err := rows.Close(); err != nil {
		return 0, err
	}
	moved := 0
	for _, row := range due {
		res, err := tx.ExecContext(ctx, `
UPDATE sys_tenant_subscription
SET status = $2,
    grace_ends_at = CASE
      WHEN $2 = 'overdue' THEN COALESCE(grace_ends_at, NOW() + INTERVAL '7 days')
      ELSE grace_ends_at
    END,
    ended_at = CASE
      WHEN $2 = 'cancelled' THEN COALESCE(ended_at, NOW())
      ELSE ended_at
    END,
    updated_at = NOW()
WHERE id = $1 AND status = $3`, row.id, to, from)
		if err != nil {
			return moved, err
		}
		n, err := res.RowsAffected()
		if err != nil {
			return moved, err
		}
		if n != 1 {
			continue
		}
		if suspend {
			if _, err := tx.ExecContext(ctx, `
UPDATE sys_tenant SET status = 0, updated_at = NOW()
WHERE id = $1 AND deleted_at IS NULL`, row.tenantID); err != nil {
				return moved, err
			}
		}
		periodEnd := row.endedAt
		switch to {
		case SubscriptionOverdue:
			if from == SubscriptionTrial {
				periodEnd = row.trialEndsAt
			}
		case SubscriptionCancelled:
			periodEnd = row.graceEndsAt
		}
		if err := insertInvoiceStub(ctx, tx, row.tenantID, row.id, row.startedAt, periodEnd, memo); err != nil {
			return moved, err
		}
		moved++
	}
	return moved, nil
}

func insertInvoiceStub(ctx context.Context, tx *Tx, tenantID, subscriptionID uint64, periodStart time.Time, periodEnd sql.NullTime, memo string) error {
	_, err := tx.ExecContext(ctx, `
INSERT INTO sys_invoice (
  tenant_id, subscription_id, kind, status, amount_cents, currency,
  period_start, period_end, memo
) VALUES ($1, $2, 'subscription', 'draft', 0, 'CNY', $3, $4, $5)`,
		tenantID, subscriptionID, periodStart, periodEnd, clipRunes(memo, 500))
	return err
}

// PlatformInvoice is one sys_invoice row for the operator console.
type PlatformInvoice struct {
	ID             uint64     `json:"id"`
	TenantID       uint64     `json:"tenantId"`
	SubscriptionID *uint64    `json:"subscriptionId,omitempty"`
	Kind           string     `json:"kind"`
	Status         string     `json:"status"`
	AmountCents    int64      `json:"amountCents"`
	Currency       string     `json:"currency"`
	PeriodStart    *time.Time `json:"periodStart,omitempty"`
	PeriodEnd      *time.Time `json:"periodEnd,omitempty"`
	Memo           string     `json:"memo"`
	CreatedAt      time.Time  `json:"createdAt"`
	UpdatedAt      time.Time  `json:"updatedAt"`
}

// ListTenantInvoices returns the newest invoices for one tenant, up to 200.
// A missing tenant is ErrNotFound.
func (r *TenantRepository) ListTenantInvoices(ctx context.Context, tenantID uint64) ([]PlatformInvoice, error) {
	var items []PlatformInvoice
	err := r.platformTx(ctx, func(ctx context.Context, tx *Tx) error {
		var exists bool
		if err := tx.QueryRowContext(ctx, `SELECT EXISTS (SELECT 1 FROM sys_tenant WHERE id = $1 AND deleted_at IS NULL)`, tenantID).Scan(&exists); err != nil {
			return err
		}
		if !exists {
			return ErrNotFound
		}
		rows, err := tx.QueryContext(ctx, invoiceSelect+`
WHERE tenant_id = $1
ORDER BY id DESC
LIMIT 200`, tenantID)
		if err != nil {
			return err
		}
		defer rows.Close()
		items, err = scanInvoices(rows)
		return err
	})
	return items, err
}

// ListRecentInvoices returns the newest invoices across tenants, up to 100.
func (r *TenantRepository) ListRecentInvoices(ctx context.Context) ([]PlatformInvoice, error) {
	var items []PlatformInvoice
	err := r.platformTx(ctx, func(ctx context.Context, tx *Tx) error {
		rows, err := tx.QueryContext(ctx, invoiceSelect+`
ORDER BY id DESC
LIMIT 100`)
		if err != nil {
			return err
		}
		defer rows.Close()
		items, err = scanInvoices(rows)
		return err
	})
	return items, err
}

const invoiceSelect = `
SELECT id, tenant_id, subscription_id, kind, status, amount_cents, currency,
       period_start, period_end, memo, created_at, updated_at
FROM sys_invoice`

func scanInvoices(rows *sql.Rows) ([]PlatformInvoice, error) {
	items := make([]PlatformInvoice, 0)
	for rows.Next() {
		var (
			item           PlatformInvoice
			subscriptionID sql.NullInt64
			periodStart    sql.NullTime
			periodEnd      sql.NullTime
		)
		if err := rows.Scan(
			&item.ID, &item.TenantID, &subscriptionID, &item.Kind, &item.Status,
			&item.AmountCents, &item.Currency, &periodStart, &periodEnd, &item.Memo,
			&item.CreatedAt, &item.UpdatedAt,
		); err != nil {
			return nil, err
		}
		if subscriptionID.Valid && subscriptionID.Int64 > 0 {
			id := uint64(subscriptionID.Int64)
			item.SubscriptionID = &id
		}
		item.PeriodStart = nullTimePtr(periodStart)
		item.PeriodEnd = nullTimePtr(periodEnd)
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return items, nil
}

// SubscriptionDetail is the tenant's current subscription plus the plan name.
type SubscriptionDetail struct {
	ID          uint64     `json:"id"`
	PlanID      uint64     `json:"planId"`
	PlanCode    string     `json:"planCode"`
	PlanName    string     `json:"planName"`
	Status      string     `json:"status"`
	StartedAt   time.Time  `json:"startedAt"`
	EndedAt     *time.Time `json:"endedAt"`
	TrialEndsAt *time.Time `json:"trialEndsAt"`
	GraceEndsAt *time.Time `json:"graceEndsAt"`
}

// TenantQuota is one sys_tenant_quota row for the billing page.
type TenantQuota struct {
	Key   string `json:"key"`
	Used  int64  `json:"used"`
	Limit int64  `json:"limit"`
}

// BillingSummary is the current tenant's subscription, quotas, and invoices.
type BillingSummary struct {
	TenantID     uint64              `json:"tenantId"`
	Subscription *SubscriptionDetail `json:"subscription"`
	Quotas       []TenantQuota       `json:"quotas"`
	Invoices     []PlatformInvoice   `json:"invoices"`
}

// GetSubscriptionDetail returns the open trial, active, or overdue row when
// one exists, otherwise the newest subscription. No row yields a nil detail.
// The read uses the caller's tenant session, so row-level security applies.
func (r *TenantRepository) GetSubscriptionDetail(ctx context.Context, tenantID uint64) (*SubscriptionDetail, error) {
	var item *SubscriptionDetail
	err := r.sessionTx(ctx, func(ctx context.Context, tx *Tx) error {
		row := tx.QueryRowContext(ctx, `
SELECT s.id, s.plan_id, p.code, p.name, s.status,
       s.started_at, s.ended_at, s.trial_ends_at, s.grace_ends_at
FROM sys_tenant_subscription s
INNER JOIN sys_plan p ON p.id = s.plan_id
WHERE s.tenant_id = $1
ORDER BY CASE WHEN s.status IN ('trial', 'active', 'overdue') THEN 0 ELSE 1 END,
         s.id DESC
LIMIT 1`, tenantID)
		scanned, err := scanSubscriptionDetail(row)
		if errors.Is(err, sql.ErrNoRows) {
			return nil
		}
		if err != nil {
			return err
		}
		item = &scanned
		return nil
	})
	return item, err
}

// ListQuotasForTenant returns quota rows for one tenant under the caller's
// tenant session. Row-level security hides other tenants.
func (r *TenantRepository) ListQuotasForTenant(ctx context.Context, tenantID uint64) ([]TenantQuota, error) {
	items := []TenantQuota{}
	err := r.sessionTx(ctx, func(ctx context.Context, tx *Tx) error {
		rows, err := tx.QueryContext(ctx, `
SELECT quota_key, used_value, limit_value
FROM sys_tenant_quota
WHERE tenant_id = $1
ORDER BY quota_key`, tenantID)
		if err != nil {
			return err
		}
		defer rows.Close()
		items = []TenantQuota{}
		for rows.Next() {
			var item TenantQuota
			if err := rows.Scan(&item.Key, &item.Used, &item.Limit); err != nil {
				return err
			}
			items = append(items, item)
		}
		return rows.Err()
	})
	return items, err
}

// ListCurrentTenantInvoices returns the newest invoices for one tenant under
// the caller's tenant session (no platform bypass). limit is clamped to 1..200.
func (r *TenantRepository) ListCurrentTenantInvoices(ctx context.Context, tenantID uint64, limit int) ([]PlatformInvoice, error) {
	if limit < 1 || limit > 200 {
		limit = 50
	}
	items := []PlatformInvoice{}
	err := r.sessionTx(ctx, func(ctx context.Context, tx *Tx) error {
		rows, err := tx.QueryContext(ctx, invoiceSelect+`
WHERE tenant_id = $1
ORDER BY id DESC
LIMIT $2`, tenantID, limit)
		if err != nil {
			return err
		}
		defer rows.Close()
		items, err = scanInvoices(rows)
		return err
	})
	if items == nil {
		items = []PlatformInvoice{}
	}
	return items, err
}

func scanSubscriptionDetail(scanner interface{ Scan(...any) error }) (SubscriptionDetail, error) {
	var (
		item        SubscriptionDetail
		endedAt     sql.NullTime
		trialEndsAt sql.NullTime
		graceEndsAt sql.NullTime
	)
	err := scanner.Scan(
		&item.ID, &item.PlanID, &item.PlanCode, &item.PlanName, &item.Status,
		&item.StartedAt, &endedAt, &trialEndsAt, &graceEndsAt,
	)
	if err != nil {
		return SubscriptionDetail{}, err
	}
	item.EndedAt = nullTimePtr(endedAt)
	item.TrialEndsAt = nullTimePtr(trialEndsAt)
	item.GraceEndsAt = nullTimePtr(graceEndsAt)
	return item, nil
}
