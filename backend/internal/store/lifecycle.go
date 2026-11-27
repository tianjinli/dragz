package store

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
)

// This file is the tenant lifecycle: self-service signup, invitations,
// membership management and the plan snapshot that a subscription is sold at.
//
// Everything here that crosses a tenant boundary (signup has no tenant yet, an
// invitation is looked up by token before the caller is a member) runs on the
// platform identity through platformTx. Everything that operates inside one
// tenant runs through sessionTx so row-level security still applies.

// SignupInput is one self-service registration. PasswordHash is already
// bcrypt-hashed by the caller: this layer never sees a plaintext password.
type SignupInput struct {
	TenantCode   string
	TenantName   string
	Username     string
	PasswordHash string
	Email        string
	Mobile       string
	DisplayName  string
	// PlanCode is the plan the tenant starts on. Empty means the lowest-sorted
	// active plan.
	PlanCode string
}

// SignupResult identifies what signup created.
type SignupResult struct {
	TenantID   uint64 `json:"tenantId"`
	TenantCode string `json:"tenantCode"`
	TenantName string `json:"tenantName"`
	UserID     uint64 `json:"userId"`
}

// InviteStatus values. pending is the only live state.
const (
	InvitePending  = "pending"
	InviteAccepted = "accepted"
	InviteRevoked  = "revoked"
	InviteExpired  = "expired"
)

// Invite is one invitation row plus the tenant name the invite link shows.
type Invite struct {
	ID            uint64     `json:"id"`
	TenantID      uint64     `json:"tenantId"`
	TenantName    string     `json:"tenantName,omitempty"`
	Email         string     `json:"email"`
	RoleIDs       []int64    `json:"roleIds"`
	DepartmentID  *uint64    `json:"departmentId,omitempty"`
	IsTenantAdmin bool       `json:"isTenantAdmin"`
	Status        string     `json:"status"`
	InvitedBy     uint64     `json:"invitedBy"`
	ExpiresAt     time.Time  `json:"expiresAt"`
	AcceptedBy    *uint64    `json:"acceptedBy,omitempty"`
	AcceptedAt    *time.Time `json:"acceptedAt,omitempty"`
	CreatedAt     time.Time  `json:"createdAt"`
	// Server-side email delivery of the invite link. Empty means the invite
	// predates delivery tracking or mail was disabled when it was created.
	DeliveryStatus string     `json:"deliveryStatus,omitempty"`
	DeliveredAt    *time.Time `json:"deliveredAt,omitempty"`
	DeliveryError  string     `json:"deliveryError,omitempty"`
}

// Invite delivery statuses, written by the async invite mail sender.
const (
	// InviteMailPending means the send is in flight (or the process died
	// mid-send; the invite UI shows it as "sending").
	InviteMailPending = "pending"
	// InviteMailSent means the SMTP engine accepted the message.
	InviteMailSent = "sent"
	// InviteMailFailed means the engine returned an error; DeliveryError has it.
	InviteMailFailed = "failed"
	// InviteMailSkipped means mail was disabled or no public base URL is
	// configured when the invite was created — not a failure, the admin is
	// expected to copy the link manually.
	InviteMailSkipped = "skipped"
)

// InviteInput creates one invitation.
type InviteInput struct {
	TenantID     uint64
	Email        string
	RoleIDs      []int64
	DepartmentID *uint64
	TenantAdmin  bool
	InvitedBy    uint64
	TTL          time.Duration
}

// Member is one membership row joined with the global account.
type Member struct {
	UserID        uint64    `json:"userId"`
	Username      string    `json:"username"`
	DisplayName   string    `json:"displayName"`
	Email         string    `json:"email"`
	Mobile        string    `json:"mobile"`
	DepartmentID  *uint64   `json:"departmentId,omitempty"`
	IsTenantAdmin bool      `json:"isTenantAdmin"`
	IsOperator    bool      `json:"isPlatformOperator"`
	Status        int16     `json:"status"`
	RoleCodes     []string  `json:"roleCodes"`
	CreatedAt     time.Time `json:"createdAt"`
}

// MemberUpdate carries the fields a caller may change. nil means "leave it".
type MemberUpdate struct {
	Status        *int16
	IsTenantAdmin *bool
	DepartmentID  *uint64
	// RoleIDs replaces the member's role set when non-nil.
	RoleIDs []int64
}

// usagePeriodTotal is the period key for gauges (storage, seats). Counters
// that reset (API calls) use a date.
const usagePeriodTotal = "total"

