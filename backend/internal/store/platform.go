package store

import (
	"context"
	"database/sql"
	"errors"
	"strconv"
	"strings"
	"time"

	"dragz-admin/backend/internal/pkg/ctxkey"
	sqlcstore "dragz-admin/backend/internal/store/sqlc"
)

// PlatformTenant is one sys_tenant row as the operator console lists it.
type PlatformTenant struct {
	ID          uint64     `json:"id"`
	Code        string     `json:"code"`
	Name        string     `json:"name"`
	Status      int16      `json:"status"`
	Remark      string     `json:"remark"`
	Slug        string     `json:"slug"`
	CreatedAt   time.Time  `json:"createdAt"`
	PurgeAfter  *time.Time `json:"purgeAfter,omitempty"`
	PurgeStatus string     `json:"purgeStatus"`
}

// PlanEntitlement is one feature line on a plan.
type PlanEntitlement struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

// PlatformPlan is a plan plus the entitlements attached to it.
type PlatformPlan struct {
	ID           uint64            `json:"id"`
	Code         string            `json:"code"`
	Name         string            `json:"name"`
	Remark       string            `json:"remark"`
	Status       int16             `json:"status"`
	Sort         int               `json:"sort"`
	Entitlements []PlanEntitlement `json:"entitlements"`
}

// SetPlatformDB installs the pool used for cross-tenant platform work.
// Production sets it to a role with BYPASSRLS, so platform reads do not depend
// on the application role's app.rls_bypass GUC. A nil value keeps the shared
// application pool for local development and tests.
func (r *TenantRepository) SetPlatformDB(db *DB) {
	if db != nil {
		r.platformDB = db
	}
}

func (r *TenantRepository) platformHandle() *DB {
	if r != nil && r.platformDB != nil {
		return r.platformDB
	}
	if r == nil {
		return nil
	}
	return r.db
}

// platformTx runs fn in a fresh transaction on the platform pool. It does not
// reuse a request transaction: platform queries must cross tenant boundaries
// using the platform database identity, even when the HTTP request itself has
// an application-role transaction open.
func (r *TenantRepository) platformTx(ctx context.Context, fn func(context.Context, *Tx) error) error {
	if ctx == nil {
		ctx = context.Background()
	}
	db := r.platformHandle()
	if db == nil {
		return ErrNotFound
	}
	// Drop the request transaction so BeginTx opens a real transaction on the
	// platform pool instead of a savepoint on the application connection.
	ctx = ctxkey.WithRLSBypass(ctxkey.WithoutTx(ctx), true)
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	if err := fn(ctx, tx); err != nil {
		return err
	}
	return tx.Commit()
}

// readPlatform is the read-only form of platformTx.
func (r *TenantRepository) readPlatform(ctx context.Context, fn func(context.Context, *Tx) error) error {
	return r.platformTx(ctx, fn)
}

