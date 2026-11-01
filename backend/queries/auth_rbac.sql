-- name: ListUserRoleIDsActive :many
SELECT ur.role_id FROM sys_user_role ur
INNER JOIN sys_role r ON r.id = ur.role_id
WHERE ur.user_id = $1 AND r.status = 1 AND r.deleted_at IS NULL
ORDER BY ur.role_id;

-- name: ListUserRoles :many
-- Code and name in one pass: the profile response needs the code to look up the
-- preset translation (role.<code>) and the name as the source-language
-- fallback, so two separately ordered queries would only risk drifting apart.
SELECT r.code, r.name FROM sys_role r
INNER JOIN sys_user_role ur ON ur.role_id = r.id
WHERE ur.user_id = $1 AND r.status = 1 AND r.deleted_at IS NULL
ORDER BY r.sort, r.id;

-- name: GetDepartmentNameByID :one
-- The code comes along so the profile can render the name in the request
-- language: "dept.<code>" is the translation key, name is the source fallback.
SELECT code, name FROM sys_department
WHERE id = $1 AND deleted_at IS NULL
LIMIT 1;

-- name: ListUserPermissionCodes :many
-- Permission codes are the button nodes of the menu tree. A code is granted
-- exactly when its menu row is granted, so this is the same grant table the
-- sidebar reads. The plugin filter matches the menu queries: a disabled
-- plugin's codes disappear together with its pages.
SELECT DISTINCT m.code
FROM sys_menu m
INNER JOIN sys_role_menu rm ON rm.menu_id = m.id
INNER JOIN sys_user_role ur ON ur.role_id = rm.role_id
INNER JOIN sys_role r ON r.id = ur.role_id
LEFT JOIN sys_plugin p ON p.plugin_id = m.plugin_id
WHERE ur.user_id = $1
  AND m.type = 'button'
  AND m.status = 1
  AND m.deleted_at IS NULL
  AND r.status = 1
  AND r.deleted_at IS NULL
  AND (m.plugin_id = '' OR COALESCE(p.enabled, FALSE))
ORDER BY m.code;

-- name: ListAllVisibleMenus :many
-- Walks down from the roots so a disabled plugin prunes its whole subtree.
-- The recursive step re-checks the plugin state at every level, and the final
-- INNER JOIN makes "reachable from an enabled root" a hard requirement: a
-- child of a disabled plugin is never promoted to a root by buildRouteTree.
-- A menu whose sys_plugin row is missing counts as disabled (COALESCE).
-- UNION (not UNION ALL) also stops the recursion if parent_id ever cycles.
WITH RECURSIVE visible_menu AS (
  SELECT m.id, m.parent_id
  FROM sys_menu m
  LEFT JOIN sys_plugin p ON p.plugin_id = m.plugin_id
  WHERE m.parent_id = 0
    AND m.status = 1
    AND m.deleted_at IS NULL
    AND m.type <> 'button'
    AND (m.plugin_id = '' OR COALESCE(p.enabled, FALSE))
  UNION
  SELECT c.id, c.parent_id
  FROM sys_menu c
  INNER JOIN visible_menu v ON c.parent_id = v.id
  LEFT JOIN sys_plugin p ON p.plugin_id = c.plugin_id
  WHERE c.status = 1
    AND c.deleted_at IS NULL
    AND c.type <> 'button'
    AND (c.plugin_id = '' OR COALESCE(p.enabled, FALSE))
)
SELECT m.id, m.parent_id, m.type, m.name, m.code, m.route_name, m.route_path, m.component, m.redirect,
  m.icon, m.sort, m.visible, m.keep_alive, m.affix_tab, m.external_url
FROM sys_menu m
INNER JOIN visible_menu v ON v.id = m.id
ORDER BY m.sort, m.id;

-- name: ListUserVisibleMenus :many
-- Two recursive sets intersected:
--   granted_menus = the user's granted menus plus their ancestors
--   visible_menu  = everything reachable from an enabled root
-- The intersection is what the sidebar may show. Filtering plugin state at
-- every level is what keeps a granted leaf from appearing when its parent
-- belongs to a disabled plugin.
WITH RECURSIVE granted_menus AS (
  SELECT DISTINCT m.id, m.parent_id
  FROM sys_menu m
  INNER JOIN sys_role_menu rm ON rm.menu_id = m.id
  INNER JOIN sys_user_role ur ON ur.role_id = rm.role_id
  INNER JOIN sys_role r ON r.id = ur.role_id
  LEFT JOIN sys_plugin p ON p.plugin_id = m.plugin_id
  WHERE ur.user_id = $1
    AND m.status = 1
    AND m.deleted_at IS NULL
    AND m.type <> 'button'
    AND r.status = 1
    AND r.deleted_at IS NULL
    AND (m.plugin_id = '' OR COALESCE(p.enabled, FALSE))
  UNION
  SELECT parent.id, parent.parent_id
  FROM sys_menu parent
  INNER JOIN granted_menus child ON child.parent_id = parent.id
  LEFT JOIN sys_plugin p ON p.plugin_id = parent.plugin_id
  WHERE parent.status = 1
    AND parent.deleted_at IS NULL
    AND parent.type <> 'button'
    AND (parent.plugin_id = '' OR COALESCE(p.enabled, FALSE))
),
visible_menu AS (
  SELECT m.id, m.parent_id
  FROM sys_menu m
  LEFT JOIN sys_plugin p ON p.plugin_id = m.plugin_id
  WHERE m.parent_id = 0
    AND m.status = 1
    AND m.deleted_at IS NULL
    AND m.type <> 'button'
    AND (m.plugin_id = '' OR COALESCE(p.enabled, FALSE))
  UNION
  SELECT c.id, c.parent_id
  FROM sys_menu c
  INNER JOIN visible_menu v ON c.parent_id = v.id
  LEFT JOIN sys_plugin p ON p.plugin_id = c.plugin_id
  WHERE c.status = 1
    AND c.deleted_at IS NULL
    AND c.type <> 'button'
    AND (c.plugin_id = '' OR COALESCE(p.enabled, FALSE))
)
SELECT DISTINCT m.id, m.parent_id, m.type, m.name, m.code, m.route_name, m.route_path,
  m.component, m.redirect, m.icon, m.sort, m.visible, m.keep_alive, m.affix_tab, m.external_url
FROM sys_menu m
INNER JOIN granted_menus gm ON gm.id = m.id
INNER JOIN visible_menu vm ON vm.id = m.id
ORDER BY m.sort, m.id;

-- name: ListUserRoleDataScopes :many
SELECT r.id, r.data_scope FROM sys_user_role ur
INNER JOIN sys_role r ON r.id = ur.role_id
WHERE ur.user_id = $1 AND r.status = 1 AND r.deleted_at IS NULL;

-- name: ListDepartmentAndChildrenIDs :many
SELECT id FROM sys_department
WHERE deleted_at IS NULL AND status = 1
  AND (id = $1 OR (',' || ancestors || ',') LIKE ('%,' || $1::text || ',%'));

-- name: ListCustomRoleDepartmentIDs :many
SELECT rd.department_id FROM sys_role_department rd
INNER JOIN sys_department d ON d.id = rd.department_id
WHERE rd.role_id = $1 AND d.status = 1 AND d.deleted_at IS NULL;