// SignupTenant creates a tenant, its first administrator and everything that
// tenant needs to be usable: a root department, an administrator role with
// every menu grant, a trial subscription pinned to a plan snapshot, and the
// plan's quotas.
//
// It is a platform operation: no tenant session exists yet, so all writes
// carry an explicit tenant_id and run with the RLS bypass identity.
func (r *TenantRepository) SignupTenant(ctx context.Context, in SignupInput) (SignupResult, error) {
	code := strings.TrimSpace(in.TenantCode)
	name := strings.TrimSpace(in.TenantName)
	username := strings.TrimSpace(in.Username)
	if code == "" || name == "" || username == "" || in.PasswordHash == "" {
		return SignupResult{}, ErrNotFound
	}
	displayName := strings.TrimSpace(in.DisplayName)
	if displayName == "" {
		displayName = username
	}
	out := SignupResult{TenantCode: code, TenantName: name}
	err := r.platformTx(ctx, func(ctx context.Context, tx *Tx) error {
		if err := tx.QueryRowContext(ctx, `
INSERT INTO sys_tenant (code, name, status, slug)
VALUES ($1, $2, 1, $3)
RETURNING id`, code, name, SlugFromCode(code)).Scan(&out.TenantID); err != nil {
			return wrapDuplicate(err)
		}
		provision, err := provisionTenant(ctx, tx, out.TenantID, in.PlanCode)
		if err != nil {
			return err
		}
		if err := tx.QueryRowContext(ctx, `
INSERT INTO sys_user (username, password_hash, display_name, email, mobile, status, must_change_password)
VALUES ($1, $2, $3, $4, $5, 1, 0)
RETURNING id`, username, in.PasswordHash, displayName, in.Email, in.Mobile).Scan(&out.UserID); err != nil {
			return wrapDuplicate(err)
		}
		if _, err := tx.ExecContext(ctx, `
UPDATE sys_user SET department_id = $2 WHERE id = $1`,
			out.UserID, provision.departmentID); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `
INSERT INTO sys_tenant_member (tenant_id, user_id, department_id, is_tenant_admin, status)
VALUES ($1, $2, $3, TRUE, 1)`, out.TenantID, out.UserID, provision.departmentID); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `
INSERT INTO sys_user_role (tenant_id, user_id, role_id, created_by)
VALUES ($1, $2, $3, $4) ON CONFLICT (user_id, role_id) DO NOTHING`,
			out.TenantID, out.UserID, provision.adminRoleID, actorID(ctx)); err != nil {
			return err
		}
		// The owner occupies one seat from the moment the tenant exists.
		_, err = tx.ExecContext(ctx, `
UPDATE sys_tenant_quota SET used_value = 1, updated_at = NOW()
WHERE tenant_id = $1 AND quota_key = 'seats'`, out.TenantID)
		return err
	})
	if err != nil {
		return SignupResult{}, err
	}
	return out, nil
}

// provisionResult is what a fresh tenant needs from provisioning.
type provisionResult struct {
	departmentID uint64
	adminRoleID  uint64
}

// provisionTenant seeds a new tenant. It is used by both self-service signup
// and the platform console: a tenant created by an operator has to be as
// usable as one created by signup, otherwise the console creates dead tenants.
func provisionTenant(ctx context.Context, tx *Tx, tenantID uint64, planCode string) (provisionResult, error) {
	var out provisionResult
	// Root department: the tree every other department hangs under, and the
	// owner's initial department.
	err := tx.QueryRowContext(ctx, `
INSERT INTO sys_department (tenant_id, parent_id, ancestors, code, name, sort, status)
VALUES ($1, 0, '0', 'root', '总部', 0, 1)
RETURNING id`, tenantID).Scan(&out.departmentID)
	if err != nil {
		return out, err
	}
	// Administrator role. Allowlisted menu grants are inserted here rather
	// than derived at request time: permissions are role_menu button rows, so
	// an empty grant set would leave the tenant owner unable to do anything.
	err = tx.QueryRowContext(ctx, `
INSERT INTO sys_role (tenant_id, code, name, data_scope, sort, status, is_system, remark)
VALUES ($1, 'tenant_admin', '租户管理员', 0, 0, 1, TRUE, '租户内置管理员角色')
RETURNING id`, tenantID).Scan(&out.adminRoleID)
	if err != nil {
		return out, err
	}
	// Allowlist only: tenant_admin must not inherit platform, menu CRUD, or
	// user-management pages. Buttons under allowed pages are included via the
	// recursive grant helper.
	if err := grantTenantAdminRoleMenus(ctx, tx, tenantID, out.adminRoleID); err != nil {
		return out, err
	}
	// System dictionaries are read-only templates in tenant 1; every tenant
	// gets its own copy so it can relabel or extend them without touching the
	// shared template. Items follow their type through the code join, and the
	// per-tenant unique keys keep a re-provision idempotent.
	if err := seedTenantDictionaries(ctx, tx, tenantID); err != nil {
		return out, err
	}
	if err := provisionSubscription(ctx, tx, tenantID, planCode); err != nil {
		return out, err
	}
	return out, nil
}