// sessionTx runs fn on the caller's tenant session. It does not turn row-level
// security bypass on, so a tenant member only sees rows their policy allows.
// A request transaction is reused through a savepoint.
func (r *TenantRepository) sessionTx(ctx context.Context, fn func(context.Context, *Tx) error) error {
	if ctx == nil {
		ctx = context.Background()
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	if err := fn(ctx, tx); err != nil {
		return err
	}
	return tx.Commit()
}

// Exists reports whether a non-deleted tenant has this id.
func (r *TenantRepository) Exists(ctx context.Context, tenantID uint64) (bool, error) {
	var found bool
	err := r.platformTx(ctx, func(ctx context.Context, tx *Tx) error {
		err := tx.QueryRowContext(ctx, `
SELECT EXISTS (
  SELECT 1 FROM sys_tenant WHERE id = $1 AND deleted_at IS NULL
)`, tenantID).Scan(&found)
		return err
	})
	return found, err
}

// ListPlatformTenants returns every non-deleted tenant, oldest id first.
func (r *TenantRepository) ListPlatformTenants(ctx context.Context) ([]PlatformTenant, error) {
	var items []PlatformTenant
	err := r.platformTx(ctx, func(ctx context.Context, tx *Tx) error {
		rows, err := tx.QueryContext(ctx, `
SELECT id, code, name, status, remark, created_at, purge_after, purge_status, slug
FROM sys_tenant
WHERE deleted_at IS NULL
ORDER BY id`)
		if err != nil {
			return err
		}
		defer rows.Close()
		items = make([]PlatformTenant, 0)
		for rows.Next() {
			item, err := scanPlatformTenant(rows)
			if err != nil {
				return err
			}
			items = append(items, item)
		}
		return rows.Err()
	})
	return items, err
}

// CreatePlatformTenant inserts a tenant. slug may be empty when the code is
// not a usable host label. A live code or slug collision is ErrDuplicateEntry.
func (r *TenantRepository) CreatePlatformTenant(ctx context.Context, code, name, slug string) (PlatformTenant, error) {
	var item PlatformTenant
	err := r.platformTx(ctx, func(ctx context.Context, tx *Tx) error {
		row := tx.QueryRowContext(ctx, `
INSERT INTO sys_tenant (code, name, status, slug, created_by, updated_by)
VALUES ($1, $2, 1, $3, $4, $4)
RETURNING id, code, name, status, remark, created_at, purge_after, purge_status, slug`,
			code, name, slug, actorID(ctx))
		scanned, scanErr := scanPlatformTenant(row)
		if scanErr != nil {
			return wrapDuplicate(scanErr)
		}
		item = scanned
		// A tenant created from the console has to be usable, not just
		// present: it gets a root department, an administrator role with the
		// menu grants, a trial subscription and the plan's quotas. Signup uses
		// the same provisioning, so the two entry points cannot drift.
		_, err := provisionTenant(ctx, tx, item.ID, "")
		return err
	})
	return item, err
}

// UpdatePlatformTenantStatus sets status on a live tenant. A missing row is ErrNotFound.
func (r *TenantRepository) UpdatePlatformTenantStatus(ctx context.Context, tenantID uint64, status int16) error {
	var affected int64
	err := r.platformTx(ctx, func(ctx context.Context, tx *Tx) error {
		result, err := tx.ExecContext(ctx, `
UPDATE sys_tenant SET status = $2, updated_at = NOW(), updated_by = $3
WHERE id = $1 AND deleted_at IS NULL`, tenantID, status, actorID(ctx))
		if err != nil {
			return err
		}
		affected, err = result.RowsAffected()
		return err
	})
	if err != nil {
		return err
	}
	if affected != 1 {
		return ErrNotFound
	}
	return nil
}

// ListPlatformPlans returns every plan with its entitlement rows.
func (r *TenantRepository) ListPlatformPlans(ctx context.Context) ([]PlatformPlan, error) {
	var items []PlatformPlan
	err := r.platformTx(ctx, func(ctx context.Context, tx *Tx) error {
		rows, err := tx.QueryContext(ctx, `
SELECT p.id, p.code, p.name, p.remark, p.status, p.sort,
       COALESCE(e.entitlement_key, ''), COALESCE(e.entitlement_value, '')
FROM sys_plan p
LEFT JOIN sys_plan_entitlement e ON e.plan_id = p.id
ORDER BY p.sort, p.id, e.entitlement_key`)
		if err != nil {
			return err
		}
		defer rows.Close()
		items = []PlatformPlan{}
		index := map[uint64]int{}
		for rows.Next() {
			var (
				id     uint64
				code   string
				name   string
				remark string
				status int16
				sort   int
				key    string
				value  string
			)
			if err := rows.Scan(&id, &code, &name, &remark, &status, &sort, &key, &value); err != nil {
				return err
			}
			at, ok := index[id]
			if !ok {
				items = append(items, PlatformPlan{
					ID: id, Code: code, Name: name, Remark: remark,
					Status: status, Sort: sort, Entitlements: []PlanEntitlement{},
				})
				at = len(items) - 1
				index[id] = at
			}
			if key != "" {
				items[at].Entitlements = append(items[at].Entitlements, PlanEntitlement{Key: key, Value: value})
			}
		}
		return rows.Err()
	})
	return items, err
}

// PlanWrite is a create or update of one sys_plan row.
// Nil Remark, Status, or Sort means "leave the stored value" on update.
// On create the caller should set them. ReplaceEntitlements rewrites the
// entitlement rows; otherwise they stay as they are.
type PlanWrite struct {
	Code                string
	Name                string
	Remark              *string
	Status              *int16
	Sort                *int
	Entitlements        []PlanEntitlement
	ReplaceEntitlements bool
}

// CreatePlatformPlan inserts a plan and its entitlements.
// A live code collision is ErrDuplicateEntry.
func (r *TenantRepository) CreatePlatformPlan(ctx context.Context, in PlanWrite) (PlatformPlan, error) {
	remark := ""
	if in.Remark != nil {
		remark = *in.Remark
	}
	status := int16(1)
	if in.Status != nil {
		status = *in.Status
	}
	sort := 0
	if in.Sort != nil {
		sort = *in.Sort
	}
	var item PlatformPlan
	err := r.platformTx(ctx, func(ctx context.Context, tx *Tx) error {
		var id uint64
		err := tx.QueryRowContext(ctx, `
INSERT INTO sys_plan (code, name, remark, status, sort, created_by, updated_by)
VALUES ($1, $2, $3, $4, $5, $6, $6)
RETURNING id`, in.Code, in.Name, remark, status, sort, actorID(ctx)).Scan(&id)
		if err != nil {
			return wrapDuplicate(err)
		}
		if err := replacePlanEntitlements(ctx, tx, id, in.Entitlements); err != nil {
			return err
		}
		item, err = loadPlatformPlan(ctx, tx, id)
		return err
	})
	return item, err
}

// UpdatePlatformPlan rewrites the plan fields that were provided.
// A missing plan is ErrNotFound. A code collision is ErrDuplicateEntry.
// Entitlements are replaced only when ReplaceEntitlements is set.
func (r *TenantRepository) UpdatePlatformPlan(ctx context.Context, planID uint64, in PlanWrite) (PlatformPlan, error) {
	var remark sql.NullString
	if in.Remark != nil {
		remark = sql.NullString{String: *in.Remark, Valid: true}
	}
	var status sql.NullInt16
	if in.Status != nil {
		status = sql.NullInt16{Int16: *in.Status, Valid: true}
	}
	var sort sql.NullInt32
	if in.Sort != nil {
		sort = sql.NullInt32{Int32: int32(*in.Sort), Valid: true}
	}
	var item PlatformPlan
	err := r.platformTx(ctx, func(ctx context.Context, tx *Tx) error {
		result, err := tx.ExecContext(ctx, `
UPDATE sys_plan
SET code = $2,
    name = $3,
    remark = COALESCE($4::varchar, remark),
    status = COALESCE($5::smallint, status),
    sort = COALESCE($6::int, sort),
    updated_at = NOW(),
    updated_by = $7
WHERE id = $1`, planID, in.Code, in.Name, remark, status, sort, actorID(ctx))
		if err != nil {
			return wrapDuplicate(err)
		}
		affected, err := result.RowsAffected()
		if err != nil {
			return err
		}
		if affected != 1 {
			return ErrNotFound
		}
		if in.ReplaceEntitlements {
			if err := replacePlanEntitlements(ctx, tx, planID, in.Entitlements); err != nil {
				return err
			}
			// A changed catalog is a new version. Existing subscriptions keep
			// the snapshot they were sold at; only new subscriptions see this
			// version. That is what makes a plan edit non-retroactive.
			if err := bumpPlanVersion(ctx, tx, planID); err != nil {
				return err
			}
		}
		item, err = loadPlatformPlan(ctx, tx, planID)
		return err
	})
	return item, err
}

// bumpPlanVersion increments sys_plan.version and records the new catalog in
// sys_plan_version. It runs inside the caller's transaction so a failed plan
// update cannot leave a version behind.
func bumpPlanVersion(ctx context.Context, tx *Tx, planID uint64) error {
	var (
		version int
		code    string
		name    string
	)
	if err := tx.QueryRowContext(ctx, `
UPDATE sys_plan SET version = version + 1, updated_at = NOW(), updated_by = $2
WHERE id = $1
RETURNING version, code, name`, planID, actorID(ctx)).Scan(&version, &code, &name); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ErrNotFound
		}
		return err
	}
	snapshot, err := buildPlanSnapshot(ctx, tx, planID, code, name, version)
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, `
INSERT INTO sys_plan_version (plan_id, version, snapshot, created_by)
VALUES ($1, $2, $3::jsonb, $4)
ON CONFLICT (plan_id, version) DO NOTHING`, planID, version, string(snapshot), actorID(ctx))
	return err
}

