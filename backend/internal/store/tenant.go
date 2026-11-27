package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	sqlcstore "dragz-admin/backend/internal/store/sqlc"
)

// TenantMembership is one tenant a user may act inside.
type TenantMembership struct {
	ID               uint64
	Code             string
	Name             string
	PlatformOperator bool
}

// OpenTenant is the current-tenant view served by GET /api/open/v1/tenant.
type OpenTenant struct {
	ID     uint64
	Code   string
	Name   string
	Status int16
}

// OutboxEvent is one unpublished row from sys_outbox.
type OutboxEvent struct {
	ID        uint64
	TenantID  uint64
	EventType string
	Payload   []byte
	CreatedAt time.Time
}

// TenantRepository reads membership, entitlements, quotas and the outbox.
// Queries run on the transaction stored in ctx when one is present.
type TenantRepository struct {
	db *DB
	// platformDB is the BYPASSRLS identity used for membership resolution,
	// outbox relaying and platform-console work. Nil falls back to db for local
	// development and tests.
	platformDB *DB
	// baseDomain is the parent of {slug}.{base} hosts. Empty disables slug
	// subdomains. Set once at startup via SetBaseDomain.
	baseDomain string

	colMu      sync.Mutex
	colChecked bool
	colExists  bool
}

// NewTenantRepository binds the tenant queries to a database handle.
func NewTenantRepository(db *DB) *TenantRepository {
	return &TenantRepository{db: db}
}

func (r *TenantRepository) q(ctx context.Context) *sqlcstore.Queries {
	return Queries(ctx, r.db)
}

// Membership returns the caller's active membership in one tenant.
// sql.ErrNoRows means the user is not a member.
// Membership is resolved before the request tenant is known, and a switch
// must see a tenant other than the one on the request transaction, so the
// read bypasses row-level security. The SQL still filters by user and tenant.
func (r *TenantRepository) Membership(ctx context.Context, userID, tenantID uint64) (TenantMembership, error) {
	var out TenantMembership
	err := r.platformTx(ctx, func(ctx context.Context, tx *Tx) error {
		row, err := sqlcstore.New(tx).GetTenantMembership(ctx, sqlcstore.GetTenantMembershipParams{
			UserID: userID, TenantID: tenantID,
		})
		if err != nil {
			return err
		}
		out = membershipFrom(row.ID, row.Code, row.Name, row.IsPlatformOperator)
		return nil
	})
	return out, err
}

