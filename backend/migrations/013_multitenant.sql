-- Shared-database multi-tenant schema.
--
-- tenant_id 1 is the default tenant. Existing rows are backfilled to 1.
-- sys_config.tenant_id 0 is the platform-global value and is allowed to stay 0.
--
-- FORCE ROW LEVEL SECURITY applies to the table owner, so a session that
-- reads or writes a tenant table during bootstrap must SET app.rls_bypass=on.
-- store.bootstrapStatements does that (see PluginRepository.Bootstrap).
-- Idempotent: safe to run again on a database that already has these objects.
BEGIN;

SELECT set_config('app.rls_bypass', 'on', true);

CREATE OR REPLACE FUNCTION app_current_tenant_id()
RETURNS BIGINT
LANGUAGE sql
STABLE
AS $$
  SELECT CASE
    WHEN current_setting('app.tenant_id', true) IS NULL THEN NULL
    WHEN btrim(current_setting('app.tenant_id', true)) = '' THEN NULL
    ELSE btrim(current_setting('app.tenant_id', true))::BIGINT
  END
$$;

CREATE OR REPLACE FUNCTION app_rls_bypass()
RETURNS BOOLEAN
LANGUAGE sql
STABLE
AS $$
  SELECT lower(COALESCE(current_setting('app.rls_bypass', true), '')) IN ('on', 'true', '1')
$$;

-- 0 and NULL mean "use the session tenant". An omitted column keeps its
-- default and is not rewritten. sys_config does not use this function: 0 there
-- is the platform-global row and must be kept.
--
-- A missing session tenant raises instead of falling back to tenant 1. A
-- silent fallback writes the row into the wrong tenant while RLS bypass is on,
-- which is unrecoverable after the fact; failing the insert is not.
CREATE OR REPLACE FUNCTION app_fill_tenant_id()
RETURNS trigger
LANGUAGE plpgsql
AS $$
DECLARE
  current_tenant BIGINT;
BEGIN
  IF NEW.tenant_id IS NULL OR NEW.tenant_id = 0 THEN
    current_tenant := app_current_tenant_id();
    IF current_tenant IS NULL OR current_tenant = 0 THEN
      RAISE EXCEPTION 'app.tenant_id is not set; refusing to guess a tenant for %', TG_TABLE_NAME
        USING ERRCODE = '23502';
    END IF;
    NEW.tenant_id := current_tenant;
  END IF;
  RETURN NEW;
END;
$$;

CREATE OR REPLACE FUNCTION app_fill_config_tenant_id()
RETURNS trigger
LANGUAGE plpgsql
AS $$
DECLARE
  current_tenant BIGINT;
BEGIN
  IF NEW.tenant_id IS NULL THEN
    current_tenant := app_current_tenant_id();
    IF current_tenant IS NULL THEN
      NEW.tenant_id := 0;
    ELSE
      NEW.tenant_id := current_tenant;
    END IF;
  END IF;
  RETURN NEW;
END;
$$;

CREATE TABLE IF NOT EXISTS sys_tenant (
  id BIGSERIAL,
  code VARCHAR(64) NOT NULL,
  name VARCHAR(128) NOT NULL,
  status SMALLINT NOT NULL DEFAULT 1,
  remark VARCHAR(500) NOT NULL DEFAULT '',
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  deleted_at TIMESTAMPTZ NULL,
  PRIMARY KEY (id)
);

ALTER TABLE sys_tenant ADD COLUMN IF NOT EXISTS remark VARCHAR(500) NOT NULL DEFAULT '';

CREATE TABLE IF NOT EXISTS sys_tenant_member (
  tenant_id BIGINT NOT NULL,
  user_id BIGINT NOT NULL,
  department_id BIGINT NULL,
  is_tenant_admin SMALLINT NOT NULL DEFAULT 0,
  is_platform_operator BOOLEAN NOT NULL DEFAULT FALSE,
  status SMALLINT NOT NULL DEFAULT 1,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  PRIMARY KEY (tenant_id, user_id)
);

ALTER TABLE sys_tenant_member ADD COLUMN IF NOT EXISTS department_id BIGINT NULL;
ALTER TABLE sys_tenant_member ADD COLUMN IF NOT EXISTS is_tenant_admin SMALLINT NOT NULL DEFAULT 0;
ALTER TABLE sys_tenant_member ADD COLUMN IF NOT EXISTS is_platform_operator BOOLEAN NOT NULL DEFAULT FALSE;
ALTER TABLE sys_tenant_member ADD COLUMN IF NOT EXISTS status SMALLINT NOT NULL DEFAULT 1;
ALTER TABLE sys_tenant_member ADD COLUMN IF NOT EXISTS created_at TIMESTAMPTZ NOT NULL DEFAULT NOW();
ALTER TABLE sys_tenant_member ADD COLUMN IF NOT EXISTS updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW();

