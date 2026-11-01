-- name: CreateRole :one
INSERT INTO sys_role (code, name, data_scope, sort, status, remark)
VALUES ($1, $2, $3, $4, $5, $6)
RETURNING id;

-- name: UpdateRole :execrows
UPDATE sys_role SET code = $1, name = $2, data_scope = $3,
  sort = $4, status = $5, remark = $6
WHERE id = $7 AND deleted_at IS NULL AND is_system = 0;

-- name: UpdateRoleStatus :execrows
UPDATE sys_role SET status = $1
WHERE id = $2 AND deleted_at IS NULL AND is_system = 0;

-- name: GetRoleDeleteChecks :one
SELECT r.is_system,
  (SELECT COUNT(*) FROM sys_user_role ur INNER JOIN sys_user u ON u.id = ur.user_id
   WHERE ur.role_id = r.id AND u.deleted_at IS NULL) AS user_count
FROM sys_role r
WHERE r.id = $1 AND r.deleted_at IS NULL;

-- name: SoftDeleteRole :execrows
UPDATE sys_role SET deleted_at = NOW(), status = 0
WHERE id = $1 AND deleted_at IS NULL;

-- name: GetRoleDataScope :one
SELECT data_scope FROM sys_role WHERE id = $1 AND deleted_at IS NULL;

-- name: GetRoleIsSystem :one
SELECT is_system FROM sys_role WHERE id = $1 AND deleted_at IS NULL;

-- name: UpdateRoleDataScope :exec
UPDATE sys_role SET data_scope = $1 WHERE id = $2;

-- name: ListRoleMenuIDs :many
SELECT menu_id FROM sys_role_menu WHERE role_id = $1 ORDER BY menu_id;

-- name: ListRoleDepartmentIDs :many
SELECT department_id FROM sys_role_department WHERE role_id = $1 ORDER BY department_id;

-- name: DeleteRoleMenus :exec
DELETE FROM sys_role_menu WHERE role_id = $1;

-- name: DeleteRoleDepartments :exec
DELETE FROM sys_role_department WHERE role_id = $1;

-- name: InsertRoleMenu :exec
INSERT INTO sys_role_menu (role_id, menu_id) VALUES ($1, $2);

-- name: InsertRoleDepartment :exec
INSERT INTO sys_role_department (role_id, department_id) VALUES ($1, $2);

-- name: ListRolesFiltered :many
SELECT r.id, r.code, r.name, r.data_scope, r.sort,
  r.status, r.is_system, r.remark,
  (SELECT COUNT(*) FROM sys_user_role ur INNER JOIN sys_user u ON u.id = ur.user_id
   WHERE ur.role_id = r.id AND u.deleted_at IS NULL) AS user_count,
  r.created_at
FROM sys_role r
WHERE r.deleted_at IS NULL
  AND (sqlc.narg('keyword_pattern')::text IS NULL OR (r.name LIKE sqlc.narg('keyword_pattern') OR r.code LIKE sqlc.narg('keyword_pattern')))
  AND (sqlc.narg('status')::smallint IS NULL OR r.status = sqlc.narg('status'))
ORDER BY r.sort, r.id;

-- name: ListMenuResources :many
SELECT id, parent_id, type, name, code, route_name,
  route_path, component, redirect, icon, method, path, is_sensitive, managed, sort, status, visible, keep_alive,
  affix_tab, external_url, created_at
FROM sys_menu
WHERE deleted_at IS NULL
ORDER BY sort, id;

-- name: CreateMenu :one
-- managed is hardcoded: anything the admin UI creates is a normal row. Only the
-- startup sync sets managed = 1, and only for codes the route table declares.
INSERT INTO sys_menu
  (parent_id, type, name, code, route_name, route_path, component, redirect, icon,
   method, path, is_sensitive, managed, sort, status, visible, keep_alive, affix_tab, external_url)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, 0, $13, $14, $15, $16, $17, $18)
RETURNING id;

-- name: MenuParentIsDescendant :one
SELECT menu_parent_is_descendant(sqlc.arg(menu_id), sqlc.arg(parent_id));

-- name: UpdateMenu :execrows
UPDATE sys_menu SET parent_id = $1, type = $2, name = $3,
  code = $4, route_name = $5, route_path = $6, component = $7, redirect = $8, icon = $9,
  method = $10, path = $11, is_sensitive = $12,
  sort = $13, status = $14, visible = $15, keep_alive = $16, affix_tab = $17, external_url = $18
WHERE id = $19 AND deleted_at IS NULL;

-- name: UpdateMenuStatus :execrows
UPDATE sys_menu SET status = $1
WHERE id = $2 AND deleted_at IS NULL;

-- name: CountMenuChildren :one
SELECT COUNT(*) FROM sys_menu WHERE parent_id = $1 AND deleted_at IS NULL;

-- name: SoftDeleteMenu :execrows
UPDATE sys_menu SET deleted_at = NOW(), status = 0
WHERE id = $1 AND deleted_at IS NULL;

-- name: DeleteRoleMenusByMenu :exec
DELETE FROM sys_role_menu WHERE menu_id = $1;

-- name: ListCasbinPolicies :many
-- Casbin policies are the granted button nodes, mirroring
-- ListUserPermissionCodes so the enforcer and /auth/codes can never disagree.
SELECT rm.role_id, m.code
FROM sys_role_menu rm
INNER JOIN sys_menu m ON m.id = rm.menu_id
INNER JOIN sys_role r ON r.id = rm.role_id
LEFT JOIN sys_plugin p ON p.plugin_id = m.plugin_id
WHERE m.type = 'button'
  AND m.status = 1
  AND m.deleted_at IS NULL
  AND r.status = 1
  AND r.deleted_at IS NULL
  AND (m.plugin_id = '' OR COALESCE(p.enabled, FALSE))
ORDER BY rm.role_id, m.code;