// UpdatePlatformPlanStatus sets status on one plan. A missing plan is ErrNotFound.
func (r *TenantRepository) UpdatePlatformPlanStatus(ctx context.Context, planID uint64, status int16) error {
	var affected int64
	err := r.platformTx(ctx, func(ctx context.Context, tx *Tx) error {
		result, err := tx.ExecContext(ctx, `
UPDATE sys_plan SET status = $2, updated_at = NOW(), updated_by = $3
WHERE id = $1`, planID, status, actorID(ctx))
		if err != nil {
			return err
		}
		affected, err = result.RowsAffected()
		return err
	})
	if err != nil {
		return err
	}
	if affected != 1 {
		return ErrNotFound
	}
	return nil
}

// replacePlanEntitlements deletes the plan's entitlement rows and inserts the
// given set. Duplicate keys in items keep the last value. The unique index
// (plan_id, entitlement_key) makes a repeated insert update in place.
func replacePlanEntitlements(ctx context.Context, tx *Tx, planID uint64, items []PlanEntitlement) error {
	if _, err := tx.ExecContext(ctx, `DELETE FROM sys_plan_entitlement WHERE plan_id = $1`, planID); err != nil {
		return err
	}
	for _, item := range items {
		if item.Key == "" {
			continue
		}
		if _, err := tx.ExecContext(ctx, `
INSERT INTO sys_plan_entitlement (plan_id, entitlement_key, entitlement_value, created_by, updated_by)
VALUES ($1, $2, $3, $4, $4)
ON CONFLICT (plan_id, entitlement_key) DO UPDATE
SET entitlement_value = EXCLUDED.entitlement_value, updated_at = NOW(),
  updated_by = EXCLUDED.updated_by`,
			planID, item.Key, item.Value, actorID(ctx)); err != nil {
			return err
		}
	}
	return nil
}