// seedTenantDictionaries copies tenant 1's is_system dictionary types and
// their items into a freshly provisioned tenant. ON CONFLICT DO NOTHING makes
// a re-run a no-op instead of a duplicate-key failure.
func seedTenantDictionaries(ctx context.Context, tx *Tx, tenantID uint64) error {
	if _, err := tx.ExecContext(ctx, `
INSERT INTO sys_dict_type (tenant_id, code, name, display_type, status, is_system, remark)
SELECT $1, t.code, t.name, t.display_type, t.status, 1, t.remark
FROM sys_dict_type t
WHERE t.tenant_id = 1 AND t.is_system = TRUE AND t.deleted_at IS NULL
ON CONFLICT DO NOTHING`, tenantID); err != nil {
		return fmt.Errorf("copy system dictionary types: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `
INSERT INTO sys_dict_item (tenant_id, dict_type_id, label, value, color, css_class, sort, status, is_default, remark)
SELECT $1, target.id, s.label, s.value, s.color, s.css_class, s.sort, s.status, s.is_default, s.remark
FROM sys_dict_item s
JOIN sys_dict_type src ON src.id = s.dict_type_id AND src.tenant_id = 1 AND src.is_system = TRUE AND src.deleted_at IS NULL
JOIN sys_dict_type target ON target.tenant_id = $1 AND target.code = src.code AND target.deleted_at IS NULL
WHERE s.deleted_at IS NULL
ON CONFLICT DO NOTHING`, tenantID); err != nil {
		return fmt.Errorf("copy system dictionary items: %w", err)
	}
	return nil
}

// provisionSubscription pins a trial subscription to a plan snapshot and
// materialises the plan's quota rows.
func provisionSubscription(ctx context.Context, tx *Tx, tenantID uint64, planCode string) error {
	planID, versionID, snapshot, err := resolvePlanSnapshot(ctx, tx, planCode)
	if err != nil {
		return err
	}
	if planID == 0 {
		// The platform has no active plan yet (a fresh database before the
		// seed runs). The tenant exists; billing starts when a plan is applied.
		return nil
	}
	trialEnds := time.Now().Add(14 * 24 * time.Hour)
	if _, err := tx.ExecContext(ctx, `
INSERT INTO sys_tenant_subscription
  (tenant_id, plan_id, status, trial_ends_at, plan_version_id, plan_snapshot)
VALUES ($1, $2, 'trial', $3, $4, $5::jsonb)`,
		tenantID, planID, trialEnds, nullUint64(versionID), string(snapshot)); err != nil {
		return err
	}
	return applySnapshotQuotas(ctx, tx, tenantID, snapshot)
}

// resolvePlanSnapshot loads (or creates) the version row for a plan and
// returns its snapshot. planID 0 with a nil error means "no active plan".
func resolvePlanSnapshot(ctx context.Context, tx *Tx, planCode string) (uint64, uint64, []byte, error) {
	var (
		id      uint64
		code    string
		name    string
		version int
	)
	query := `SELECT id, code, name, version FROM sys_plan WHERE status = 1`
	args := []any{}
	if planCode != "" {
		query += ` AND code = $1`
		args = append(args, planCode)
	}
	query += ` ORDER BY sort, id LIMIT 1`
	err := tx.QueryRowContext(ctx, query, args...).Scan(&id, &code, &name, &version)
	if errors.Is(err, sql.ErrNoRows) && planCode != "" {
		// The requested plan is gone or disabled: fall back to the default
		// plan so a signup is not lost to a stale plan code.
		err = tx.QueryRowContext(ctx, `
SELECT id, code, name, version FROM sys_plan WHERE status = 1 ORDER BY sort, id LIMIT 1`).
			Scan(&id, &code, &name, &version)
	}
	if errors.Is(err, sql.ErrNoRows) {
		return 0, 0, nil, nil
	}
	if err != nil {
		return 0, 0, nil, err
	}
	snapshot, err := buildPlanSnapshot(ctx, tx, id, code, name, version)
	if err != nil {
		return 0, 0, nil, err
	}
	var versionID uint64
	err = tx.QueryRowContext(ctx, `
INSERT INTO sys_plan_version (plan_id, version, snapshot)
VALUES ($1, $2, $3::jsonb)
ON CONFLICT (plan_id, version) DO NOTHING
RETURNING id`, id, version, string(snapshot)).Scan(&versionID)
	if errors.Is(err, sql.ErrNoRows) {
		// Another transaction won the race: use its row.
		err = tx.QueryRowContext(ctx, `
SELECT id FROM sys_plan_version WHERE plan_id = $1 AND version = $2`, id, version).Scan(&versionID)
	}
	if err != nil {
		return 0, 0, nil, err
	}
	return id, versionID, snapshot, nil
}

// planSnapshotFor loads one plan by id and returns its version row plus the
// snapshot JSON, creating the version row when it is missing.
func planSnapshotFor(ctx context.Context, tx *Tx, planID uint64) (uint64, []byte, error) {
	var (
		code    string
		name    string
		version int
	)
	if err := tx.QueryRowContext(ctx, `
SELECT code, name, version FROM sys_plan WHERE id = $1`, planID).Scan(&code, &name, &version); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return 0, nil, ErrNotFound
		}
		return 0, nil, err
	}
	snapshot, err := buildPlanSnapshot(ctx, tx, planID, code, name, version)
	if err != nil {
		return 0, nil, err
	}
	var versionID uint64
	err = tx.QueryRowContext(ctx, `
INSERT INTO sys_plan_version (plan_id, version, snapshot)
VALUES ($1, $2, $3::jsonb)
ON CONFLICT (plan_id, version) DO NOTHING
RETURNING id`, planID, version, string(snapshot)).Scan(&versionID)
	if errors.Is(err, sql.ErrNoRows) {
		err = tx.QueryRowContext(ctx, `
SELECT id FROM sys_plan_version WHERE plan_id = $1 AND version = $2`, planID, version).Scan(&versionID)
	}
	if err != nil {
		return 0, nil, err
	}
	return versionID, snapshot, nil
}

// buildPlanSnapshot freezes the plan and its entitlements into JSON.
func buildPlanSnapshot(ctx context.Context, tx *Tx, planID uint64, code, name string, version int) ([]byte, error) {
	rows, err := tx.QueryContext(ctx, `
SELECT entitlement_key, entitlement_value FROM sys_plan_entitlement
WHERE plan_id = $1 ORDER BY entitlement_key`, planID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	entitlements := map[string]string{}
	for rows.Next() {
		var key, value string
		if err := rows.Scan(&key, &value); err != nil {
			return nil, err
		}
		entitlements[key] = value
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return json.Marshal(planSnapshot{
		PlanID: planID, Code: code, Name: name, Version: version, Entitlements: entitlements,
	})
}

// planSnapshot is the frozen view of a plan that a subscription was sold at.
// Changing the plan later must not change what an existing subscriber is
// entitled to, which is why the snapshot is copied onto the subscription.
type planSnapshot struct {
	PlanID       uint64            `json:"planId"`
	Code         string            `json:"code"`
	Name         string            `json:"name"`
	Version      int               `json:"version"`
	Entitlements map[string]string `json:"entitlements"`
}

// applySnapshotQuotas copies quota.* entitlements from the snapshot into
// sys_tenant_quota. A value that is not a non-negative integer is skipped.
func applySnapshotQuotas(ctx context.Context, tx *Tx, tenantID uint64, snapshot []byte) error {
	var parsed planSnapshot
	if err := json.Unmarshal(snapshot, &parsed); err != nil {
		return nil
	}
	for key, value := range parsed.Entitlements {
		quotaKey, ok := strings.CutPrefix(key, "quota.")
		if !ok || quotaKey == "" {
			continue
		}
		limit, ok := parseQuotaLimit(value)
		if !ok {
			continue
		}
		if _, err := tx.ExecContext(ctx, `
INSERT INTO sys_tenant_quota (tenant_id, quota_key, limit_value, used_value)
VALUES ($1, $2, $3, 0)
ON CONFLICT (tenant_id, quota_key) DO UPDATE
SET limit_value = EXCLUDED.limit_value, updated_at = NOW()`,
			tenantID, quotaKey, limit); err != nil {
			return err
		}
	}
	return nil
}

// CreateInvite issues one invitation and returns the plaintext token. Only the
// hash is stored; the caller must show the token once.
func (r *TenantRepository) CreateInvite(ctx context.Context, in InviteInput) (Invite, string, error) {
	token, err := newInviteToken()
	if err != nil {
		return Invite{}, "", err
	}
	email := strings.TrimSpace(in.Email)
	ttl := in.TTL
	if ttl <= 0 {
		ttl = 7 * 24 * time.Hour
	}
	var invite Invite
	err = r.platformTx(ctx, func(ctx context.Context, tx *Tx) error {
		// Re-inviting the same address replaces the previous link instead of
		// leaving two valid tokens for one seat.
		if email != "" {
			if _, err := tx.ExecContext(ctx, `
UPDATE sys_tenant_invite SET status = 'revoked', updated_by = $3
WHERE tenant_id = $1 AND lower(email) = lower($2) AND status = 'pending'`,
				in.TenantID, email, in.InvitedBy); err != nil {
				return err
			}
		}
		var department sql.NullInt64
		if in.DepartmentID != nil {
			department = sql.NullInt64{Int64: int64(*in.DepartmentID), Valid: true}
		}
		admin := in.TenantAdmin
		err := tx.QueryRowContext(ctx, `
INSERT INTO sys_tenant_invite
  (tenant_id, email, role_ids, department_id, is_tenant_admin, token_hash, status, invited_by, expires_at,
   created_by, updated_by)
VALUES ($1, $2, $3::bigint[], $4, $5, $6, 'pending', $7, $8, $7, $7)
RETURNING id, tenant_id, email, role_ids, department_id, is_tenant_admin, status,
          invited_by, expires_at, accepted_by, accepted_at, created_at`,
			in.TenantID, email, int64Array(in.RoleIDs), department, admin,
			HashInviteToken(token), in.InvitedBy, time.Now().Add(ttl)).
			Scan(&invite.ID, &invite.TenantID, &invite.Email, &invite.RoleIDs, &department,
				&admin, &invite.Status, &invite.InvitedBy, &invite.ExpiresAt,
				&invite.AcceptedBy, &invite.AcceptedAt, &invite.CreatedAt)
		if err != nil {
			return wrapDuplicate(err)
		}
		invite.DepartmentID = nullInt64Ptr(department)
		invite.IsTenantAdmin = admin
		if invite.RoleIDs == nil {
			invite.RoleIDs = []int64{}
		}
		return nil
	})
	if err != nil {
		return Invite{}, "", err
	}
	return invite, token, nil
}

// ListInvites returns a tenant's invitations, newest first. It is the only
// read that surfaces delivery tracking; the token-resolution queries keep
// their original column list.
func (r *TenantRepository) ListInvites(ctx context.Context, tenantID uint64) ([]Invite, error) {
	var items []Invite
	err := r.sessionTx(ctx, func(ctx context.Context, tx *Tx) error {
		rows, err := tx.QueryContext(ctx, `
SELECT i.id, i.tenant_id, i.email, i.role_ids, i.department_id, i.is_tenant_admin,
       i.status, i.invited_by, i.expires_at, i.accepted_by, i.accepted_at, i.created_at,
       i.delivery_status, i.delivered_at, i.delivery_error
FROM sys_tenant_invite i
WHERE i.tenant_id = $1
ORDER BY i.id DESC`, tenantID)
		if err != nil {
			return err
		}
		defer rows.Close()
		items = make([]Invite, 0)
		for rows.Next() {
			invite, err := scanInviteWithDelivery(rows)
			if err != nil {
				return err
			}
			items = append(items, invite)
		}
		return rows.Err()
	})
	return items, err
}

// MarkInviteDelivery records the outcome of the async invite email send. It
// never touches the invite's own status: a delivery failure must not turn a
// valid invitation invalid.
func (r *TenantRepository) MarkInviteDelivery(ctx context.Context, inviteID uint64, status, sendErr string) error {
	errMsg := strings.TrimSpace(sendErr)
	if len(errMsg) > 500 {
		errMsg = errMsg[:500]
	}
	return r.platformTx(ctx, func(ctx context.Context, tx *Tx) error {
		_, err := tx.ExecContext(ctx, `
UPDATE sys_tenant_invite
SET delivery_status = $2,
    delivered_at = CASE WHEN $2 = $3 THEN NOW() ELSE delivered_at END,
    delivery_error = $4
WHERE id = $1`, inviteID, status, InviteMailSent, errMsg)
		return err
	})
}

// RevokeInvite invalidates a pending invitation.
func (r *TenantRepository) RevokeInvite(ctx context.Context, tenantID, inviteID uint64) error {
	var affected int64
	err := r.sessionTx(ctx, func(ctx context.Context, tx *Tx) error {
		result, err := tx.ExecContext(ctx, `
UPDATE sys_tenant_invite SET status = 'revoked', updated_by = $3
WHERE id = $1 AND tenant_id = $2 AND status = 'pending'`, inviteID, tenantID, actorID(ctx))
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

// InviteByToken resolves an invitation link. It runs on the platform identity:
// the visitor is not a member yet, so no tenant session can be opened.
func (r *TenantRepository) InviteByToken(ctx context.Context, token string) (Invite, error) {
	var invite Invite
	err := r.platformTx(ctx, func(ctx context.Context, tx *Tx) error {
		row := tx.QueryRowContext(ctx, `
SELECT i.id, i.tenant_id, t.name, i.email, i.role_ids, i.department_id, i.is_tenant_admin,
       i.status, i.invited_by, i.expires_at, i.accepted_by, i.accepted_at, i.created_at
FROM sys_tenant_invite i
INNER JOIN sys_tenant t ON t.id = i.tenant_id
WHERE i.token_hash = $1`, HashInviteToken(token))
		var err error
		invite, err = scanInviteWithTenant(row)
		return err
	})
	return invite, err
}

// AcceptInvite turns a pending invitation into a membership for userID.
//
// It is idempotent: accepting twice (a double-clicked link, a retried request)
// returns the same invite instead of failing, and an account that is already a
// member is not charged a second seat.
func (r *TenantRepository) AcceptInvite(ctx context.Context, token string, userID uint64, userEmail string) (Invite, error) {
	var invite Invite
	err := r.platformTx(ctx, func(ctx context.Context, tx *Tx) error {
		row := tx.QueryRowContext(ctx, `
SELECT i.id, i.tenant_id, t.name, i.email, i.role_ids, i.department_id, i.is_tenant_admin,
       i.status, i.invited_by, i.expires_at, i.accepted_by, i.accepted_at, i.created_at
FROM sys_tenant_invite i
INNER JOIN sys_tenant t ON t.id = i.tenant_id
WHERE i.token_hash = $1
FOR UPDATE OF i`, HashInviteToken(token))
		var err error
		invite, err = scanInviteWithTenant(row)
		if errors.Is(err, sql.ErrNoRows) {
			return ErrNotFound
		}
		if err != nil {
			return err
		}
		if invite.Status == InviteAccepted {
			// A retry after a successful accept: make sure the caller really
			// owns the membership before confirming anything.
			if invite.AcceptedBy != nil && *invite.AcceptedBy != userID {
				return ErrInviteInvalid
			}
			return nil
		}
		if invite.Status != InvitePending || !invite.ExpiresAt.After(time.Now()) {
			return ErrInviteInvalid
		}
		if invite.Email != "" && userEmail != "" && !strings.EqualFold(invite.Email, userEmail) {
			return ErrInviteEmailMismatch
		}
		// Re-check membership: the seat is only consumed for a new member.
		var member bool
		if err := tx.QueryRowContext(ctx, `
SELECT EXISTS (
  SELECT 1 FROM sys_tenant_member WHERE tenant_id = $1 AND user_id = $2
)`, invite.TenantID, userID).Scan(&member); err != nil {
			return err
		}
		if !member {
			if err := consumeSeat(ctx, tx, invite.TenantID); err != nil {
				return err
			}
			admin := invite.IsTenantAdmin
			if _, err := tx.ExecContext(ctx, `
INSERT INTO sys_tenant_member (tenant_id, user_id, department_id, is_tenant_admin, status,
  created_by, updated_by)
VALUES ($1, $2, $3, $4, 1, $5, $5)`,
				invite.TenantID, userID, nullableUint64(invite.DepartmentID), admin, userID); err != nil {
				return err
			}
			for _, roleID := range invite.RoleIDs {
				if _, err := tx.ExecContext(ctx, `
INSERT INTO sys_user_role (tenant_id, user_id, role_id, created_by)
VALUES ($1, $2, $3, $4) ON CONFLICT (user_id, role_id) DO NOTHING`,
					invite.TenantID, userID, roleID, userID); err != nil {
					return err
				}
			}
			if _, err := tx.ExecContext(ctx, `
UPDATE sys_user SET department_id = COALESCE(department_id, $2)
WHERE id = $1`, userID, nullableUint64(invite.DepartmentID)); err != nil {
				return err
			}
		}
		if _, err := tx.ExecContext(ctx, `
UPDATE sys_tenant_invite
SET status = 'accepted', accepted_by = $2, accepted_at = NOW(), updated_by = $2
WHERE id = $1 AND status = 'pending'`, invite.ID, userID); err != nil {
			return err
		}
		invite.Status = InviteAccepted
		invite.AcceptedBy = &userID
		now := time.Now()
		invite.AcceptedAt = &now
		return nil
	})
	return invite, err
}

// ExpireInvites moves pending invitations past their expiry to expired. It is
// called by the maintenance job.
func (r *TenantRepository) ExpireInvites(ctx context.Context) (int64, error) {
	var affected int64
	err := r.platformTx(ctx, func(ctx context.Context, tx *Tx) error {
		result, err := tx.ExecContext(ctx, `
UPDATE sys_tenant_invite SET status = 'expired'
WHERE status = 'pending' AND expires_at <= NOW()`)
		if err != nil {
			return err
		}
		affected, err = result.RowsAffected()
		return err
	})
	return affected, err
}

// ListMembers joins each membership with its global account and its role set.
func (r *TenantRepository) ListMembers(ctx context.Context, tenantID uint64) ([]Member, error) {
	var items []Member
	err := r.sessionTx(ctx, func(ctx context.Context, tx *Tx) error {
		rows, err := tx.QueryContext(ctx, `
SELECT m.user_id, u.username, u.display_name, u.email, u.mobile, m.department_id,
       m.is_tenant_admin, m.status, m.created_at,
       COALESCE((
         SELECT string_agg(r.code, ',' ORDER BY r.code)
         FROM sys_user_role ur
         INNER JOIN sys_role r ON r.id = ur.role_id AND r.deleted_at IS NULL
         WHERE ur.user_id = m.user_id AND ur.tenant_id = m.tenant_id
       ), '') AS role_codes,
       EXISTS (SELECT 1 FROM sys_platform_operator po WHERE po.user_id = m.user_id) AS is_operator
FROM sys_tenant_member m
INNER JOIN sys_user u ON u.id = m.user_id AND u.deleted_at IS NULL
WHERE m.tenant_id = $1
ORDER BY m.created_at, m.user_id`, tenantID)
		if err != nil {
			return err
		}
		defer rows.Close()
		items = make([]Member, 0)
		for rows.Next() {
			var (
				item     Member
				dept     sql.NullInt64
				admin    bool
				roleText string
			)
			if err := rows.Scan(&item.UserID, &item.Username, &item.DisplayName, &item.Email,
				&item.Mobile, &dept, &admin, &item.Status, &item.CreatedAt, &roleText, &item.IsOperator); err != nil {
				return err
			}
			item.DepartmentID = nullInt64Ptr(dept)
			item.IsTenantAdmin = admin
			item.RoleCodes = splitCodes(roleText)
			items = append(items, item)
		}
		return rows.Err()
	})
	return items, err
}

// UpdateMember applies a partial membership change. Demoting or disabling the
// last active administrator is refused: a tenant with no administrator cannot
// be recovered from inside the tenant.
func (r *TenantRepository) UpdateMember(ctx context.Context, tenantID, userID uint64, in MemberUpdate) error {
	return r.sessionTx(ctx, func(ctx context.Context, tx *Tx) error {
		if err := lockMember(ctx, tx, tenantID, userID); err != nil {
			return err
		}
		if in.IsTenantAdmin != nil && !*in.IsTenantAdmin {
			if err := ensureOtherAdmin(ctx, tx, tenantID, userID); err != nil {
				return err
			}
		}
		if in.Status != nil && *in.Status != 1 {
			var admin bool
			if err := tx.QueryRowContext(ctx, `
SELECT is_tenant_admin FROM sys_tenant_member
WHERE tenant_id = $1 AND user_id = $2`, tenantID, userID).Scan(&admin); err != nil {
				return err
			}
			if admin {
				if err := ensureOtherAdmin(ctx, tx, tenantID, userID); err != nil {
					return err
				}
			}
		}
		if in.Status != nil {
			if _, err := tx.ExecContext(ctx, `
UPDATE sys_tenant_member SET status = $3, updated_at = NOW(), updated_by = $4
WHERE tenant_id = $1 AND user_id = $2`, tenantID, userID, *in.Status, actorID(ctx)); err != nil {
				return err
			}
		}
		if in.IsTenantAdmin != nil {
			admin := *in.IsTenantAdmin
			if _, err := tx.ExecContext(ctx, `
UPDATE sys_tenant_member SET is_tenant_admin = $3, updated_at = NOW(), updated_by = $4
WHERE tenant_id = $1 AND user_id = $2`, tenantID, userID, admin, actorID(ctx)); err != nil {
				return err
			}
		}
		if in.DepartmentID != nil {
			if _, err := tx.ExecContext(ctx, `
UPDATE sys_tenant_member SET department_id = $3, updated_at = NOW(), updated_by = $4
WHERE tenant_id = $1 AND user_id = $2`, tenantID, userID, *in.DepartmentID, actorID(ctx)); err != nil {
				return err
			}
		}
		if in.RoleIDs != nil {
			if _, err := tx.ExecContext(ctx, `
DELETE FROM sys_user_role WHERE tenant_id = $1 AND user_id = $2`, tenantID, userID); err != nil {
				return err
			}
			for _, roleID := range in.RoleIDs {
				if _, err := tx.ExecContext(ctx, `
INSERT INTO sys_user_role (tenant_id, user_id, role_id, created_by)
SELECT $1, $2, id, $4 FROM sys_role
WHERE id = $3 AND tenant_id = $1 AND deleted_at IS NULL
ON CONFLICT (user_id, role_id) DO NOTHING`, tenantID, userID, roleID, actorID(ctx)); err != nil {
					return err
				}
			}
		}
		return nil
	})
}

// RemoveMember drops the membership (and its tenant-scoped role grants) and
// releases the seat. The global account survives: it may belong to other
// tenants, and the person may be invited again later.
func (r *TenantRepository) RemoveMember(ctx context.Context, tenantID, userID uint64) error {
	return r.sessionTx(ctx, func(ctx context.Context, tx *Tx) error {
		if err := lockMember(ctx, tx, tenantID, userID); err != nil {
			return err
		}
		var admin bool
		if err := tx.QueryRowContext(ctx, `
SELECT is_tenant_admin FROM sys_tenant_member
WHERE tenant_id = $1 AND user_id = $2`, tenantID, userID).Scan(&admin); err != nil {
			return err
		}
		if admin {
			if err := ensureOtherAdmin(ctx, tx, tenantID, userID); err != nil {
				return err
			}
		}
		if _, err := tx.ExecContext(ctx, `
DELETE FROM sys_user_role WHERE tenant_id = $1 AND user_id = $2`, tenantID, userID); err != nil {
			return err
		}
		result, err := tx.ExecContext(ctx, `
DELETE FROM sys_tenant_member WHERE tenant_id = $1 AND user_id = $2`, tenantID, userID)
		if err != nil {
			return err
		}
		affected, err := result.RowsAffected()
		if err != nil {
			return err
		}
		if affected != 1 {
			return ErrNotFound
		}
		if _, err := tx.ExecContext(ctx, `
UPDATE sys_tenant_quota
SET used_value = GREATEST(used_value - 1, 0), updated_at = NOW()
WHERE tenant_id = $1 AND quota_key = 'seats'`, tenantID); err != nil {
			return err
		}
		// The removed member's own sessions must not keep the tenant they just
		// lost. Bumping the token version invalidates every access token; the
		// refresh path re-derives the tenant anyway.
		_, err = tx.ExecContext(ctx, `
UPDATE sys_user SET token_version = token_version + 1 WHERE id = $1`, userID)
		return err
	})
}

func lockMember(ctx context.Context, tx *Tx, tenantID, userID uint64) error {
	var id uint64
	err := tx.QueryRowContext(ctx, `
SELECT user_id FROM sys_tenant_member
WHERE tenant_id = $1 AND user_id = $2 FOR UPDATE`, tenantID, userID).Scan(&id)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrNotFound
	}
	return err
}

// ensureOtherAdmin fails when userID is the tenant's only active administrator.
func ensureOtherAdmin(ctx context.Context, tx *Tx, tenantID, userID uint64) error {
	var others int
	if err := tx.QueryRowContext(ctx, `
SELECT COUNT(*) FROM sys_tenant_member
WHERE tenant_id = $1 AND user_id <> $2 AND status = 1 AND is_tenant_admin = TRUE`,
		tenantID, userID).Scan(&others); err != nil {
		return err
	}
	if others == 0 {
		return ErrLastTenantAdmin
	}
	return nil
}

// consumeSeat takes one seat when the tenant meters seats.
func consumeSeat(ctx context.Context, tx *Tx, tenantID uint64) error {
	var (
		used  uint64
		limit uint64
	)
	err := tx.QueryRowContext(ctx, `
SELECT used_value, limit_value FROM sys_tenant_quota
WHERE tenant_id = $1 AND quota_key = 'seats' FOR UPDATE`, tenantID).Scan(&used, &limit)
	if errors.Is(err, sql.ErrNoRows) {
		// No seats quota: the plan does not limit members.
		return nil
	}
	if err != nil {
		return err
	}
	if used >= limit {
		return ErrQuotaExceeded
	}
	_, err = tx.ExecContext(ctx, `
UPDATE sys_tenant_quota SET used_value = used_value + 1, updated_at = NOW()
WHERE tenant_id = $1 AND quota_key = 'seats'`, tenantID)
	return err
}

// ---------------------------------------------------------------------------
// Platform staff
// ---------------------------------------------------------------------------

// PlatformOperator is one row of the staff directory with the account it names.
type PlatformOperator struct {
	UserID      uint64    `json:"userId"`
	Username    string    `json:"username"`
	DisplayName string    `json:"displayName"`
	GrantedBy   uint64    `json:"grantedBy"`
	Note        string    `json:"note"`
	CreatedAt   time.Time `json:"createdAt"`
}

// IsPlatformOperator answers the only question the console gate needs. It runs
// on the platform identity because a super administrator may not be a member
// of the tenant whose route they are calling.
func (r *TenantRepository) IsPlatformOperator(ctx context.Context, userID uint64) (bool, error) {
	if userID == 0 {
		return false, nil
	}
	var found bool
	err := r.platformTx(ctx, func(ctx context.Context, tx *Tx) error {
		return tx.QueryRowContext(ctx, `
SELECT EXISTS (SELECT 1 FROM sys_platform_operator WHERE user_id = $1)`, userID).Scan(&found)
	})
	return found, err
}

// ListPlatformOperators returns the staff directory, oldest grant first.
func (r *TenantRepository) ListPlatformOperators(ctx context.Context) ([]PlatformOperator, error) {
	var items []PlatformOperator
	err := r.platformTx(ctx, func(ctx context.Context, tx *Tx) error {
		rows, err := tx.QueryContext(ctx, `
SELECT po.user_id, u.username, u.display_name, po.granted_by, po.note, po.created_at
FROM sys_platform_operator po
INNER JOIN sys_user u ON u.id = po.user_id AND u.deleted_at IS NULL
ORDER BY po.created_at, po.user_id`)
		if err != nil {
			return err
		}
		defer rows.Close()
		items = make([]PlatformOperator, 0)
		for rows.Next() {
			var item PlatformOperator
			if err := rows.Scan(&item.UserID, &item.Username, &item.DisplayName,
				&item.GrantedBy, &item.Note, &item.CreatedAt); err != nil {
				return err
			}
			items = append(items, item)
		}
		return rows.Err()
	})
	return items, err
}

// GrantPlatformOperator adds one account to the staff directory. Granting twice
// is a no-op that keeps the original granted_by.
func (r *TenantRepository) GrantPlatformOperator(ctx context.Context, userID, grantedBy uint64, note string) error {
	if userID == 0 {
		return ErrNotFound
	}
	return r.platformTx(ctx, func(ctx context.Context, tx *Tx) error {
		var exists bool
		if err := tx.QueryRowContext(ctx, `
SELECT EXISTS (SELECT 1 FROM sys_user WHERE id = $1 AND deleted_at IS NULL)`, userID).Scan(&exists); err != nil {
			return err
		}
		if !exists {
			return ErrNotFound
		}
		_, err := tx.ExecContext(ctx, `
INSERT INTO sys_platform_operator (user_id, granted_by, note, created_by)
VALUES ($1, $2, $3, $2)
ON CONFLICT (user_id) DO NOTHING`, userID, grantedBy, clipRunes(note, 255))
		return err
	})
}

// RevokePlatformOperator removes one account from the staff directory.
func (r *TenantRepository) RevokePlatformOperator(ctx context.Context, userID uint64) error {
	var affected int64
	err := r.platformTx(ctx, func(ctx context.Context, tx *Tx) error {
		result, err := tx.ExecContext(ctx,
			`DELETE FROM sys_platform_operator WHERE user_id = $1`, userID)
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

// ---------------------------------------------------------------------------
// Usage metering
// ---------------------------------------------------------------------------

// AddUsage increments a usage counter and returns the new value. It runs in the
// caller's tenant session, so RLS keeps a tenant from writing another's counter.
func (r *TenantRepository) AddUsage(ctx context.Context, tenantID uint64, quotaKey, periodKey string, amount uint64) (uint64, error) {
	if tenantID == 0 || quotaKey == "" || periodKey == "" {
		return 0, ErrNotFound
	}
	var used uint64
	err := r.sessionTx(ctx, func(ctx context.Context, tx *Tx) error {
		return tx.QueryRowContext(ctx, `
INSERT INTO sys_usage_counter (tenant_id, quota_key, period_key, used_value)
VALUES ($1, $2, $3, $4)
ON CONFLICT (tenant_id, quota_key, period_key) DO UPDATE
SET used_value = sys_usage_counter.used_value + EXCLUDED.used_value,
    updated_at = NOW()
RETURNING used_value`, tenantID, quotaKey, periodKey, amount).Scan(&used)
	})
	return used, err
}

// Usage returns one counter. sql.ErrNoRows means it was never incremented.
func (r *TenantRepository) Usage(ctx context.Context, tenantID uint64, quotaKey, periodKey string) (uint64, error) {
	var used uint64
	err := r.sessionTx(ctx, func(ctx context.Context, tx *Tx) error {
		return tx.QueryRowContext(ctx, `
SELECT used_value FROM sys_usage_counter
WHERE tenant_id = $1 AND quota_key = $2 AND period_key = $3`,
			tenantID, quotaKey, periodKey).Scan(&used)
	})
	return used, err
}

// QuotaLimit returns the configured limit for a quota key. found is false when
// the tenant has no such quota row, which means "unlimited".
func (r *TenantRepository) QuotaLimit(ctx context.Context, tenantID uint64, quotaKey string) (uint64, bool, error) {
	var limit uint64
	found := false
	err := r.sessionTx(ctx, func(ctx context.Context, tx *Tx) error {
		err := tx.QueryRowContext(ctx, `
SELECT limit_value FROM sys_tenant_quota WHERE tenant_id = $1 AND quota_key = $2`,
			tenantID, quotaKey).Scan(&limit)
		if errors.Is(err, sql.ErrNoRows) {
			return nil
		}
		if err != nil {
			return err
		}
		found = true
		return nil
	})
	return limit, found, err
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func newInviteToken() (string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("generate invite token: %w", err)
	}
	return hex.EncodeToString(buf), nil
}

// HashInviteToken is the only form of the token that is stored.
func HashInviteToken(token string) string {
	sum := sha256.Sum256([]byte(strings.TrimSpace(token)))
	return hex.EncodeToString(sum[:])
}

func int64Array(values []int64) []int64 {
	if values == nil {
		return []int64{}
	}
	return values
}

func splitCodes(value string) []string {
	if strings.TrimSpace(value) == "" {
		return []string{}
	}
	parts := strings.Split(value, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		if trimmed := strings.TrimSpace(part); trimmed != "" {
			out = append(out, trimmed)
		}
	}
	return out
}

func nullableUint64(v *uint64) sql.NullInt64 {
	if v == nil {
		return sql.NullInt64{}
	}
	return sql.NullInt64{Int64: int64(*v), Valid: true}
}

func nullUint64(v uint64) sql.NullInt64 {
	if v == 0 {
		return sql.NullInt64{}
	}
	return sql.NullInt64{Int64: int64(v), Valid: true}
}

func scanInvite(scanner interface{ Scan(...any) error }) (Invite, error) {
	var (
		item Invite
		dept sql.NullInt64
		adm  bool
	)
	err := scanner.Scan(&item.ID, &item.TenantID, &item.Email, &item.RoleIDs, &dept,
		&adm, &item.Status, &item.InvitedBy, &item.ExpiresAt, &item.AcceptedBy,
		&item.AcceptedAt, &item.CreatedAt)
	if err != nil {
		return Invite{}, err
	}
	item.DepartmentID = nullInt64Ptr(dept)
	item.IsTenantAdmin = adm
	if item.RoleIDs == nil {
		item.RoleIDs = []int64{}
	}
	return item, nil
}

func scanInviteWithTenant(scanner interface{ Scan(...any) error }) (Invite, error) {
	var (
		item     Invite
		dept     sql.NullInt64
		adm      bool
		accepted sql.NullInt64
		at       sql.NullTime
	)
	err := scanner.Scan(&item.ID, &item.TenantID, &item.TenantName, &item.Email, &item.RoleIDs,
		&dept, &adm, &item.Status, &item.InvitedBy, &item.ExpiresAt, &accepted, &at, &item.CreatedAt)
	if err != nil {
		return Invite{}, err
	}
	item.DepartmentID = nullInt64Ptr(dept)
	item.AcceptedBy = nullInt64Ptr(accepted)
	item.AcceptedAt = nullTimePtr(at)
	item.IsTenantAdmin = adm
	if item.RoleIDs == nil {
		item.RoleIDs = []int64{}
	}
	return item, nil
}

// scanInviteWithDelivery reads the ListInvites row shape: the base columns
// plus delivery tracking.
func scanInviteWithDelivery(scanner interface{ Scan(...any) error }) (Invite, error) {
	var (
		item     Invite
		dept     sql.NullInt64
		adm      bool
		accepted sql.NullInt64
		at       sql.NullTime
		at2      sql.NullTime
	)
	err := scanner.Scan(&item.ID, &item.TenantID, &item.Email, &item.RoleIDs,
		&dept, &adm, &item.Status, &item.InvitedBy, &item.ExpiresAt, &accepted, &at, &item.CreatedAt,
		&item.DeliveryStatus, &at2, &item.DeliveryError)
	if err != nil {
		return Invite{}, err
	}
	item.DepartmentID = nullInt64Ptr(dept)
	item.AcceptedBy = nullInt64Ptr(accepted)
	item.AcceptedAt = nullTimePtr(at)
	item.DeliveredAt = nullTimePtr(at2)
	item.IsTenantAdmin = adm
	if item.RoleIDs == nil {
		item.RoleIDs = []int64{}
	}
	return item, nil
}