CREATE TABLE IF NOT EXISTS sys_plan (
  id BIGSERIAL,
  code VARCHAR(64) NOT NULL,
  name VARCHAR(128) NOT NULL,
  description VARCHAR(500) NOT NULL DEFAULT '',
  status SMALLINT NOT NULL DEFAULT 1,
  sort INT NOT NULL DEFAULT 0,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  PRIMARY KEY (id)
);

CREATE TABLE IF NOT EXISTS sys_plan_entitlement (
  id BIGSERIAL,
  plan_id BIGINT NOT NULL,
  entitlement_key VARCHAR(128) NOT NULL,
  entitlement_value TEXT NOT NULL DEFAULT '',
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  PRIMARY KEY (id)
);

CREATE TABLE IF NOT EXISTS sys_tenant_subscription (
  id BIGSERIAL,
  tenant_id BIGINT NOT NULL,
  plan_id BIGINT NOT NULL,
  status VARCHAR(32) NOT NULL DEFAULT 'active',
  started_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  ended_at TIMESTAMPTZ NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  PRIMARY KEY (id)
);

CREATE TABLE IF NOT EXISTS sys_usage_counter (
  tenant_id BIGINT NOT NULL,
  quota_key VARCHAR(128) NOT NULL,
  period_key VARCHAR(32) NOT NULL,
  used_value BIGINT NOT NULL DEFAULT 0,
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  PRIMARY KEY (tenant_id, quota_key, period_key)
);

CREATE TABLE IF NOT EXISTS sys_tenant_entitlement (
  tenant_id BIGINT NOT NULL,
  feature_key VARCHAR(128) NOT NULL,
  enabled BOOLEAN NOT NULL DEFAULT TRUE,
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  PRIMARY KEY (tenant_id, feature_key)
);

CREATE TABLE IF NOT EXISTS sys_tenant_quota (
  tenant_id BIGINT NOT NULL,
  quota_key VARCHAR(128) NOT NULL,
  limit_value BIGINT NOT NULL,
  used_value BIGINT NOT NULL DEFAULT 0,
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  PRIMARY KEY (tenant_id, quota_key)
);

CREATE TABLE IF NOT EXISTS sys_outbox (
  id BIGSERIAL,
  tenant_id BIGINT NOT NULL DEFAULT 1,
  event_type VARCHAR(128) NOT NULL,
  payload JSONB NOT NULL DEFAULT '{}'::jsonb,
  status SMALLINT NOT NULL DEFAULT 0,
  attempts INT NOT NULL DEFAULT 0,
  last_error VARCHAR(1000) NOT NULL DEFAULT '',
  available_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  sent_at TIMESTAMPTZ NULL,
  PRIMARY KEY (id)
);

ALTER TABLE sys_outbox ADD COLUMN IF NOT EXISTS attempts INT NOT NULL DEFAULT 0;
ALTER TABLE sys_outbox ADD COLUMN IF NOT EXISTS last_error VARCHAR(1000) NOT NULL DEFAULT '';
ALTER TABLE sys_outbox ADD COLUMN IF NOT EXISTS available_at TIMESTAMPTZ NOT NULL DEFAULT NOW();
ALTER TABLE sys_outbox ADD COLUMN IF NOT EXISTS sent_at TIMESTAMPTZ NULL;
ALTER TABLE sys_outbox ALTER COLUMN tenant_id SET DEFAULT 1;

CREATE TABLE IF NOT EXISTS sys_tenant_impersonation (
  id BIGSERIAL,
  tenant_id BIGINT NOT NULL,
  actor_user_id BIGINT NOT NULL,
  target_user_id BIGINT NOT NULL,
  reason VARCHAR(500) NOT NULL DEFAULT '',
  started_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  expires_at TIMESTAMPTZ NOT NULL,
  ended_at TIMESTAMPTZ NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  PRIMARY KEY (id)
);

-- Users, identities, refresh tokens, login logs, menus, auth providers,
-- storage config, plugins and translations stay global.
ALTER TABLE sys_refresh_token DROP COLUMN IF EXISTS tenant_id;

