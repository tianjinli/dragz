package store

import (
	"context"
	"database/sql"
	"time"
)

// DefaultTenantID is the seeded tenant. Purge never deletes it.
const DefaultTenantID uint64 = 1

// tenantExportLimit caps each collection in the synchronous JSON snapshot.
// TODO: async zip export for tenants that do not fit in one response.
const tenantExportLimit = 200

// TenantPurgeAllowed reports whether a purge job may delete this tenant.
// Tenant 1 is the default tenant and is never eligible.
func TenantPurgeAllowed(tenantID uint64) bool {
	return tenantID > DefaultTenantID
}

// TenantMemberSnapshot is one membership row in an export.
type TenantMemberSnapshot struct {
	UserID        uint64  `json:"userId"`
	Username      string  `json:"username"`
	DisplayName   string  `json:"displayName"`
	DepartmentID  *uint64 `json:"departmentId,omitempty"`
	IsTenantAdmin int16   `json:"isTenantAdmin"`
	Status        int16   `json:"status"`
}

// TenantDepartmentSnapshot is one live department in an export.
type TenantDepartmentSnapshot struct {
	ID       uint64 `json:"id"`
	Code     string `json:"code"`
	Name     string `json:"name"`
	ParentID uint64 `json:"parentId"`
	Status   int16  `json:"status"`
}

// TenantRoleSnapshot is one live role in an export.
type TenantRoleSnapshot struct {
	ID     uint64 `json:"id"`
	Code   string `json:"code"`
	Name   string `json:"name"`
	Status int16  `json:"status"`
}

// TenantSubscriptionSnapshot is the latest subscription row, including a
// cancelled one.
type TenantSubscriptionSnapshot struct {
	ID          uint64  `json:"id"`
	PlanID      uint64  `json:"planId"`
	Status      string  `json:"status"`
	StartedAt   string  `json:"startedAt"`
	EndedAt     *string `json:"endedAt,omitempty"`
	TrialEndsAt *string `json:"trialEndsAt,omitempty"`
	GraceEndsAt *string `json:"graceEndsAt,omitempty"`
}

// TenantSnapshot is the synchronous export body.
// TODO: async zip export can replace this JSON snapshot for large tenants.
type TenantSnapshot struct {
	Tenant       PlatformTenant              `json:"tenant"`
	Members      []TenantMemberSnapshot      `json:"members"`
	Departments  []TenantDepartmentSnapshot  `json:"departments"`
	Roles        []TenantRoleSnapshot        `json:"roles"`
	Subscription *TenantSubscriptionSnapshot `json:"subscription"`
}

