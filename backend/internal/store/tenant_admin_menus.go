package store

import (
	"context"
	"fmt"

	"github.com/lib/pq"
)

// tenantAdminPageCodes is the built-in tenant_admin role's page allowlist.
//
// Directories are not listed here on purpose: walking down from "organization"
// would also grant system_user and system_menu. Pages are expanded to their
// button children, then walked up to the directories that make the menu tree
// renderable.
func tenantAdminPageCodes() []string {
	return []string{
		"dashboard",
		"system_member",
		"system_department",
		"system_position",
		"system_role",
		"system_config",
		"system_notice",
		"my_messages",
		"system_file",
		"system_file_recycle",
		"system_online",
		"system_token",
		"system_login_log",
		"system_operation_log",
	}
}

// syncTenantAdminRoleMenus makes every live tenant_admin role match the
// allowlist: revoke grants outside it, then insert any missing page/button/
// ancestor directory rows. Used on boot so upgrades do not leave old tenants
// with platform or menu-management rights.
func syncTenantAdminRoleMenus(ctx context.Context, tx *Tx) error {
	codes := tenantAdminPageCodes()
	if _, err := tx.ExecContext(ctx, `
WITH RECURSIVE
roots AS (
  SELECT id, parent_id FROM sys_menu
  WHERE deleted_at IS NULL AND status = 1 AND code = ANY($1)
),
descendants AS (
  SELECT id, parent_id FROM roots
  UNION
  SELECT m.id, m.parent_id FROM sys_menu m
  INNER JOIN descendants d ON m.parent_id = d.id
  WHERE m.deleted_at IS NULL AND m.status = 1
),
ancestors AS (
  SELECT id, parent_id FROM roots
  UNION
  SELECT m.id, m.parent_id FROM sys_menu m
  INNER JOIN ancestors a ON m.id = a.parent_id
  WHERE m.deleted_at IS NULL AND m.status = 1 AND a.parent_id <> 0
),
allowed AS (
  SELECT id FROM descendants
  UNION
  SELECT id FROM ancestors
)
DELETE FROM sys_role_menu rm
USING sys_role r
WHERE r.code = 'tenant_admin'
  AND r.deleted_at IS NULL
  AND rm.role_id = r.id
  AND NOT EXISTS (SELECT 1 FROM allowed a WHERE a.id = rm.menu_id)`, pq.Array(codes)); err != nil {
		return fmt.Errorf("revoke tenant_admin menus outside allowlist: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `
WITH RECURSIVE
roots AS (
  SELECT id, parent_id FROM sys_menu
  WHERE deleted_at IS NULL AND status = 1 AND code = ANY($1)
),
descendants AS (
  SELECT id, parent_id FROM roots
  UNION
  SELECT m.id, m.parent_id FROM sys_menu m
  INNER JOIN descendants d ON m.parent_id = d.id
  WHERE m.deleted_at IS NULL AND m.status = 1
),
ancestors AS (
  SELECT id, parent_id FROM roots
  UNION
  SELECT m.id, m.parent_id FROM sys_menu m
  INNER JOIN ancestors a ON m.id = a.parent_id
  WHERE m.deleted_at IS NULL AND m.status = 1 AND a.parent_id <> 0
),
allowed AS (
  SELECT id FROM descendants
  UNION
  SELECT id FROM ancestors
)
INSERT INTO sys_role_menu (tenant_id, role_id, menu_id)
SELECT r.tenant_id, r.id, a.id
FROM sys_role r
CROSS JOIN allowed a
WHERE r.code = 'tenant_admin'
  AND r.deleted_at IS NULL
ON CONFLICT (role_id, menu_id) DO NOTHING`, pq.Array(codes)); err != nil {
		return fmt.Errorf("grant tenant_admin allowlist menus: %w", err)
	}
	return nil
}

// grantTenantAdminRoleMenus seeds one freshly provisioned tenant_admin role
// with the allowlist. Prefer this over syncTenantAdminRoleMenus during signup
// so other tenants are not rewritten mid-transaction.
func grantTenantAdminRoleMenus(ctx context.Context, tx *Tx, tenantID, roleID uint64) error {
	codes := tenantAdminPageCodes()
	if _, err := tx.ExecContext(ctx, `
WITH RECURSIVE
roots AS (
  SELECT id, parent_id FROM sys_menu
  WHERE deleted_at IS NULL AND status = 1 AND code = ANY($1)
),
descendants AS (
  SELECT id, parent_id FROM roots
  UNION
  SELECT m.id, m.parent_id FROM sys_menu m
  INNER JOIN descendants d ON m.parent_id = d.id
  WHERE m.deleted_at IS NULL AND m.status = 1
),
ancestors AS (
  SELECT id, parent_id FROM roots
  UNION
  SELECT m.id, m.parent_id FROM sys_menu m
  INNER JOIN ancestors a ON m.id = a.parent_id
  WHERE m.deleted_at IS NULL AND m.status = 1 AND a.parent_id <> 0
),
allowed AS (
  SELECT id FROM descendants
  UNION
  SELECT id FROM ancestors
)
INSERT INTO sys_role_menu (tenant_id, role_id, menu_id)
SELECT $2, $3, a.id
FROM allowed a
ON CONFLICT (role_id, menu_id) DO NOTHING`, pq.Array(codes), tenantID, roleID); err != nil {
		return fmt.Errorf("grant tenant_admin menus for role %d: %w", roleID, err)
	}
	return nil
}