func loadPlatformPlan(ctx context.Context, tx *Tx, planID uint64) (PlatformPlan, error) {
	rows, err := tx.QueryContext(ctx, `
SELECT p.id, p.code, p.name, p.remark, p.status, p.sort,
       COALESCE(e.entitlement_key, ''), COALESCE(e.entitlement_value, '')
FROM sys_plan p
LEFT JOIN sys_plan_entitlement e ON e.plan_id = p.id
WHERE p.id = $1
ORDER BY e.entitlement_key`, planID)
	if err != nil {
		return PlatformPlan{}, err
	}
	defer rows.Close()
	var (
		item  PlatformPlan
		found bool
	)
	for rows.Next() {
		var key, value string
		if err := rows.Scan(&item.ID, &item.Code, &item.Name, &item.Remark, &item.Status, &item.Sort, &key, &value); err != nil {
			return PlatformPlan{}, err
		}
		if !found {
			item.Entitlements = []PlanEntitlement{}
			found = true
		}
		if key != "" {
			item.Entitlements = append(item.Entitlements, PlanEntitlement{Key: key, Value: value})
		}
	}
	if err := rows.Err(); err != nil {
		return PlatformPlan{}, err
	}
	if !found {
		return PlatformPlan{}, ErrNotFound
	}
	return item, nil
}

