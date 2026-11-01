-- Permission buttons: one sys_menu row per declared permission code.
--
-- These statements belong to the startup sync (see store.SyncPermissionMenus),
-- never to the admin UI: method/path/parent_id/plugin_id are owned by the route
-- table, and only name and sort may be edited by an administrator.

-- name: ListMenuCodeIDs :many
-- Buttons are excluded: a button may never become the parent of another node.
SELECT code, id FROM sys_menu
WHERE deleted_at IS NULL AND type <> 'button';

-- name: ListButtonMenuCodes :many
-- Only rows the sync owns. A button an administrator created by hand
-- (managed = 0) must never be retired just because no route declares it.
SELECT code FROM sys_menu
WHERE type = 'button' AND deleted_at IS NULL AND managed = 1;

-- name: RevivePermissionButton :one
-- Brings an existing button row back in line with the route table, whether it
-- is live or was retired by SoftDeleteStaleButton. Reviving in place is what
-- keeps the grants: sys_role_menu points at this id, so a fresh INSERT would
-- leave them attached to a tombstone nobody can see any more.
-- name and sort are deliberately NOT touched: an administrator's rename and
-- ordering survive every restart. managed is forced to 1: a hand-made button
-- whose code later appears in the route table is adopted by the sync, because
-- the route table owns the code.
-- A page or directory holding the code is never taken over: type = 'button'
-- makes this match no row, and the caller falls back to an INSERT, which the
-- partial unique index rejects with a duplicate the caller reports as
-- ErrMenuCodeTaken.
-- One row only: uk_menu_code is unique among live rows, so an administrator
-- who deleted a button and recreated the code leaves tombstones behind. The
-- live row wins; otherwise the newest tombstone is revived, which is the one
-- whose grants are worth inheriting.
UPDATE sys_menu
SET parent_id = $1,
    method = $2,
    path = $3,
    is_sensitive = $4,
    plugin_id = $5,
    managed = 1,
    status = 1,
    deleted_at = NULL,
    updated_at = NOW()
WHERE id = (
  SELECT m.id FROM sys_menu m
  WHERE m.code = $6 AND m.type = 'button'
  ORDER BY (m.deleted_at IS NULL) DESC, m.id DESC
  LIMIT 1
)
RETURNING id;

-- name: InsertPermissionButton :exec
-- First sight of a code: no button row exists at all, not even a retired one.
INSERT INTO sys_menu
  (parent_id, type, name, code, method, path, is_sensitive, sort, status, visible, plugin_id, managed)
VALUES ($1, 'button', $2, $3, $4, $5, $6, $7, 1, 0, $8, 1);

-- name: SoftDeleteStaleButton :exec
-- Grants are intentionally left in place so a re-added route inherits them.
UPDATE sys_menu
SET deleted_at = NOW(), status = 0, updated_at = NOW()
WHERE code = $1 AND type = 'button' AND deleted_at IS NULL AND managed = 1;

-- name: GrantAllButtonsToSystemRole :exec
-- Role 1 is the built-in super administrator: its grants are not editable and
-- its users bypass Casbin, so re-granting on every boot cannot surprise anyone.
INSERT INTO sys_role_menu (role_id, menu_id)
SELECT 1, id FROM sys_menu WHERE type = 'button' AND deleted_at IS NULL
ON CONFLICT DO NOTHING;

-- name: GetMenuType :one
SELECT type FROM sys_menu WHERE id = $1 AND deleted_at IS NULL;

-- name: GetMenuManagedFields :one
-- Everything the sync owns, locked for the duration of the update transaction
-- so a concurrent sync cannot interleave with an administrator's edit.
SELECT parent_id, type, code, method, path, is_sensitive, plugin_id, status, managed
FROM sys_menu
WHERE id = $1 AND deleted_at IS NULL
FOR UPDATE;
