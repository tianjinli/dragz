-- Schema hardening: constraint completeness, index hygiene, audit triggers.
--
-- The project has no migration runner: `migrations/001_system_init.sql` only
-- runs when Docker initialises a fresh volume, and every later change must also
-- be expressed in `store.bootstrapStatements` (idempotent, runs on every start).
-- This file is the written record of that change; the executable copy lives in
-- the bootstrap list. Keep the two in sync.
--
-- What this fixes and why:
--
-- 1. Foreign keys. 016 introduced tenant-composite FKs, but only in its own
--    file: a database rebuilt from 001 + bootstrap ended up with no FKs at all
--    (verified: one FK, the invite one). Every parent therefore gets
--    UNIQUE (tenant_id, id) and every child a composite FK, so a row can never
--    point at another tenant's parent.
-- 2. Contact uniqueness. sys_user.email/mobile were indexed but not unique,
--    while queries/users.sql resolves a user by `WHERE email = $1 LIMIT 1`.
--    Duplicate addresses silently bound invites to an arbitrary account.
-- 3. CHECK constraints on the 0/1 SMALLINT flags. Zero existed before; a typo
--    in 5 instead of 1 was accepted by the database.
-- 4. Dead indexes. `idx_*_deleted ON (deleted_at)` is never chosen for
--    `deleted_at IS NULL` (selectivity ~1) and only costs writes; dropped.
-- 5. Tenant-leading indexes. RLS appends `tenant_id = app_current_tenant_id()`
--    to every statement, so the hot log/notice/file paths need tenant_id first.
-- 6. updated_at. It was maintained by hand in every query; a missed SET left a
--    stale timestamp. A trigger now stamps it unless the statement set it
--    explicitly.
-- 7. Invite lifecycle timestamps: revoked_at/updated_at so revocations are
--    auditable.
BEGIN;

SELECT set_config('app.rls_bypass', 'on', true);

-- ---- 1. parent keys + composite FKs ---------------------------------------

CREATE UNIQUE INDEX IF NOT EXISTS uk_department_tenant_id ON sys_department (tenant_id, id);
CREATE UNIQUE INDEX IF NOT EXISTS uk_position_tenant_id ON sys_position (tenant_id, id);
CREATE UNIQUE INDEX IF NOT EXISTS uk_role_tenant_id ON sys_role (tenant_id, id);
CREATE UNIQUE INDEX IF NOT EXISTS uk_dict_type_tenant_id ON sys_dict_type (tenant_id, id);
CREATE UNIQUE INDEX IF NOT EXISTS uk_file_tenant_id ON sys_file (tenant_id, id);
CREATE UNIQUE INDEX IF NOT EXISTS uk_notice_tenant_id ON sys_notice (tenant_id, id);
CREATE UNIQUE INDEX IF NOT EXISTS uk_scheduled_job_tenant_id ON sys_scheduled_job (tenant_id, id);

-- Rows that already point outside their tenant cannot satisfy the new FK.
-- They are meaningless (the referenced parent is invisible under RLS), so the
-- same cleanup 016 performed is repeated here for databases that never ran it.
DELETE FROM sys_user_role ur
WHERE NOT EXISTS (SELECT 1 FROM sys_role r WHERE r.id = ur.role_id AND r.tenant_id = ur.tenant_id);
DELETE FROM sys_user_position up
WHERE NOT EXISTS (SELECT 1 FROM sys_position p WHERE p.id = up.position_id AND p.tenant_id = up.tenant_id);
DELETE FROM sys_role_department rd
WHERE NOT EXISTS (SELECT 1 FROM sys_department d WHERE d.id = rd.department_id AND d.tenant_id = rd.tenant_id);
DELETE FROM sys_dict_item di
WHERE NOT EXISTS (SELECT 1 FROM sys_dict_type dt WHERE dt.id = di.dict_type_id AND dt.tenant_id = di.tenant_id);
DELETE FROM sys_file_reference fr
WHERE NOT EXISTS (SELECT 1 FROM sys_file f WHERE f.id = fr.file_id AND f.tenant_id = fr.tenant_id);
DELETE FROM sys_notice_read nr
WHERE NOT EXISTS (SELECT 1 FROM sys_notice n WHERE n.id = nr.notice_id AND n.tenant_id = nr.tenant_id);
DELETE FROM sys_job_log jl
WHERE NOT EXISTS (SELECT 1 FROM sys_scheduled_job j WHERE j.id = jl.job_id AND j.tenant_id = jl.tenant_id);