func scanPlatformTenant(scanner interface{ Scan(...any) error }) (PlatformTenant, error) {
	var (
		item       PlatformTenant
		purgeAfter sql.NullTime
	)
	err := scanner.Scan(
		&item.ID, &item.Code, &item.Name, &item.Status, &item.Remark, &item.CreatedAt,
		&purgeAfter, &item.PurgeStatus, &item.Slug,
	)
	if err != nil {
		return PlatformTenant{}, err
	}
	item.PurgeAfter = nullTimePtr(purgeAfter)
	return item, nil
}

// SyncQuotasFromPlan copies plan entitlements named quota.<key> into
// sys_tenant_quota. The quota key is the part after "quota." (quota.seats
// becomes seats). A value that is not a non-negative integer is skipped.
// An existing quota row is left unchanged.
func (r *TenantRepository) SyncQuotasFromPlan(ctx context.Context, tenantID, planID uint64) error {
	if tenantID == 0 || planID == 0 {
		return nil
	}
	return r.platformTx(ctx, func(ctx context.Context, tx *Tx) error {
		return applyPlanQuotas(ctx, sqlcstore.New(tx), tenantID, planID)
	})
}

func applyPlanQuotas(ctx context.Context, q *sqlcstore.Queries, tenantID, planID uint64) error {
	if tenantID == 0 || planID == 0 || q == nil {
		return nil
	}
	rows, err := q.ListPlanQuotaEntitlements(ctx, planID)
	if err != nil {
		return err
	}
	for _, row := range rows {
		key, ok := strings.CutPrefix(row.EntitlementKey, "quota.")
		if !ok || key == "" {
			continue
		}
		limit, ok := parseQuotaLimit(row.EntitlementValue)
		if !ok {
			continue
		}
		if err := q.EnsureTenantQuota(ctx, sqlcstore.EnsureTenantQuotaParams{
			TenantID: tenantID, QuotaKey: key, LimitValue: limit,
		}); err != nil {
			return err
		}
	}
	return nil
}

func parseQuotaLimit(value string) (uint64, bool) {
	n, err := strconv.ParseInt(strings.TrimSpace(value), 10, 64)
	if err != nil || n < 0 {
		return 0, false
	}
	return uint64(n), true
}

// InsertImpersonation records that actorUserID is acting inside tenantID.
// The target user is the actor: the token is issued for the operator, scoped
// to that tenant. expiresAt is when the grant ends.
func (r *TenantRepository) InsertImpersonation(ctx context.Context, tenantID, actorUserID uint64, reason string, expiresAt time.Time) (uint64, error) {
	var id uint64
	err := r.platformTx(ctx, func(ctx context.Context, tx *Tx) error {
		var exists bool
		if err := tx.QueryRowContext(ctx, `SELECT EXISTS (SELECT 1 FROM sys_tenant WHERE id = $1 AND deleted_at IS NULL)`, tenantID).Scan(&exists); err != nil {
			return err
		}
		if !exists {
			return ErrNotFound
		}
		return tx.QueryRowContext(ctx, `
INSERT INTO sys_tenant_impersonation
  (tenant_id, actor_user_id, target_user_id, reason, expires_at, created_by)
VALUES ($1, $2, $2, $3, $4, $2)
RETURNING id`, tenantID, actorUserID, reason, expiresAt).Scan(&id)
	})
	return id, err
}