ALTER TABLE sys_department ADD COLUMN IF NOT EXISTS tenant_id BIGINT NOT NULL DEFAULT 1;
ALTER TABLE sys_position ADD COLUMN IF NOT EXISTS tenant_id BIGINT NOT NULL DEFAULT 1;
ALTER TABLE sys_role ADD COLUMN IF NOT EXISTS tenant_id BIGINT NOT NULL DEFAULT 1;
ALTER TABLE sys_role ALTER COLUMN tenant_id SET DEFAULT 1;
ALTER TABLE sys_user_role ADD COLUMN IF NOT EXISTS tenant_id BIGINT NOT NULL DEFAULT 1;
ALTER TABLE sys_user_position ADD COLUMN IF NOT EXISTS tenant_id BIGINT NOT NULL DEFAULT 1;
ALTER TABLE sys_role_department ADD COLUMN IF NOT EXISTS tenant_id BIGINT NOT NULL DEFAULT 1;
ALTER TABLE sys_role_menu ADD COLUMN IF NOT EXISTS tenant_id BIGINT NOT NULL DEFAULT 1;
ALTER TABLE sys_dict_type ADD COLUMN IF NOT EXISTS tenant_id BIGINT NOT NULL DEFAULT 1;
ALTER TABLE sys_dict_item ADD COLUMN IF NOT EXISTS tenant_id BIGINT NOT NULL DEFAULT 1;
ALTER TABLE sys_file ADD COLUMN IF NOT EXISTS tenant_id BIGINT NOT NULL DEFAULT 1;
ALTER TABLE sys_file_reference ADD COLUMN IF NOT EXISTS tenant_id BIGINT NOT NULL DEFAULT 1;
ALTER TABLE sys_notice ADD COLUMN IF NOT EXISTS tenant_id BIGINT NOT NULL DEFAULT 1;
ALTER TABLE sys_notice_read ADD COLUMN IF NOT EXISTS tenant_id BIGINT NOT NULL DEFAULT 1;
ALTER TABLE sys_notification ADD COLUMN IF NOT EXISTS tenant_id BIGINT NOT NULL DEFAULT 1;
ALTER TABLE sys_user_notify_pref ADD COLUMN IF NOT EXISTS tenant_id BIGINT NOT NULL DEFAULT 1;
ALTER TABLE sys_api_token ADD COLUMN IF NOT EXISTS tenant_id BIGINT NOT NULL DEFAULT 1;
ALTER TABLE sys_api_token ALTER COLUMN tenant_id SET DEFAULT 1;
ALTER TABLE sys_operation_log ADD COLUMN IF NOT EXISTS tenant_id BIGINT NOT NULL DEFAULT 1;
ALTER TABLE sys_error_log ADD COLUMN IF NOT EXISTS tenant_id BIGINT NOT NULL DEFAULT 1;
ALTER TABLE sys_webhook ADD COLUMN IF NOT EXISTS tenant_id BIGINT NOT NULL DEFAULT 1;
ALTER TABLE sys_scheduled_job ADD COLUMN IF NOT EXISTS tenant_id BIGINT NOT NULL DEFAULT 1;
ALTER TABLE sys_job_log ADD COLUMN IF NOT EXISTS tenant_id BIGINT NOT NULL DEFAULT 1;
ALTER TABLE sys_config ADD COLUMN IF NOT EXISTS tenant_id BIGINT NOT NULL DEFAULT 0;

-- 0 is not a tenant on these tables. sys_config is omitted on purpose.
DO $$
DECLARE
  target text;
BEGIN
  FOREACH target IN ARRAY ARRAY[
    'sys_department', 'sys_position', 'sys_role', 'sys_user_role', 'sys_user_position',
    'sys_role_department', 'sys_role_menu', 'sys_dict_type', 'sys_dict_item',
    'sys_file', 'sys_file_reference', 'sys_notice', 'sys_notice_read',
    'sys_notification', 'sys_user_notify_pref', 'sys_api_token',
    'sys_operation_log', 'sys_error_log', 'sys_webhook', 'sys_scheduled_job', 'sys_job_log'
  ]
  LOOP
    EXECUTE format('UPDATE %I SET tenant_id = 1 WHERE tenant_id = 0', target);
  END LOOP;
END
$$;

ALTER TABLE sys_department DROP CONSTRAINT IF EXISTS uk_department_code;
DROP INDEX IF EXISTS uk_department_code;
CREATE UNIQUE INDEX IF NOT EXISTS uk_department_code ON sys_department (tenant_id, code) WHERE deleted_at IS NULL;