-- ADD CONSTRAINT has no IF NOT EXISTS, hence the guarded blocks.
DO $$
BEGIN
  ALTER TABLE sys_user_role ADD CONSTRAINT fk_user_role_role_tenant
    FOREIGN KEY (tenant_id, role_id) REFERENCES sys_role (tenant_id, id) ON DELETE CASCADE;
EXCEPTION WHEN duplicate_object THEN NULL; END $$;

DO $$
BEGIN
  ALTER TABLE sys_user_position ADD CONSTRAINT fk_user_position_position_tenant
    FOREIGN KEY (tenant_id, position_id) REFERENCES sys_position (tenant_id, id) ON DELETE CASCADE;
EXCEPTION WHEN duplicate_object THEN NULL; END $$;

DO $$
BEGIN
  ALTER TABLE sys_role_department ADD CONSTRAINT fk_role_department_department_tenant
    FOREIGN KEY (tenant_id, department_id) REFERENCES sys_department (tenant_id, id) ON DELETE CASCADE;
EXCEPTION WHEN duplicate_object THEN NULL; END $$;

DO $$
BEGIN
  ALTER TABLE sys_dict_item ADD CONSTRAINT fk_dict_item_type_tenant
    FOREIGN KEY (tenant_id, dict_type_id) REFERENCES sys_dict_type (tenant_id, id) ON DELETE CASCADE;
EXCEPTION WHEN duplicate_object THEN NULL; END $$;

DO $$
BEGIN
  ALTER TABLE sys_file_reference ADD CONSTRAINT fk_file_reference_file_tenant
    FOREIGN KEY (tenant_id, file_id) REFERENCES sys_file (tenant_id, id) ON DELETE CASCADE;
EXCEPTION WHEN duplicate_object THEN NULL; END $$;

DO $$
BEGIN
  ALTER TABLE sys_notice_read ADD CONSTRAINT fk_notice_read_notice_tenant
    FOREIGN KEY (tenant_id, notice_id) REFERENCES sys_notice (tenant_id, id) ON DELETE CASCADE;
EXCEPTION WHEN duplicate_object THEN NULL; END $$;

DO $$
BEGIN
  ALTER TABLE sys_job_log ADD CONSTRAINT fk_job_log_job_tenant
    FOREIGN KEY (tenant_id, job_id) REFERENCES sys_scheduled_job (tenant_id, id) ON DELETE CASCADE;
EXCEPTION WHEN duplicate_object THEN NULL; END $$;

-- ---- 2. contact uniqueness ------------------------------------------------
-- Empty means "not set": the partial predicate keeps any number of users
-- without an address. lower() matches the case-insensitive lookup the invite
-- and OAuth paths perform.
CREATE UNIQUE INDEX IF NOT EXISTS uk_user_email
  ON sys_user (lower(email)) WHERE email <> '' AND deleted_at IS NULL;
CREATE UNIQUE INDEX IF NOT EXISTS uk_user_mobile
  ON sys_user (mobile) WHERE mobile <> '' AND deleted_at IS NULL;

-- ---- 3. flag CHECK constraints -------------------------------------------
-- Every column here is a 0/1 flag (sys_user.gender is 0/1/2). Existing rows are
-- validated, so a database holding an out-of-range flag fails loudly instead of
-- silently accepting it for ever.
DO $$
DECLARE
  r record;