// ListForUser returns every active tenant the user belongs to, oldest id first.
// The list spans tenants, so it bypasses the request tenant filter.
func (r *TenantRepository) ListForUser(ctx context.Context, userID uint64) ([]TenantMembership, error) {
	var items []TenantMembership
	err := r.platformTx(ctx, func(ctx context.Context, tx *Tx) error {
		rows, err := sqlcstore.New(tx).ListTenantsForUser(ctx, userID)
		if err != nil {
			return err
		}
		items = make([]TenantMembership, 0, len(rows))
		for _, row := range rows {
			items = append(items, membershipFrom(row.ID, row.Code, row.Name, row.IsPlatformOperator))
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	if items == nil {
		items = []TenantMembership{}
	}
	return items, nil
}

// FirstForUser returns the user's oldest active membership.
// sql.ErrNoRows means the user belongs to no tenant.
// The call runs before a tenant is selected, so it cannot use that tenant's filter.
func (r *TenantRepository) FirstForUser(ctx context.Context, userID uint64) (TenantMembership, error) {
	var out TenantMembership
	err := r.platformTx(ctx, func(ctx context.Context, tx *Tx) error {
		row, err := sqlcstore.New(tx).FirstTenantForUser(ctx, userID)
		if err != nil {
			return err
		}
		out = membershipFrom(row.ID, row.Code, row.Name, row.IsPlatformOperator)
		return nil
	})
	return out, err
}

// GetOpenTenant reads one non-deleted tenant by id. The request transaction
// is already limited by row-level security, so the caller only sees their tenant.
func (r *TenantRepository) GetOpenTenant(ctx context.Context, tenantID uint64) (OpenTenant, error) {
	row, err := r.q(ctx).GetOpenTenant(ctx, int64(tenantID))
	if err != nil {
		return OpenTenant{}, err
	}
	return OpenTenant{ID: uint64(row.ID), Code: row.Code, Name: row.Name, Status: row.Status}, nil
}

// EntitlementEnabled reports whether the tenant may use featureKey.
// Order: subscription status → plan snapshot/catalog → deny.
//
// A cancelled subscription denies every feature. Overdue is a grace period:
// the plan catalog still applies, the same as trial and active. The global
// feature switch is checked before this method runs.
//
// A feature the plan does not list is denied. See the no-row branch below for
// why the default is deny rather than allow.
func (r *TenantRepository) EntitlementEnabled(ctx context.Context, tenantID uint64, featureKey string) (bool, error) {
	status, err := r.SubscriptionStatus(ctx, tenantID)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return false, err
	}
	if status == SubscriptionCancelled {
		return false, nil
	}
	value, found, err := r.planEntitlementValue(ctx, tenantID, featureKey)
	if err != nil {
		return false, err
	}
	if !found || value == "" {
		// A plan that does not mention a feature does not grant it.
		//
		// "No row means allowed" made every unlisted feature free: adding a new
		// paid capability to the code silently handed it to every tenant, and a
		// plan could never express "this tier does not include that". Denying is
		// the only default that stays true when the feature list grows, and a
		// plan that should include the feature is a one-row seed to change.
		return false, nil
	}
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "0", "false", "no", "off":
		return false, nil
	default:
		return true, nil
	}
}

// planEntitlementValue reads the effective entitlement for a feature.
//
// The subscription snapshot wins over the live plan catalog. A tenant that
// subscribed at plan version 3 keeps version 3 when the platform edits the
// plan: the entitlement is part of what they signed up for, and a catalogue
// edit must not silently downgrade an existing customer. Rows written before
// snapshots existed have an empty snapshot, so the live catalog is the
// fallback and their behaviour is unchanged.
func (r *TenantRepository) planEntitlementValue(ctx context.Context, tenantID uint64, featureKey string) (string, bool, error) {
	var value string
	err := DBTX(ctx, r.db).QueryRowContext(ctx, `
SELECT COALESCE(
  NULLIF(s.plan_snapshot->'entitlements'->>$2, ''),
  pe.entitlement_value,
  ''
)
FROM sys_tenant_subscription s
LEFT JOIN sys_plan_entitlement pe
  ON pe.plan_id = s.plan_id AND pe.entitlement_key = $2
WHERE s.tenant_id = $1
  AND s.status IN ('active', 'trial', 'overdue')
ORDER BY s.id DESC
LIMIT 1`, tenantID, featureKey).Scan(&value)
	if errors.Is(err, sql.ErrNoRows) {
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}
	return value, true, nil
}

// SubscriptionStatus returns the latest subscription status for the tenant.
// sql.ErrNoRows means the tenant has no subscription row.
func (r *TenantRepository) SubscriptionStatus(ctx context.Context, tenantID uint64) (string, error) {
	return r.q(ctx).GetTenantSubscriptionStatus(ctx, tenantID)
}

// HasActiveImpersonation reports an open, unexpired impersonation grant.
func (r *TenantRepository) HasActiveImpersonation(ctx context.Context, tenantID, actorUserID uint64) (bool, error) {
	var ok bool
	err := r.platformTx(ctx, func(ctx context.Context, tx *Tx) error {
		var err error
		ok, err = sqlcstore.New(tx).HasActiveImpersonation(ctx, sqlcstore.HasActiveImpersonationParams{
			TenantID: tenantID, ActorUserID: actorUserID,
		})
		return err
	})
	return ok, err
}

// TryConsumeQuota adds amount to the used counter when the tenant still has
// room. ErrQuotaExceeded means the quota row is missing or the increment
// would pass the limit.
func (r *TenantRepository) TryConsumeQuota(ctx context.Context, tenantID uint64, quotaKey string, amount uint64) error {
	if amount == 0 {
		return fmt.Errorf("quota amount must be positive")
	}
	_, err := r.q(ctx).TryConsumeQuota(ctx, sqlcstore.TryConsumeQuotaParams{
		Amount: amount, TenantID: tenantID, QuotaKey: quotaKey,
	})
	if errors.Is(err, sql.ErrNoRows) {
		return ErrQuotaExceeded
	}
	return err
}

// ConsumeQuotaIfConfigured is TryConsumeQuota when a quota row exists.
// A missing row means the plan does not meter that key, so the call succeeds.
func (r *TenantRepository) ConsumeQuotaIfConfigured(ctx context.Context, tenantID uint64, quotaKey string, amount uint64) error {
	err := r.TryConsumeQuota(ctx, tenantID, quotaKey, amount)
	if errors.Is(err, ErrQuotaExceeded) {
		// Distinguish "no row" from "full": retry a zero-amount probe is not
		// available, so read the row directly.
		_, getErr := r.q(ctx).GetTenantQuota(ctx, sqlcstore.GetTenantQuotaParams{
			TenantID: tenantID, QuotaKey: quotaKey,
		})
		if errors.Is(getErr, sql.ErrNoRows) {
			return nil
		}
		if getErr != nil {
			return getErr
		}
		return ErrQuotaExceeded
	}
	return err
}

// releaseQuota gives quota back after a seat or a resource is removed.
// It runs on the caller's queries so it can join an existing transaction;
// a tenant without a row for that key is a no-op.
func releaseQuota(ctx context.Context, q *sqlcstore.Queries, tenantID uint64, quotaKey string, amount uint64) error {
	if tenantID == 0 || quotaKey == "" || amount == 0 {
		return nil
	}
	return q.ReleaseQuota(ctx, sqlcstore.ReleaseQuotaParams{
		Amount: amount, TenantID: tenantID, QuotaKey: quotaKey,
	})
}

// ReleaseQuota gives quota back after a reservation is abandoned (a failed
// upload, a rolled-back member creation). It never goes below zero.
func (r *TenantRepository) ReleaseQuota(ctx context.Context, tenantID uint64, quotaKey string, amount uint64) error {
	if amount == 0 {
		return nil
	}
	return releaseQuota(ctx, r.q(ctx), tenantID, quotaKey, amount)
}

// InsertOutbox appends one event. An empty payload is stored as {}.
func (r *TenantRepository) InsertOutbox(ctx context.Context, tenantID uint64, eventType string, payload []byte) (uint64, error) {
	if len(payload) == 0 {
		payload = []byte(`{}`)
	}
	id, err := r.q(ctx).InsertOutboxEvent(ctx, sqlcstore.InsertOutboxEventParams{
		TenantID: tenantID, EventType: eventType, Payload: json.RawMessage(payload),
	})
	if err != nil {
		return 0, err
	}
	return uint64(id), nil
}

// ClaimPendingOutbox claims up to limit pending events for one worker and
// returns them. limit <= 0 means 100.
//
// The claim is the concurrency control. Two relays running at once must not
// deliver the same event twice, so this is an UPDATE ... FOR UPDATE SKIP LOCKED
// rather than a SELECT: the row moves to "processing" inside the same
// statement, and a competing worker skips it. The worker publishes every
// tenant's events, so the claim runs on the platform identity.
func (r *TenantRepository) ClaimPendingOutbox(ctx context.Context, workerID string, limit int32) ([]OutboxEvent, error) {
	if limit <= 0 {
		limit = 100
	}
	if workerID == "" {
		return nil, fmt.Errorf("outbox claim requires a worker id")
	}
	var items []OutboxEvent
	err := r.platformTx(ctx, func(ctx context.Context, tx *Tx) error {
		rows, err := sqlcstore.New(tx).ClaimPendingOutbox(ctx, sqlcstore.ClaimPendingOutboxParams{
			WorkerID: workerID, BatchSize: limit,
		})
		if err != nil {
			return err
		}
		items = make([]OutboxEvent, 0, len(rows))
		for _, row := range rows {
			items = append(items, OutboxEvent{
				ID: uint64(row.ID), TenantID: row.TenantID, EventType: row.EventType,
				Payload: append([]byte(nil), row.Payload...), CreatedAt: row.CreatedAt,
			})
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	if items == nil {
		items = []OutboxEvent{}
	}
	return items, nil
}

// ReclaimStaleOutbox returns events whose worker stopped reporting back, so a
// crashed process cannot strand a row in "processing" forever.
func (r *TenantRepository) ReclaimStaleOutbox(ctx context.Context, leaseSeconds int32) (int64, error) {
	if leaseSeconds <= 0 {
		leaseSeconds = 600
	}
	var affected int64
	err := r.platformTx(ctx, func(ctx context.Context, tx *Tx) error {
		var err error
		affected, err = sqlcstore.New(tx).ReclaimStaleOutbox(ctx, leaseSeconds)
		return err
	})
	return affected, err
}

// MarkOutboxSent marks one pending event as sent. A missing or already-sent
// row is ErrNotFound. The caller supplies the transaction for the event's tenant.
func (r *TenantRepository) MarkOutboxSent(ctx context.Context, id uint64) error {
	affected, err := r.q(ctx).MarkOutboxSent(ctx, int64(id))
	if err != nil {
		return err
	}
	if affected != 1 {
		return ErrNotFound
	}
	return nil
}

// MarkOutboxFailed records one failed delivery. The row stays pending with a
// backoff until the eighth failure, which dead-letters it. last_error is
// clipped to the column width. A missing or already-finished row is a no-op.
func (r *TenantRepository) MarkOutboxFailed(ctx context.Context, id uint64, errMsg string) error {
	return r.q(ctx).MarkOutboxFailed(ctx, sqlcstore.MarkOutboxFailedParams{
		ID: int64(id), LastError: clipRunes(errMsg, 1000),
	})
}

// clipRunes keeps at most max characters so a long error still fits a varchar column.
func clipRunes(s string, max int) string {
	if max <= 0 {
		return ""
	}
	n := 0
	for i := range s {
		if n == max {
			return s[:i]
		}
		n++
	}
	return s
}

// APITokenTenantID reads sys_api_token.tenant_id when that column exists.
// The bool is false when the column is absent; the caller should then use
// the user's first membership. A present column whose value is 0 is returned
// as 0 with the bool true, which has the same fallback.
func (r *TenantRepository) APITokenTenantID(ctx context.Context, tokenID uint64) (uint64, bool, error) {
	exists, err := r.apiTokenHasTenantID(ctx)
	if err != nil || !exists {
		return 0, false, err
	}
	var id uint64
	err = r.platformTx(ctx, func(ctx context.Context, tx *Tx) error {
		return tx.QueryRowContext(ctx, `SELECT tenant_id FROM sys_api_token WHERE id = $1`, tokenID).Scan(&id)
	})
	if err != nil {
		return 0, true, err
	}
	return id, true, nil
}

func (r *TenantRepository) apiTokenHasTenantID(ctx context.Context) (bool, error) {
	r.colMu.Lock()
	defer r.colMu.Unlock()
	if r.colChecked {
		return r.colExists, nil
	}
	var exists bool
	err := r.platformTx(ctx, func(ctx context.Context, tx *Tx) error {
		return tx.QueryRowContext(ctx, `
SELECT EXISTS (
  SELECT 1 FROM information_schema.columns
  WHERE table_name = 'sys_api_token' AND column_name = 'tenant_id'
)`).Scan(&exists)
	})
	if err != nil {
		return false, err
	}
	r.colChecked = true
	r.colExists = exists
	return exists, nil
}

func membershipFrom(id int64, code, name string, operator bool) TenantMembership {
	return TenantMembership{ID: uint64(id), Code: code, Name: name, PlatformOperator: operator}
}
