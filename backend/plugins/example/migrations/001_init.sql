-- 001_init.sql - the example plugin's whole footprint.
--
-- This file is executed as ONE transaction with no bind parameters, so it may
-- contain several statements (pgx switches to the simple protocol when there
-- are no arguments). It must never contain $1-style placeholders.
--
-- It is append-only: once applied, editing this file makes the checksum check
-- fail and the plugin refuses to enable. Add 002_*.sql instead.
--
-- 2026-10-29: the permission insert was removed together with the
-- sys_api_permission table. Permission codes are now menu buttons owned by the
-- startup sync, which reads plugin.Route.Permission and attaches the button to
-- this page (see plugin.go: MenuCode = "example_page"). Nothing here has to
-- seed permissions any more.

CREATE TABLE IF NOT EXISTS sys_example_item (
  id BIGSERIAL PRIMARY KEY,
  name VARCHAR(128) NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Menu id 1000 keeps plugin-owned menus out of the core bands (1-99, 82-86).
-- plugin_id is what makes the menu disappear when the plugin is disabled.
INSERT INTO sys_menu
  (id, parent_id, type, name, code, route_name, route_path, component, redirect, icon,
   sort, status, visible, keep_alive, affix_tab, external_url, plugin_id)
VALUES
  (1000, 0, 'page', '示例插件', 'example_page', 'ExamplePage', '/example', '/example/index',
   '', 'lucide:box', 900, 1, 1, 0, 0, '', 'example')
ON CONFLICT DO NOTHING;

INSERT INTO sys_role_menu (role_id, menu_id)
SELECT 1, id FROM sys_menu WHERE code = 'example_page' AND deleted_at IS NULL
ON CONFLICT DO NOTHING;

-- English label for the seeded page; absent-on-conflict like every seed.
INSERT INTO sys_translation (entity_key, lang, value)
VALUES ('menu.example_page', 'en', 'Example Plugin')
ON CONFLICT (entity_key, lang) DO NOTHING;

INSERT INTO sys_example_item (name)
SELECT 'hello' WHERE NOT EXISTS (SELECT 1 FROM sys_example_item);