ALTER TABLE sys_position DROP CONSTRAINT IF EXISTS uk_position_code;
DROP INDEX IF EXISTS uk_position_code;
CREATE UNIQUE INDEX IF NOT EXISTS uk_position_code ON sys_position (tenant_id, code) WHERE deleted_at IS NULL;

ALTER TABLE sys_role DROP CONSTRAINT IF EXISTS uk_role_code;
DROP INDEX IF EXISTS uk_role_code;
CREATE UNIQUE INDEX IF NOT EXISTS uk_role_code ON sys_role (tenant_id, code) WHERE deleted_at IS NULL;

ALTER TABLE sys_dict_type DROP CONSTRAINT IF EXISTS uk_dict_type_code;
DROP INDEX IF EXISTS uk_dict_type_code;
CREATE UNIQUE INDEX IF NOT EXISTS uk_dict_type_code ON sys_dict_type (tenant_id, code) WHERE deleted_at IS NULL;

ALTER TABLE sys_dict_item DROP CONSTRAINT IF EXISTS uk_dict_item_value;
DROP INDEX IF EXISTS uk_dict_item_value;
CREATE UNIQUE INDEX IF NOT EXISTS uk_dict_item_value ON sys_dict_item (tenant_id, dict_type_id, value) WHERE deleted_at IS NULL;

ALTER TABLE sys_scheduled_job DROP CONSTRAINT IF EXISTS uk_scheduled_job_code;
DROP INDEX IF EXISTS uk_scheduled_job_code;
CREATE UNIQUE INDEX IF NOT EXISTS uk_scheduled_job_code ON sys_scheduled_job (tenant_id, code) WHERE deleted_at IS NULL;

ALTER TABLE sys_config DROP CONSTRAINT IF EXISTS uk_config_key;
DROP INDEX IF EXISTS uk_config_key;
ALTER TABLE sys_config ADD CONSTRAINT uk_config_key UNIQUE (tenant_id, config_key);

ALTER TABLE sys_tenant DROP CONSTRAINT IF EXISTS uk_tenant_code;
DROP INDEX IF EXISTS uk_tenant_code;
CREATE UNIQUE INDEX IF NOT EXISTS uk_tenant_code ON sys_tenant (code) WHERE deleted_at IS NULL;

CREATE UNIQUE INDEX IF NOT EXISTS uk_plan_code ON sys_plan (code);
CREATE UNIQUE INDEX IF NOT EXISTS uk_plan_entitlement ON sys_plan_entitlement (plan_id, entitlement_key);
CREATE UNIQUE INDEX IF NOT EXISTS uk_tenant_subscription_active
  ON sys_tenant_subscription (tenant_id) WHERE status = 'active';
CREATE INDEX IF NOT EXISTS idx_tenant_member_user ON sys_tenant_member (user_id);
CREATE INDEX IF NOT EXISTS idx_outbox_pending ON sys_outbox (status, id);
CREATE INDEX IF NOT EXISTS idx_tenant_impersonation_open ON sys_tenant_impersonation (tenant_id, actor_user_id)
  WHERE ended_at IS NULL;

INSERT INTO sys_plan (id, code, name, description, status, sort)
SELECT 1, 'free', '免费版', '平台内置免费套餐', 1, 0
WHERE NOT EXISTS (SELECT 1 FROM sys_plan WHERE id = 1 OR code = 'free');

INSERT INTO sys_plan_entitlement (plan_id, entitlement_key, entitlement_value)
SELECT p.id, v.key, 'true'
FROM sys_plan p
CROSS JOIN (VALUES
  ('feature.apiToken.enabled'),
  ('feature.notice.enabled'),
  ('feature.fileUpload.enabled'),
  ('feature.scheduler.enabled')
) AS v(key)
WHERE p.code = 'free'
  AND NOT EXISTS (
    SELECT 1 FROM sys_plan_entitlement e
    WHERE e.plan_id = p.id AND e.entitlement_key = v.key
  );

INSERT INTO sys_plan_entitlement (plan_id, entitlement_key, entitlement_value)
SELECT p.id, 'quota.seats', '100'
FROM sys_plan p
WHERE p.code = 'free'
  AND NOT EXISTS (
    SELECT 1 FROM sys_plan_entitlement e
    WHERE e.plan_id = p.id AND e.entitlement_key = 'quota.seats'
  );