BEGIN
  FOR r IN SELECT * FROM (VALUES
    ('sys_department','status'),
    ('sys_position','status'),
    ('sys_user','status'),
    ('sys_user','must_change_password'),
    ('sys_role','status'),
    ('sys_menu','status'),
    ('sys_menu','visible'),
    ('sys_menu','keep_alive'),
    ('sys_menu','affix_tab'),
    ('sys_menu','managed'),
    ('sys_dict_type','status'),
    ('sys_dict_item','status'),
    ('sys_auth_provider','auto_register'),
    ('sys_auth_provider','status'),
    ('sys_storage_config','use_https'),
    ('sys_storage_config','private_access'),
    ('sys_storage_config','status'),
    ('sys_user_notify_pref','enabled'),
    ('sys_api_token','status'),
    ('sys_login_log','success'),
    ('sys_operation_log','success'),
    ('sys_error_log','resolved'),
    ('sys_webhook','allow_private_network'),
    ('sys_webhook','status'),
    ('sys_scheduled_job','status'),
    ('sys_tenant','status'),
    ('sys_tenant_member','status'),
    ('sys_plan','status')
  ) AS v(tbl, col)
  LOOP
    IF NOT EXISTS (
      SELECT 1 FROM pg_constraint
      WHERE conname = 'ck_' || r.tbl || '_' || r.col
        AND conrelid = r.tbl::regclass
    ) THEN
      EXECUTE format(
        'ALTER TABLE %I ADD CONSTRAINT %I CHECK (%I IN (0,1))',
        r.tbl, 'ck_' || r.tbl || '_' || r.col, r.col
      );
    END IF;
  END LOOP;
END $$;

-- Non-flag ranges that stayed unguarded.
DO $$
BEGIN
  ALTER TABLE sys_user ADD CONSTRAINT ck_sys_user_gender CHECK (gender IN (0,1,2));
EXCEPTION WHEN duplicate_object THEN NULL; END $$;
DO $$
BEGIN
  ALTER TABLE sys_job_log ADD CONSTRAINT ck_sys_job_log_success
    CHECK (success IS NULL OR success IN (0,1));
EXCEPTION WHEN duplicate_object THEN NULL; END $$;
DO $$
BEGIN
  ALTER TABLE sys_outbox ADD CONSTRAINT ck_sys_outbox_status CHECK (status IN (0,1,2,3));
EXCEPTION WHEN duplicate_object THEN NULL; END $$;
DO $$
BEGIN
  ALTER TABLE sys_invoice ADD CONSTRAINT ck_sys_invoice_amount CHECK (amount_cents >= 0);
EXCEPTION WHEN duplicate_object THEN NULL; END $$;
DO $$
BEGIN
  ALTER TABLE sys_tenant_quota ADD CONSTRAINT ck_sys_tenant_quota_limit CHECK (limit_value >= 0);
EXCEPTION WHEN duplicate_object THEN NULL; END $$;

-- ---- 4. dead indexes ------------------------------------------------------
DROP INDEX IF EXISTS idx_department_deleted;
DROP INDEX IF EXISTS idx_position_deleted;
DROP INDEX IF EXISTS idx_user_deleted;
DROP INDEX IF EXISTS idx_role_deleted;
DROP INDEX IF EXISTS idx_menu_deleted;
DROP INDEX IF EXISTS idx_dict_type_deleted;
DROP INDEX IF EXISTS idx_dict_item_deleted;
DROP INDEX IF EXISTS idx_auth_provider_deleted;
DROP INDEX IF EXISTS idx_storage_deleted;
DROP INDEX IF EXISTS idx_file_deleted;
DROP INDEX IF EXISTS idx_notice_deleted;
DROP INDEX IF EXISTS idx_webhook_deleted;
DROP INDEX IF EXISTS idx_scheduled_job_deleted;

