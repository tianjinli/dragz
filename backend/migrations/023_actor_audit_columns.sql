-- 023: created_by / updated_by audit columns for human-maintained tables.
--
-- Scope. Only tables a human edits through the admin UI and that are worth
-- showing "who created / last edited this" for. Write-once logs, machine-driven
-- queues, relation tables and counters are deliberately excluded: their rows
-- either already carry a dedicated actor column (sys_file.uploader_id,
-- sys_operation_log.user_id, sys_config_log.operator_id, ...) or have no stable
-- meaning for "last editor". Detailed change history stays in
-- sys_operation_log.
--
-- Conventions:
--   * BIGINT NOT NULL DEFAULT 0. 0 means "no user context": startup bootstrap,
--     seeds, plugin migrations, scheduled jobs, payment callbacks.
--   * No foreign key. 0 is a sentinel, users can be soft-deleted, and platform
--     operators act across tenants; a FK to sys_user(id) would reject all three.
--     Display joins are LEFT JOINs and render "system" when nothing matches.
--   * No trigger. updated_at is stamped by a trigger (022), but the editor must
--     not be: system writes (last login, token version, quota counters, next run
--     time) would overwrite the human's name. Every business statement that a
--     person triggers sets updated_by explicitly.
--   * No index yet. Add (tenant_id, created_by, id) / (tenant_id, updated_by,
--     id) only when a screen actually filters by them.
--
-- Historical rows keep 0; guessing an author from sys_operation_log would be
-- wrong wherever the log was pruned. sys_notice is the one exception: its
-- creator already exists, so the initial editor is that same person.

BEGIN;

SELECT set_config('app.rls_bypass', 'on', true);

-- ---- created_by + updated_by ----------------------------------------------

ALTER TABLE sys_department       ADD COLUMN IF NOT EXISTS created_by BIGINT NOT NULL DEFAULT 0;
ALTER TABLE sys_department       ADD COLUMN IF NOT EXISTS updated_by BIGINT NOT NULL DEFAULT 0;
ALTER TABLE sys_position         ADD COLUMN IF NOT EXISTS created_by BIGINT NOT NULL DEFAULT 0;
ALTER TABLE sys_position         ADD COLUMN IF NOT EXISTS updated_by BIGINT NOT NULL DEFAULT 0;
ALTER TABLE sys_user             ADD COLUMN IF NOT EXISTS created_by BIGINT NOT NULL DEFAULT 0;
ALTER TABLE sys_user             ADD COLUMN IF NOT EXISTS updated_by BIGINT NOT NULL DEFAULT 0;
ALTER TABLE sys_role             ADD COLUMN IF NOT EXISTS created_by BIGINT NOT NULL DEFAULT 0;
ALTER TABLE sys_role             ADD COLUMN IF NOT EXISTS updated_by BIGINT NOT NULL DEFAULT 0;
ALTER TABLE sys_dict_type        ADD COLUMN IF NOT EXISTS created_by BIGINT NOT NULL DEFAULT 0;
ALTER TABLE sys_dict_type        ADD COLUMN IF NOT EXISTS updated_by BIGINT NOT NULL DEFAULT 0;
ALTER TABLE sys_dict_item        ADD COLUMN IF NOT EXISTS created_by BIGINT NOT NULL DEFAULT 0;
ALTER TABLE sys_dict_item        ADD COLUMN IF NOT EXISTS updated_by BIGINT NOT NULL DEFAULT 0;
ALTER TABLE sys_config           ADD COLUMN IF NOT EXISTS created_by BIGINT NOT NULL DEFAULT 0;
ALTER TABLE sys_config           ADD COLUMN IF NOT EXISTS updated_by BIGINT NOT NULL DEFAULT 0;
ALTER TABLE sys_auth_provider    ADD COLUMN IF NOT EXISTS created_by BIGINT NOT NULL DEFAULT 0;
ALTER TABLE sys_auth_provider    ADD COLUMN IF NOT EXISTS updated_by BIGINT NOT NULL DEFAULT 0;
ALTER TABLE sys_storage_config   ADD COLUMN IF NOT EXISTS created_by BIGINT NOT NULL DEFAULT 0;
ALTER TABLE sys_storage_config   ADD COLUMN IF NOT EXISTS updated_by BIGINT NOT NULL DEFAULT 0;
ALTER TABLE sys_webhook          ADD COLUMN IF NOT EXISTS created_by BIGINT NOT NULL DEFAULT 0;
ALTER TABLE sys_webhook          ADD COLUMN IF NOT EXISTS updated_by BIGINT NOT NULL DEFAULT 0;
ALTER TABLE sys_scheduled_job    ADD COLUMN IF NOT EXISTS created_by BIGINT NOT NULL DEFAULT 0;
ALTER TABLE sys_scheduled_job    ADD COLUMN IF NOT EXISTS updated_by BIGINT NOT NULL DEFAULT 0;
ALTER TABLE sys_translation      ADD COLUMN IF NOT EXISTS created_by BIGINT NOT NULL DEFAULT 0;
ALTER TABLE sys_translation      ADD COLUMN IF NOT EXISTS updated_by BIGINT NOT NULL DEFAULT 0;

-- ---- updated_by only ------------------------------------------------------
--
-- sys_menu: rows are mostly written by the startup route sync (0) and by
-- plugins; a manual edit still names its editor.
-- sys_tenant_entitlement / sys_tenant_quota: used_value is bumped by the request
-- path on every call, so there is no created_by and the editor is only recorded
-- when a platform operator changes the limit or the switch.

ALTER TABLE sys_menu                ADD COLUMN IF NOT EXISTS updated_by BIGINT NOT NULL DEFAULT 0;
ALTER TABLE sys_tenant_entitlement  ADD COLUMN IF NOT EXISTS updated_by BIGINT NOT NULL DEFAULT 0;
ALTER TABLE sys_tenant_quota        ADD COLUMN IF NOT EXISTS updated_by BIGINT NOT NULL DEFAULT 0;

-- ---- sys_notice -----------------------------------------------------------
--
-- created_by exists since 010. Two additions:
--   * updated_by: "last edited", distinct from published_by.
--   * revoked_by: a revocation is its own action; reusing updated_by would lose
--     the distinction between "edited and later revoked" and "revoked".

ALTER TABLE sys_notice ADD COLUMN IF NOT EXISTS updated_by BIGINT NOT NULL DEFAULT 0;
ALTER TABLE sys_notice ADD COLUMN IF NOT EXISTS revoked_by BIGINT NOT NULL DEFAULT 0;

UPDATE sys_notice SET updated_by = created_by WHERE updated_by = 0 AND created_by > 0;

COMMIT;