// ExportTenant returns a bounded JSON snapshot. A missing tenant is ErrNotFound.
func (r *TenantRepository) ExportTenant(ctx context.Context, tenantID uint64) (TenantSnapshot, error) {
	var snap TenantSnapshot
	err := r.platformTx(ctx, func(ctx context.Context, tx *Tx) error {
		tenant, err := loadPlatformTenant(ctx, tx, tenantID)
		if err != nil {
			return err
		}
		snap.Tenant = tenant
		members, err := tx.QueryContext(ctx, `
SELECT m.user_id, COALESCE(u.username, ''), COALESCE(u.display_name, ''),
       m.department_id, m.is_tenant_admin, m.status
FROM sys_tenant_member m
LEFT JOIN sys_user u ON u.id = m.user_id AND u.deleted_at IS NULL
WHERE m.tenant_id = $1
ORDER BY m.user_id
LIMIT $2`, tenantID, tenantExportLimit)
		if err != nil {
			return err
		}
		defer members.Close()
		snap.Members = make([]TenantMemberSnapshot, 0)
		for members.Next() {
			var (
				item TenantMemberSnapshot
				dept sql.NullInt64
			)
			if err := members.Scan(&item.UserID, &item.Username, &item.DisplayName, &dept, &item.IsTenantAdmin, &item.Status); err != nil {
				return err
			}
			if dept.Valid && dept.Int64 > 0 {
				id := uint64(dept.Int64)
				item.DepartmentID = &id
			}
			snap.Members = append(snap.Members, item)
		}
		if err := members.Err(); err != nil {
			return err
		}
		departments, err := tx.QueryContext(ctx, `
SELECT id, code, name, parent_id, status
FROM sys_department
WHERE tenant_id = $1 AND deleted_at IS NULL
ORDER BY id
LIMIT $2`, tenantID, tenantExportLimit)
		if err != nil {
			return err
		}
		defer departments.Close()
		snap.Departments = make([]TenantDepartmentSnapshot, 0)
		for departments.Next() {
			var item TenantDepartmentSnapshot
			if err := departments.Scan(&item.ID, &item.Code, &item.Name, &item.ParentID, &item.Status); err != nil {
				return err
			}
			snap.Departments = append(snap.Departments, item)
		}
		if err := departments.Err(); err != nil {
			return err
		}
		roles, err := tx.QueryContext(ctx, `
SELECT id, code, name, status
FROM sys_role
WHERE tenant_id = $1 AND deleted_at IS NULL
ORDER BY id
LIMIT $2`, tenantID, tenantExportLimit)
		if err != nil {
			return err
		}
		defer roles.Close()
		snap.Roles = make([]TenantRoleSnapshot, 0)
		for roles.Next() {
			var item TenantRoleSnapshot
			if err := roles.Scan(&item.ID, &item.Code, &item.Name, &item.Status); err != nil {
				return err
			}
			snap.Roles = append(snap.Roles, item)
		}
		if err := roles.Err(); err != nil {
			return err
		}
		var (
			sub         TenantSubscriptionSnapshot
			startedAt   sql.NullTime
			endedAt     sql.NullTime
			trialEndsAt sql.NullTime
			graceEndsAt sql.NullTime
		)
		err = tx.QueryRowContext(ctx, `
SELECT id, plan_id, status, started_at, ended_at, trial_ends_at, grace_ends_at
FROM sys_tenant_subscription
WHERE tenant_id = $1
ORDER BY id DESC
LIMIT 1`, tenantID).Scan(&sub.ID, &sub.PlanID, &sub.Status, &startedAt, &endedAt, &trialEndsAt, &graceEndsAt)
		if errorsIsNoRows(err) {
			return nil
		}
		if err != nil {
			return err
		}
		if startedAt.Valid {
			sub.StartedAt = startedAt.Time.UTC().Format(time.RFC3339)
		}
		sub.EndedAt = formatNullTime(endedAt)
		sub.TrialEndsAt = formatNullTime(trialEndsAt)
		sub.GraceEndsAt = formatNullTime(graceEndsAt)
		snap.Subscription = &sub
		return nil
	})
	return snap, err
}