-- ---- 5. tenant-leading indexes -------------------------------------------
CREATE INDEX IF NOT EXISTS idx_notification_tenant_user
  ON sys_notification (tenant_id, user_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_operation_log_tenant_created
  ON sys_operation_log (tenant_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_error_log_tenant_created
  ON sys_error_log (tenant_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_notice_tenant_status
  ON sys_notice (tenant_id, status, publish_at);
CREATE INDEX IF NOT EXISTS idx_file_tenant_created
  ON sys_file (tenant_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_job_log_tenant_job
  ON sys_job_log (tenant_id, job_id, started_at DESC);
CREATE INDEX IF NOT EXISTS idx_file_reference_tenant_business
  ON sys_file_reference (tenant_id, business_type, business_id);
CREATE INDEX IF NOT EXISTS idx_api_token_tenant_user
  ON sys_api_token (tenant_id, user_id, status);

-- Superseded by the tenant-leading versions above.
DROP INDEX IF EXISTS idx_notification_user_created;
DROP INDEX IF EXISTS idx_file_reference_business;

-- ---- 6. updated_at trigger ------------------------------------------------
-- Only stamps when the statement left the column untouched, so an explicit
-- SET updated_at (backfills, imported history) still wins.
CREATE OR REPLACE FUNCTION app_touch_updated_at()
RETURNS trigger
LANGUAGE plpgsql
AS $$
BEGIN
  IF NEW.updated_at IS NOT DISTINCT FROM OLD.updated_at THEN
    NEW.updated_at := NOW();
  END IF;
  RETURN NEW;
END;
$$;

DO $$
DECLARE
  target text;
BEGIN
  FOR target IN
    SELECT c.relname
    FROM pg_class c
    JOIN pg_namespace n ON n.oid = c.relnamespace
    JOIN pg_attribute a ON a.attrelid = c.oid AND a.attname = 'updated_at' AND a.attnum > 0
    WHERE n.nspname = 'public' AND c.relkind = 'r'
  LOOP
    EXECUTE format('DROP TRIGGER IF EXISTS trg_touch_updated_at ON %I', target);
    EXECUTE format(
      'CREATE TRIGGER trg_touch_updated_at BEFORE UPDATE ON %I FOR EACH ROW EXECUTE FUNCTION app_touch_updated_at()',
      target
    );
  END LOOP;
END $$;

-- ---- 7. invite lifecycle timestamps --------------------------------------
ALTER TABLE sys_tenant_invite ADD COLUMN IF NOT EXISTS revoked_at TIMESTAMPTZ NULL;
ALTER TABLE sys_tenant_invite ADD COLUMN IF NOT EXISTS updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW();

COMMIT;

-- ---- runtime role (deployment, not schema) --------------------------------
-- FORCE ROW LEVEL SECURITY binds the table owner, but nothing binds a
-- SUPERUSER or BYPASSRLS role. The docker image creates POSTGRES_USER as a
-- superuser, and connecting the API with that account disables every policy
-- (verified locally: a tenant read saw another tenant's row). The API must use
-- a dedicated role, e.g.:
--
--   CREATE ROLE dragz_app LOGIN PASSWORD '...' NOSUPERUSER NOBYPASSRLS;
--   GRANT CONNECT ON DATABASE dragz_local TO dragz_app;
--   GRANT USAGE ON SCHEMA public TO dragz_app;
--   GRANT SELECT, INSERT, UPDATE, DELETE ON ALL TABLES IN SCHEMA public TO dragz_app;
--   GRANT USAGE, SELECT ON ALL SEQUENCES IN SCHEMA public TO dragz_app;
--   ALTER DEFAULT PRIVILEGES IN SCHEMA public
--     GRANT SELECT, INSERT, UPDATE, DELETE ON TABLES TO dragz_app;
--
-- Bootstrap also warns at startup when the connected role is SUPERUSER or
-- BYPASSRLS, so a misconfigured deployment is visible in the log instead of
-- only in an incident.