INSERT INTO sys_tenant (id, code, name, status)
SELECT 1, 'default', '默认租户', 1
WHERE NOT EXISTS (SELECT 1 FROM sys_tenant WHERE id = 1 OR code = 'default');

INSERT INTO sys_tenant_subscription (tenant_id, plan_id, status)
SELECT t.id, p.id, 'active'
FROM sys_tenant t
CROSS JOIN sys_plan p
WHERE t.id = 1 AND p.code = 'free'
  AND NOT EXISTS (
    SELECT 1 FROM sys_tenant_subscription s
    WHERE s.tenant_id = t.id AND s.status = 'active'
  );

INSERT INTO sys_tenant_member (
  tenant_id, user_id, department_id, is_tenant_admin, is_platform_operator
)
SELECT 1, u.id, u.department_id, u.is_super_admin, (u.is_super_admin <> 0)
FROM sys_user u
ON CONFLICT (tenant_id, user_id) DO NOTHING;

SELECT setval(
  pg_get_serial_sequence('sys_plan', 'id'),
  GREATEST(COALESCE((SELECT MAX(id) FROM sys_plan), 1), 1)
);
SELECT setval(
  pg_get_serial_sequence('sys_tenant', 'id'),
  GREATEST(COALESCE((SELECT MAX(id) FROM sys_tenant), 1), 1)
);

DO $$
DECLARE
  target text;
BEGIN
  FOREACH target IN ARRAY ARRAY[
    'sys_department', 'sys_position', 'sys_role', 'sys_user_role', 'sys_user_position',
    'sys_role_department', 'sys_role_menu', 'sys_dict_type', 'sys_dict_item',
    'sys_file', 'sys_file_reference', 'sys_notice', 'sys_notice_read',
    'sys_notification', 'sys_user_notify_pref', 'sys_api_token',
    'sys_operation_log', 'sys_error_log', 'sys_webhook', 'sys_scheduled_job', 'sys_job_log',
    'sys_tenant_member', 'sys_tenant_subscription', 'sys_usage_counter',
    'sys_tenant_entitlement', 'sys_tenant_quota', 'sys_outbox', 'sys_tenant_impersonation'
  ]
  LOOP
    EXECUTE format('DROP TRIGGER IF EXISTS trg_fill_tenant_id ON %I', target);
    EXECUTE format(
      'CREATE TRIGGER trg_fill_tenant_id BEFORE INSERT ON %I FOR EACH ROW EXECUTE FUNCTION app_fill_tenant_id()',
      target
    );
    EXECUTE format('ALTER TABLE %I ENABLE ROW LEVEL SECURITY', target);
    EXECUTE format('ALTER TABLE %I FORCE ROW LEVEL SECURITY', target);
    EXECUTE format('DROP POLICY IF EXISTS tenant_isolation ON %I', target);
    EXECUTE format(
      'CREATE POLICY tenant_isolation ON %I USING (app_rls_bypass() OR tenant_id = app_current_tenant_id()) WITH CHECK (app_rls_bypass() OR tenant_id = app_current_tenant_id())',
      target
    );
  END LOOP;
END
$$;

DROP TRIGGER IF EXISTS trg_fill_tenant_id ON sys_config;
DROP TRIGGER IF EXISTS trg_fill_config_tenant_id ON sys_config;
CREATE TRIGGER trg_fill_config_tenant_id
  BEFORE INSERT ON sys_config
  FOR EACH ROW EXECUTE FUNCTION app_fill_config_tenant_id();

ALTER TABLE sys_config ENABLE ROW LEVEL SECURITY;
ALTER TABLE sys_config FORCE ROW LEVEL SECURITY;
DROP POLICY IF EXISTS tenant_isolation ON sys_config;
CREATE POLICY tenant_isolation ON sys_config
  USING (app_rls_bypass() OR tenant_id = 0 OR tenant_id = app_current_tenant_id())
  WITH CHECK (app_rls_bypass() OR tenant_id = 0 OR tenant_id = app_current_tenant_id());

ALTER TABLE sys_tenant ENABLE ROW LEVEL SECURITY;
ALTER TABLE sys_tenant FORCE ROW LEVEL SECURITY;
DROP POLICY IF EXISTS tenant_isolation ON sys_tenant;
CREATE POLICY tenant_isolation ON sys_tenant
  USING (app_rls_bypass() OR id = app_current_tenant_id())
  WITH CHECK (app_rls_bypass() OR id = app_current_tenant_id());

COMMIT;
