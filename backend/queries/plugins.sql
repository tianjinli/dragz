-- Queries backing the plugin center: catalog sync, runtime state, migration
-- bookkeeping and the idempotent seeds for the plugin-center permission/menu.
--
-- The bootstrap DDL lives in internal/store/plugin.go (it runs at startup
-- against both fresh and existing databases), so it is intentionally not
-- expressed here.

-- name: UpsertPlugin :exec
-- Refreshes the catalog row from the compiled-in declaration.
-- enabled is only initialised on INSERT (ship dark: a new non-builtin plugin
-- starts disabled), and builtins are forced on. An existing non-builtin keeps
-- whatever the administrator chose.
INSERT INTO sys_plugin (plugin_id, name, version, description, is_builtin, enabled)
VALUES ($1, $2, $3, $4, $5, $5)
ON CONFLICT (plugin_id) DO UPDATE
SET name = EXCLUDED.name,
    version = EXCLUDED.version,
    description = EXCLUDED.description,
    is_builtin = EXCLUDED.is_builtin,
    enabled = CASE WHEN EXCLUDED.is_builtin THEN TRUE ELSE sys_plugin.enabled END,
    updated_at = NOW();

-- name: ListPlugins :many
SELECT plugin_id, name, version, description, is_builtin, enabled, last_error, enabled_at,
  created_at, updated_at
FROM sys_plugin
ORDER BY plugin_id;

-- name: GetPlugin :one
SELECT plugin_id, name, version, description, is_builtin, enabled, last_error, enabled_at,
  created_at, updated_at
FROM sys_plugin
WHERE plugin_id = $1;

-- name: EnablePlugin :execrows
-- Builtins are excluded in SQL as well as in Go: defense in depth, so a bug in
-- the service cannot brick the platform plugin.
UPDATE sys_plugin
SET enabled = TRUE, last_error = '', enabled_at = NOW(), updated_at = NOW()
WHERE plugin_id = $1 AND is_builtin = FALSE;

-- name: DisablePlugin :execrows
UPDATE sys_plugin
SET enabled = FALSE, updated_at = NOW()
WHERE plugin_id = $1 AND is_builtin = FALSE;

-- name: SetPluginError :exec
-- Records why a plugin is not running. Used for startup auto-disable and for a
-- failed enable attempt.
UPDATE sys_plugin
SET last_error = $2, updated_at = NOW()
WHERE plugin_id = $1;

-- name: ListPluginMigrations :many
SELECT version, checksum FROM sys_plugin_migration
WHERE plugin_id = $1
ORDER BY version;

-- name: RecordPluginMigration :exec
INSERT INTO sys_plugin_migration (plugin_id, version, name, checksum)
VALUES ($1, $2, $3, $4);

-- name: CountPluginMigrations :one
SELECT COUNT(*) FROM sys_plugin_migration;

-- ---- idempotent seeds for the plugin center itself (core, plugin_id = '') ----

-- name: SeedPluginMenu :exec
-- id 87 sits above the core bands (1-63) and below the plugin-owned range
-- (1000+), which keeps both sides free to grow.
-- plugin_id is left at its '' default: the plugin center is core, so it must
-- stay visible even when every non-builtin plugin is off.
INSERT INTO sys_menu
  (id, parent_id, type, name, code, route_name, route_path, component, redirect, icon,
   sort, status, visible, keep_alive, affix_tab, external_url)
VALUES
  (87, 20, 'page', '插件中心', 'system_plugin', 'SystemPlugin', '/settings/plugins',
   '/system/plugin/index', '', 'lucide:puzzle', 24, 1, 1, 1, 0, '')
ON CONFLICT DO NOTHING;

-- name: SeedPluginRoleMenu :exec
INSERT INTO sys_role_menu (role_id, menu_id)
SELECT 1, id FROM sys_menu WHERE code = 'system_plugin' AND deleted_at IS NULL
ON CONFLICT DO NOTHING;