// ScheduleTenantPurge marks the tenant for deletion after days and suspends it.
// Tenant 1 is ErrProtected. A missing or already-purged tenant is ErrNotFound.
func (r *TenantRepository) ScheduleTenantPurge(ctx context.Context, tenantID uint64, days int) error {
	if !TenantPurgeAllowed(tenantID) {
		return ErrProtected
	}
	if days < 1 || days > 3650 {
		return ErrInvalidPurgeWindow
	}
	var affected int64
	err := r.platformTx(ctx, func(ctx context.Context, tx *Tx) error {
		result, err := tx.ExecContext(ctx, `
UPDATE sys_tenant
SET purge_status = 'pending_delete',
    purge_after = NOW() + make_interval(days => $2),
    status = 0,
    updated_at = NOW()
WHERE id = $1 AND deleted_at IS NULL AND purge_status <> 'purged'`, tenantID, days)
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

// CancelTenantPurge clears a future pending delete. A missing tenant is
// ErrNotFound. A purge that is not pending, or whose deadline has passed, is
// ErrPurgeNotCancelable. Status is left as the operator set it.
func (r *TenantRepository) CancelTenantPurge(ctx context.Context, tenantID uint64) error {
	var affected int64
	err := r.platformTx(ctx, func(ctx context.Context, tx *Tx) error {
		result, err := tx.ExecContext(ctx, `
UPDATE sys_tenant
SET purge_status = '',
    purge_after = NULL,
    updated_at = NOW()
WHERE id = $1
  AND deleted_at IS NULL
  AND purge_status = 'pending_delete'
  AND purge_after IS NOT NULL
  AND purge_after > NOW()`, tenantID)
		if err != nil {
			return err
		}
		affected, err = result.RowsAffected()
		if err != nil || affected == 1 {
			return err
		}
		var exists bool
		if err := tx.QueryRowContext(ctx, `
SELECT EXISTS (SELECT 1 FROM sys_tenant WHERE id = $1 AND deleted_at IS NULL)`, tenantID).Scan(&exists); err != nil {
			return err
		}
		if !exists {
			return ErrNotFound
		}
		return ErrPurgeNotCancelable
	})
	return err
}

// PurgeDueTenants deletes tenant-scoped data for every tenant whose purge
// deadline has arrived, then soft-deletes the tenant row. Tenant 1 is skipped
// even if a row claims to be due. Global sys_user accounts are not deleted.
func (r *TenantRepository) PurgeDueTenants(ctx context.Context) (int, error) {
	var purged int
	err := r.platformTx(ctx, func(ctx context.Context, tx *Tx) error {
		rows, err := tx.QueryContext(ctx, `
SELECT id
FROM sys_tenant
WHERE deleted_at IS NULL
  AND purge_status = 'pending_delete'
  AND purge_after IS NOT NULL
  AND purge_after <= NOW()
  AND id <> 1
ORDER BY id`)
		if err != nil {
			return err
		}
		defer rows.Close()
		var ids []uint64
		for rows.Next() {
			var id int64
			if err := rows.Scan(&id); err != nil {
				return err
			}
			if id > 0 {
				ids = append(ids, uint64(id))
			}
		}
		if err := rows.Err(); err != nil {
			return err
		}
		for _, id := range ids {
			if !TenantPurgeAllowed(id) {
				continue
			}
			if err := purgeTenantData(ctx, tx, id); err != nil {
				return err
			}
			purged++
		}
		return nil
	})
	return purged, err
}

// purgeTenantStatements remove one tenant's rows. Tables with deleted_at are
// soft-deleted so a later row can reuse the code. Junction and billing rows
// are removed. sys_user is intentionally absent.
var purgeTenantStatements = []string{
	`UPDATE sys_department SET deleted_at = NOW(), updated_at = NOW() WHERE tenant_id = $1 AND deleted_at IS NULL`,
	`UPDATE sys_role SET deleted_at = NOW(), updated_at = NOW() WHERE tenant_id = $1 AND deleted_at IS NULL`,
	`DELETE FROM sys_user_role WHERE tenant_id = $1`,
	`DELETE FROM sys_role_menu WHERE tenant_id = $1`,
	`UPDATE sys_notice SET deleted_at = NOW(), updated_at = NOW() WHERE tenant_id = $1 AND deleted_at IS NULL`,
	`UPDATE sys_file SET deleted_at = NOW(), updated_at = NOW(), status = 'purged' WHERE tenant_id = $1 AND deleted_at IS NULL`,
	`UPDATE sys_webhook SET deleted_at = NOW(), updated_at = NOW() WHERE tenant_id = $1 AND deleted_at IS NULL`,
	`DELETE FROM sys_api_token WHERE tenant_id = $1`,
	`DELETE FROM sys_notification WHERE tenant_id = $1`,
	`DELETE FROM sys_outbox WHERE tenant_id = $1`,
	`DELETE FROM sys_tenant_member WHERE tenant_id = $1`,
	`DELETE FROM sys_tenant_subscription WHERE tenant_id = $1`,
	`DELETE FROM sys_tenant_quota WHERE tenant_id = $1`,
	`DELETE FROM sys_tenant_entitlement WHERE tenant_id = $1`,
	`DELETE FROM sys_invoice WHERE tenant_id = $1`,
	`DELETE FROM sys_tenant_domain WHERE tenant_id = $1`,
}

func purgeTenantData(ctx context.Context, tx *Tx, tenantID uint64) error {
	if !TenantPurgeAllowed(tenantID) {
		return ErrProtected
	}
	for _, statement := range purgeTenantStatements {
		if _, err := tx.ExecContext(ctx, statement, tenantID); err != nil {
			return err
		}
	}
	result, err := tx.ExecContext(ctx, `
UPDATE sys_tenant
SET deleted_at = NOW(), purge_status = 'purged', status = 0, updated_at = NOW()
WHERE id = $1 AND id <> 1 AND deleted_at IS NULL`, tenantID)
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
	return nil
}

func loadPlatformTenant(ctx context.Context, tx *Tx, tenantID uint64) (PlatformTenant, error) {
	row := tx.QueryRowContext(ctx, `
SELECT id, code, name, status, remark, created_at, purge_after, purge_status, slug
FROM sys_tenant
WHERE id = $1 AND deleted_at IS NULL`, tenantID)
	item, err := scanPlatformTenant(row)
	if errorsIsNoRows(err) {
		return PlatformTenant{}, ErrNotFound
	}
	return item, err
}

func errorsIsNoRows(err error) bool {
	return err == sql.ErrNoRows
}

func formatNullTime(value sql.NullTime) *string {
	if !value.Valid {
		return nil
	}
	formatted := value.Time.UTC().Format(time.RFC3339)
	return &formatted
}
