package store

import (
	"context"
	"database/sql"
	"fmt"
	"regexp"
	"time"

	"dragz-admin/backend/internal/pkg/ctxkey"
	sqlcstore "dragz-admin/backend/internal/store/sqlc"
)

// migrationLockKey is the pg_advisory_lock key that serialises every schema
// mutation (bootstrap, seed, catalog sync, startup migrations, enable and
// disable) across processes. The hex spells "dragzplg".
const migrationLockKey int64 = 0x647261677A706C67

// maxLastErrorLen keeps a runaway SQL error from filling the column.
const maxLastErrorLen = 1000

// pluginIDPattern mirrors plugin.ValidID. It is duplicated instead of imported
// because store must not depend on internal/plugin (store → plugin → app →
// store). Only UninstallPlugin uses it, and only to refuse an identifier it is
// about to put in a DDL statement.
var pluginIDPattern = regexp.MustCompile(`^[a-z][a-z0-9]{2,15}$`)

// tableNamePattern is the only shape UninstallPlugin will interpolate into
// DROP TABLE. It excludes schema qualification, quoting and anything else that
// could turn a declared table name into a different statement.
var tableNamePattern = regexp.MustCompile(`^[a-z][a-z0-9_]{0,62}$`)

// PluginCatalogEntry is the neutral projection of a plugin declaration that
// the store accepts. It exists so this package does not import
// internal/plugin, which would create a cycle
// (store → plugin → app → store).
type PluginCatalogEntry struct {
	ID          string
	Name        string
	Version     string
	Description string
	Builtin     bool
}

// PluginRecord is the persisted state of one plugin.
type PluginRecord struct {
	ID          string
	Name        string
	Version     string
	Description string
	Builtin     bool
	Enabled     bool
	LastError   string
	EnabledAt   time.Time
	// InstalledAt is set by Install (and by Enable for the enable-without-prior-install path).
	// It is cleared when the catalog row is deleted on Uninstall.
	InstalledAt time.Time
	UpdatedAt   time.Time
}

// PluginRepository owns sys_plugin, sys_plugin_migration and the idempotent
// bootstrap DDL that creates them on both fresh and existing databases.
type PluginRepository struct {
	db      *DB
	queries *sqlcstore.Queries
}

// NewPluginRepository binds the generated queries to a database handle.
func NewPluginRepository(db *DB) *PluginRepository {
	return &PluginRepository{db: db, queries: sqlcstore.New(newSessionDBTX(db))}
}

// bootstrapStatements must stay idempotent: they run on every startup, against
// a brand-new database and against one created months ago.
var bootstrapStatements = []string{
	`CREATE TABLE IF NOT EXISTS sys_plugin (
  plugin_id VARCHAR(16) PRIMARY KEY,
  name VARCHAR(128) NOT NULL DEFAULT '',
  version VARCHAR(32) NOT NULL DEFAULT '',
  description VARCHAR(500) NOT NULL DEFAULT '',
  is_builtin BOOLEAN NOT NULL DEFAULT FALSE,
  enabled BOOLEAN NOT NULL DEFAULT FALSE,
  last_error TEXT NOT NULL DEFAULT '',
  enabled_at TIMESTAMPTZ NULL,
  installed_at TIMESTAMPTZ NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
)`,
	// Install marks a plugin without enabling it; older databases lack the column.
	`ALTER TABLE sys_plugin ADD COLUMN IF NOT EXISTS installed_at TIMESTAMPTZ NULL`,
	`CREATE TABLE IF NOT EXISTS sys_plugin_migration (
  plugin_id VARCHAR(16) NOT NULL,
  version VARCHAR(32) NOT NULL,
  name VARCHAR(128) NOT NULL DEFAULT '',
  checksum VARCHAR(64) NOT NULL,
  applied_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  PRIMARY KEY (plugin_id, version)
)`,
	// '' = core menu (always visible). The default backfills existing rows
	// without a table rewrite on PG 11+.
	`ALTER TABLE sys_menu ADD COLUMN IF NOT EXISTS plugin_id VARCHAR(16) NOT NULL DEFAULT ''`,
	`CREATE INDEX IF NOT EXISTS idx_menu_plugin_id ON sys_menu (plugin_id)`,
	// Permission buttons carry the endpoint they guard. Owned by the startup
	// sync (see SyncPermissionMenus), never by the admin UI.
	`ALTER TABLE sys_menu ADD COLUMN IF NOT EXISTS method VARCHAR(16) NOT NULL DEFAULT ''`,
	`ALTER TABLE sys_menu ADD COLUMN IF NOT EXISTS path VARCHAR(255) NOT NULL DEFAULT ''`,
	`ALTER TABLE sys_menu ADD COLUMN IF NOT EXISTS is_sensitive SMALLINT NOT NULL DEFAULT 0`,
	// 1 = the row belongs to the route-table sync. Existing rows start at 0 and
	// are flipped by the first sync, which is harmless: the sync only refuses to
	// retire rows it owns, and everything it owns is re-declared on every boot.
	`ALTER TABLE sys_menu ADD COLUMN IF NOT EXISTS managed SMALLINT NOT NULL DEFAULT 0`,
	// Dead since permission codes moved to sys_menu.code: the column was never
	// populated by the seed and never read by any query.
	`ALTER TABLE sys_menu DROP COLUMN IF EXISTS permission`,
	// Business keys are unique among LIVE rows only (see schema/tables.sql).
	// A database created before this change still carries a table constraint
	// that keeps a deleted row's code occupied, which made "delete a record,
	// then create it again with the same code" fail with a duplicate-key
	// error. Dropping the constraint and installing the partial index fixes
	// that in place: the tombstones survive, the codes they hold do not block
	// new records. Both halves are no-ops on a database built from the current
	// migrations, so this block can stay forever.
	//
	// Department, position, role, dictionary and job codes are unique per
	// tenant. Username and menu code stay global. tenant_id has to exist
	// before the index is rebuilt; a database created before the multi-tenant
	// migration does not have the column yet.
	`ALTER TABLE sys_department ADD COLUMN IF NOT EXISTS tenant_id BIGINT NOT NULL DEFAULT 1`,
	`ALTER TABLE sys_position ADD COLUMN IF NOT EXISTS tenant_id BIGINT NOT NULL DEFAULT 1`,
	`ALTER TABLE sys_role ADD COLUMN IF NOT EXISTS tenant_id BIGINT NOT NULL DEFAULT 1`,
	`ALTER TABLE sys_dict_type ADD COLUMN IF NOT EXISTS tenant_id BIGINT NOT NULL DEFAULT 1`,
	`ALTER TABLE sys_dict_item ADD COLUMN IF NOT EXISTS tenant_id BIGINT NOT NULL DEFAULT 1`,
	`ALTER TABLE sys_scheduled_job ADD COLUMN IF NOT EXISTS tenant_id BIGINT NOT NULL DEFAULT 1`,
	`ALTER TABLE sys_department DROP CONSTRAINT IF EXISTS uk_department_code`,
	`DROP INDEX IF EXISTS uk_department_code`,
	`CREATE UNIQUE INDEX IF NOT EXISTS uk_department_code ON sys_department (tenant_id, code) WHERE deleted_at IS NULL`,
	`ALTER TABLE sys_position DROP CONSTRAINT IF EXISTS uk_position_code`,
	`DROP INDEX IF EXISTS uk_position_code`,
	`CREATE UNIQUE INDEX IF NOT EXISTS uk_position_code ON sys_position (tenant_id, code) WHERE deleted_at IS NULL`,
	`ALTER TABLE sys_user DROP CONSTRAINT IF EXISTS uk_user_username`,
	`CREATE UNIQUE INDEX IF NOT EXISTS uk_user_username ON sys_user (username) WHERE deleted_at IS NULL`,
	`ALTER TABLE sys_role DROP CONSTRAINT IF EXISTS uk_role_code`,
	`DROP INDEX IF EXISTS uk_role_code`,
	`CREATE UNIQUE INDEX IF NOT EXISTS uk_role_code ON sys_role (tenant_id, code) WHERE deleted_at IS NULL`,
	`ALTER TABLE sys_menu DROP CONSTRAINT IF EXISTS uk_menu_code`,
	`CREATE UNIQUE INDEX IF NOT EXISTS uk_menu_code ON sys_menu (code) WHERE deleted_at IS NULL`,
	`ALTER TABLE sys_dict_type DROP CONSTRAINT IF EXISTS uk_dict_type_code`,
	`DROP INDEX IF EXISTS uk_dict_type_code`,
	`CREATE UNIQUE INDEX IF NOT EXISTS uk_dict_type_code ON sys_dict_type (tenant_id, code) WHERE deleted_at IS NULL`,
	`ALTER TABLE sys_dict_item DROP CONSTRAINT IF EXISTS uk_dict_item_value`,
	`DROP INDEX IF EXISTS uk_dict_item_value`,
	`CREATE UNIQUE INDEX IF NOT EXISTS uk_dict_item_value ON sys_dict_item (tenant_id, dict_type_id, value) WHERE deleted_at IS NULL`,
	`ALTER TABLE sys_scheduled_job DROP CONSTRAINT IF EXISTS uk_scheduled_job_code`,
	`DROP INDEX IF EXISTS uk_scheduled_job_code`,
	`CREATE UNIQUE INDEX IF NOT EXISTS uk_scheduled_job_code ON sys_scheduled_job (tenant_id, code) WHERE deleted_at IS NULL`,
	// Global datetime display pattern used by the admin UI.
	`INSERT INTO sys_config
  (config_group, config_key, name, value_type, config_value, default_value, description)
SELECT
  'basic', 'system.dateTimeFormat', 'config.label.system_dateTimeFormat', 'string',
  'YYYY-MM-DD HH:mm:ss', 'YYYY-MM-DD HH:mm:ss', 'config.desc.system_dateTimeFormat'
WHERE NOT EXISTS (
  SELECT 1 FROM sys_config WHERE config_key = 'system.dateTimeFormat'
)`,
	// Scheduled notice delivery: push when publish_at becomes due.
	// Historical already-visible rows are marked notified by the dispatcher
	// startup backfill so upgrading does not re-blast every published notice.
	`ALTER TABLE sys_notice ADD COLUMN IF NOT EXISTS notified_at TIMESTAMPTZ NULL`,
	`CREATE INDEX IF NOT EXISTS idx_notice_due_notify
  ON sys_notice (publish_at)
  WHERE deleted_at IS NULL AND status = 'published' AND notified_at IS NULL`,
	`INSERT INTO sys_scheduled_job
  (name, code, handler, cron_expression, payload, timeout_seconds, max_retries, status, next_run_at)
SELECT
  '推送到期通知公告', 'notice.notify_due', 'notice.notify_due',
  '* * * * *', '{}'::jsonb, 60, 1, 1, NOW() + INTERVAL '1 minute'
WHERE NOT EXISTS (
  SELECT 1 FROM sys_scheduled_job
  WHERE code = 'notice.notify_due' AND deleted_at IS NULL
)`,
	// Outbox relay: publish pending sys_outbox rows once a minute.
	`INSERT INTO sys_scheduled_job
  (name, code, handler, cron_expression, payload, timeout_seconds, max_retries, status, next_run_at)
SELECT
  '投递事务外盒', 'outbox.publish', 'outbox.publish',
  '* * * * *', '{}'::jsonb, 60, 1, 1, NOW() + INTERVAL '1 minute'
WHERE NOT EXISTS (
  SELECT 1 FROM sys_scheduled_job
  WHERE code = 'outbox.publish' AND deleted_at IS NULL
)`,
	// Hourly: trial/active past their end move to overdue, then overdue past
	// grace is cancelled and the tenant is suspended.
	`INSERT INTO sys_scheduled_job
  (name, code, handler, cron_expression, payload, timeout_seconds, max_retries, status, next_run_at)
SELECT
  '对账订阅状态', 'billing.reconcile_subscriptions', 'billing.reconcile_subscriptions',
  '0 * * * *', '{}'::jsonb, 120, 1, 1, NOW() + INTERVAL '1 hour'
WHERE NOT EXISTS (
  SELECT 1 FROM sys_scheduled_job
  WHERE code = 'billing.reconcile_subscriptions' AND deleted_at IS NULL
)`,
	// Daily: tenants whose purge_after has arrived are wiped. Tenant 1 is skipped.
	`INSERT INTO sys_scheduled_job
  (name, code, handler, cron_expression, payload, timeout_seconds, max_retries, status, next_run_at)
SELECT
  '清理到期租户', 'tenant.purge_due', 'tenant.purge_due',
  '0 3 * * *', '{}'::jsonb, 300, 1, 1, NOW() + INTERVAL '1 day'
WHERE NOT EXISTS (
  SELECT 1 FROM sys_scheduled_job
  WHERE code = 'tenant.purge_due' AND deleted_at IS NULL
)`,
	// The translation page reads every preset text through these two views.
	// They live in migrations/001_system_init.sql, which the Postgres image only
	// runs against an empty data directory — a database initialised before the
	// views were added never gets them, and the page answers 500 with
	// `relation "v_preset_entity" does not exist`. Recreating them on every boot
	// is what makes the page work on an already-initialised database.
	//
	// Keep these definitions identical to schema/tables.sql: sqlc type-checks the
	// queries against that file, so a drift here only shows up at runtime.
	`CREATE OR REPLACE VIEW v_preset_entity AS
  SELECT 'menu.' || code AS entity_key, name AS source, updated_at FROM sys_menu
    WHERE deleted_at IS NULL AND type IN ('page', 'directory')
  UNION ALL
  SELECT 'perm.' || code, name, updated_at FROM sys_menu
    WHERE deleted_at IS NULL AND type = 'button'
  UNION ALL
  SELECT 'role.' || code, name, updated_at FROM sys_role WHERE deleted_at IS NULL
  UNION ALL
  SELECT 'role.' || code || '.remark', remark, updated_at FROM sys_role
    WHERE deleted_at IS NULL AND remark <> ''
  UNION ALL
  SELECT 'dept.' || code, name, updated_at FROM sys_department WHERE deleted_at IS NULL
  UNION ALL
  SELECT 'post.' || code, name, updated_at FROM sys_position WHERE deleted_at IS NULL
  UNION ALL
  SELECT 'post.' || code || '.remark', remark, updated_at FROM sys_position
    WHERE deleted_at IS NULL AND remark <> ''
  UNION ALL
  SELECT 'job.' || code, name, updated_at FROM sys_scheduled_job WHERE deleted_at IS NULL
  UNION ALL
  SELECT 'storage.' || storage_type, name, updated_at FROM sys_storage_config
    WHERE deleted_at IS NULL`,
	`CREATE OR REPLACE VIEW v_preset_source_row AS
  SELECT 'menu'::text AS namespace, id, code AS code, name AS source,
         ''::text AS remark
  FROM sys_menu WHERE deleted_at IS NULL
  UNION ALL
  SELECT 'role', id, code, name, remark FROM sys_role WHERE deleted_at IS NULL
  UNION ALL
  SELECT 'dept', id, code, name, '' FROM sys_department WHERE deleted_at IS NULL
  UNION ALL
  SELECT 'post', id, code, name, remark FROM sys_position WHERE deleted_at IS NULL
  UNION ALL
  SELECT 'job', id, code, name, '' FROM sys_scheduled_job WHERE deleted_at IS NULL
  UNION ALL
  SELECT 'storage', id, storage_type, name, '' FROM sys_storage_config
    WHERE deleted_at IS NULL`,
	// Multi-tenant schema (migrations/013_multitenant.sql).
	// FORCE ROW LEVEL SECURITY applies to the table owner, so Bootstrap must
	// SET app.rls_bypass=on before these statements run.
	`CREATE OR REPLACE FUNCTION app_current_tenant_id()
RETURNS BIGINT LANGUAGE sql STABLE AS $$
  SELECT CASE
    WHEN current_setting('app.tenant_id', true) IS NULL THEN NULL
    WHEN btrim(current_setting('app.tenant_id', true)) = '' THEN NULL
    ELSE btrim(current_setting('app.tenant_id', true))::BIGINT
  END
$$`,
	`CREATE OR REPLACE FUNCTION app_rls_bypass()
RETURNS BOOLEAN LANGUAGE sql STABLE AS $$
  SELECT lower(COALESCE(current_setting('app.rls_bypass', true), '')) IN ('on', 'true', '1')
$$`,
	`CREATE OR REPLACE FUNCTION app_fill_tenant_id()
RETURNS trigger LANGUAGE plpgsql AS $$
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
$$`,
	`CREATE OR REPLACE FUNCTION app_fill_config_tenant_id()
RETURNS trigger LANGUAGE plpgsql AS $$
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
$$`,
	`CREATE TABLE IF NOT EXISTS sys_tenant (
  id BIGSERIAL,
  code VARCHAR(64) NOT NULL,
  name VARCHAR(128) NOT NULL,
  status SMALLINT NOT NULL DEFAULT 1,
  remark VARCHAR(500) NOT NULL DEFAULT '',
  slug VARCHAR(64) NOT NULL DEFAULT '',
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  deleted_at TIMESTAMPTZ NULL,
  PRIMARY KEY (id)
)`,
	`ALTER TABLE sys_tenant ADD COLUMN IF NOT EXISTS remark VARCHAR(500) NOT NULL DEFAULT ''`,
	`ALTER TABLE sys_tenant ADD COLUMN IF NOT EXISTS purge_after TIMESTAMPTZ NULL`,
	`ALTER TABLE sys_tenant ADD COLUMN IF NOT EXISTS purge_status VARCHAR(32) NOT NULL DEFAULT ''`,
	`ALTER TABLE sys_tenant ADD COLUMN IF NOT EXISTS slug VARCHAR(64) NOT NULL DEFAULT ''`,
	`CREATE TABLE IF NOT EXISTS sys_tenant_member (
  tenant_id BIGINT NOT NULL,
  user_id BIGINT NOT NULL,
  department_id BIGINT NULL,
  is_tenant_admin SMALLINT NOT NULL DEFAULT 0,
  is_platform_operator BOOLEAN NOT NULL DEFAULT FALSE,
  status SMALLINT NOT NULL DEFAULT 1,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  PRIMARY KEY (tenant_id, user_id)
)`,
	`ALTER TABLE sys_tenant_member ADD COLUMN IF NOT EXISTS department_id BIGINT NULL`,
	`ALTER TABLE sys_tenant_member ADD COLUMN IF NOT EXISTS is_tenant_admin SMALLINT NOT NULL DEFAULT 0`,
	`ALTER TABLE sys_tenant_member ADD COLUMN IF NOT EXISTS is_platform_operator BOOLEAN NOT NULL DEFAULT FALSE`,
	`ALTER TABLE sys_tenant_member ADD COLUMN IF NOT EXISTS status SMALLINT NOT NULL DEFAULT 1`,
	`ALTER TABLE sys_tenant_member ADD COLUMN IF NOT EXISTS created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()`,
	`ALTER TABLE sys_tenant_member ADD COLUMN IF NOT EXISTS updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()`,
	`CREATE TABLE IF NOT EXISTS sys_plan (
  id BIGSERIAL,
  code VARCHAR(64) NOT NULL,
  name VARCHAR(128) NOT NULL,
  description VARCHAR(500) NOT NULL DEFAULT '',
  status SMALLINT NOT NULL DEFAULT 1,
  sort INT NOT NULL DEFAULT 0,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  PRIMARY KEY (id)
)`,
	`CREATE TABLE IF NOT EXISTS sys_plan_entitlement (
  id BIGSERIAL,
  plan_id BIGINT NOT NULL,
  entitlement_key VARCHAR(128) NOT NULL,
  entitlement_value TEXT NOT NULL DEFAULT '',
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  PRIMARY KEY (id)
)`,
	`CREATE TABLE IF NOT EXISTS sys_tenant_subscription (
  id BIGSERIAL,
  tenant_id BIGINT NOT NULL,
  plan_id BIGINT NOT NULL,
  status VARCHAR(32) NOT NULL DEFAULT 'active',
  started_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  ended_at TIMESTAMPTZ NULL,
  trial_ends_at TIMESTAMPTZ NULL,
  grace_ends_at TIMESTAMPTZ NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  PRIMARY KEY (id)
)`,
	`ALTER TABLE sys_tenant_subscription ADD COLUMN IF NOT EXISTS trial_ends_at TIMESTAMPTZ NULL`,
	`ALTER TABLE sys_tenant_subscription ADD COLUMN IF NOT EXISTS grace_ends_at TIMESTAMPTZ NULL`,
	`CREATE TABLE IF NOT EXISTS sys_usage_counter (
  tenant_id BIGINT NOT NULL,
  quota_key VARCHAR(128) NOT NULL,
  period_key VARCHAR(32) NOT NULL,
  used_value BIGINT NOT NULL DEFAULT 0,
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  PRIMARY KEY (tenant_id, quota_key, period_key)
)`,
	`CREATE TABLE IF NOT EXISTS sys_tenant_entitlement (
  tenant_id BIGINT NOT NULL,
  feature_key VARCHAR(128) NOT NULL,
  enabled BOOLEAN NOT NULL DEFAULT TRUE,
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  PRIMARY KEY (tenant_id, feature_key)
)`,
	`CREATE TABLE IF NOT EXISTS sys_tenant_quota (
  tenant_id BIGINT NOT NULL,
  quota_key VARCHAR(128) NOT NULL,
  limit_value BIGINT NOT NULL,
  used_value BIGINT NOT NULL DEFAULT 0,
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  PRIMARY KEY (tenant_id, quota_key)
)`,
	`CREATE TABLE IF NOT EXISTS sys_outbox (
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
)`,
	`ALTER TABLE sys_outbox ADD COLUMN IF NOT EXISTS attempts INT NOT NULL DEFAULT 0`,
	`ALTER TABLE sys_outbox ADD COLUMN IF NOT EXISTS last_error VARCHAR(1000) NOT NULL DEFAULT ''`,
	`ALTER TABLE sys_outbox ADD COLUMN IF NOT EXISTS available_at TIMESTAMPTZ NOT NULL DEFAULT NOW()`,
	`ALTER TABLE sys_outbox ADD COLUMN IF NOT EXISTS sent_at TIMESTAMPTZ NULL`,
	`ALTER TABLE sys_outbox ALTER COLUMN tenant_id SET DEFAULT 1`,
	`CREATE TABLE IF NOT EXISTS sys_tenant_impersonation (
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
)`,
	// 013_multitenant.sql dropped this column, but 017_refresh_token_tenant.sql
	// restored it because a refresh session must remember its tenant. Keep the
	// repair idempotent: add-if-missing, never drop (dropping here undid 017 on
	// every boot and broke login with "column tenant_id does not exist").
	`ALTER TABLE sys_refresh_token ADD COLUMN IF NOT EXISTS tenant_id BIGINT NULL`,
	`ALTER TABLE sys_user_role ADD COLUMN IF NOT EXISTS tenant_id BIGINT NOT NULL DEFAULT 1`,
	`ALTER TABLE sys_user_position ADD COLUMN IF NOT EXISTS tenant_id BIGINT NOT NULL DEFAULT 1`,
	`ALTER TABLE sys_role_department ADD COLUMN IF NOT EXISTS tenant_id BIGINT NOT NULL DEFAULT 1`,
	`ALTER TABLE sys_role_menu ADD COLUMN IF NOT EXISTS tenant_id BIGINT NOT NULL DEFAULT 1`,
	`ALTER TABLE sys_file ADD COLUMN IF NOT EXISTS tenant_id BIGINT NOT NULL DEFAULT 1`,
	`ALTER TABLE sys_file_reference ADD COLUMN IF NOT EXISTS tenant_id BIGINT NOT NULL DEFAULT 1`,
	`ALTER TABLE sys_notice ADD COLUMN IF NOT EXISTS tenant_id BIGINT NOT NULL DEFAULT 1`,
	`ALTER TABLE sys_notice_read ADD COLUMN IF NOT EXISTS tenant_id BIGINT NOT NULL DEFAULT 1`,
	`ALTER TABLE sys_notification ADD COLUMN IF NOT EXISTS tenant_id BIGINT NOT NULL DEFAULT 1`,
	`ALTER TABLE sys_user_notify_pref ADD COLUMN IF NOT EXISTS tenant_id BIGINT NOT NULL DEFAULT 1`,
	`ALTER TABLE sys_api_token ADD COLUMN IF NOT EXISTS tenant_id BIGINT NOT NULL DEFAULT 1`,
	`ALTER TABLE sys_api_token ALTER COLUMN tenant_id SET DEFAULT 1`,
	`ALTER TABLE sys_operation_log ADD COLUMN IF NOT EXISTS tenant_id BIGINT NOT NULL DEFAULT 1`,
	`ALTER TABLE sys_error_log ADD COLUMN IF NOT EXISTS tenant_id BIGINT NOT NULL DEFAULT 1`,
	`ALTER TABLE sys_webhook ADD COLUMN IF NOT EXISTS tenant_id BIGINT NOT NULL DEFAULT 1`,
	`ALTER TABLE sys_job_log ADD COLUMN IF NOT EXISTS tenant_id BIGINT NOT NULL DEFAULT 1`,
	`ALTER TABLE sys_config ADD COLUMN IF NOT EXISTS tenant_id BIGINT NOT NULL DEFAULT 0`,
	`ALTER TABLE sys_role ALTER COLUMN tenant_id SET DEFAULT 1`,
	`DO $$
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
$$`,
	`ALTER TABLE sys_config DROP CONSTRAINT IF EXISTS uk_config_key`,
	`DROP INDEX IF EXISTS uk_config_key`,
	`ALTER TABLE sys_config ADD CONSTRAINT uk_config_key UNIQUE (tenant_id, config_key)`,
	`ALTER TABLE sys_tenant DROP CONSTRAINT IF EXISTS uk_tenant_code`,
	`DROP INDEX IF EXISTS uk_tenant_code`,
	`CREATE UNIQUE INDEX IF NOT EXISTS uk_tenant_code ON sys_tenant (code) WHERE deleted_at IS NULL`,
	`CREATE UNIQUE INDEX IF NOT EXISTS uk_plan_code ON sys_plan (code)`,
	`CREATE UNIQUE INDEX IF NOT EXISTS uk_plan_entitlement ON sys_plan_entitlement (plan_id, entitlement_key)`,
	`CREATE UNIQUE INDEX IF NOT EXISTS uk_tenant_subscription_active ON sys_tenant_subscription (tenant_id) WHERE status = 'active'`,
	`CREATE INDEX IF NOT EXISTS idx_tenant_member_user ON sys_tenant_member (user_id)`,
	`CREATE INDEX IF NOT EXISTS idx_outbox_pending ON sys_outbox (status, id)`,
	`CREATE INDEX IF NOT EXISTS idx_tenant_impersonation_open ON sys_tenant_impersonation (tenant_id, actor_user_id) WHERE ended_at IS NULL`,
	`CREATE TABLE IF NOT EXISTS sys_invoice (
  id BIGSERIAL,
  tenant_id BIGINT NOT NULL,
  subscription_id BIGINT NULL,
  kind VARCHAR(32) NOT NULL,
  status VARCHAR(32) NOT NULL DEFAULT 'draft',
  amount_cents BIGINT NOT NULL DEFAULT 0,
  currency CHAR(3) NOT NULL DEFAULT 'CNY',
  period_start TIMESTAMPTZ NULL,
  period_end TIMESTAMPTZ NULL,
  memo VARCHAR(500) NOT NULL DEFAULT '',
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  PRIMARY KEY (id)
)`,
	`CREATE INDEX IF NOT EXISTS idx_invoice_tenant_id ON sys_invoice (tenant_id, id DESC)`,
	`INSERT INTO sys_plan (id, code, name, description, status, sort)
SELECT 1, 'free', '免费版', '平台内置免费套餐', 1, 0
WHERE NOT EXISTS (SELECT 1 FROM sys_plan WHERE id = 1 OR code = 'free')`,
	`INSERT INTO sys_plan_entitlement (plan_id, entitlement_key, entitlement_value)
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
  )`,
	`INSERT INTO sys_plan_entitlement (plan_id, entitlement_key, entitlement_value)
SELECT p.id, 'quota.seats', '100'
FROM sys_plan p
WHERE p.code = 'free'
  AND NOT EXISTS (
    SELECT 1 FROM sys_plan_entitlement e
    WHERE e.plan_id = p.id AND e.entitlement_key = 'quota.seats'
  )`,
	`INSERT INTO sys_tenant (id, code, name, status, slug)
SELECT 1, 'default', '默认租户', 1, 'default'
WHERE NOT EXISTS (SELECT 1 FROM sys_tenant WHERE id = 1 OR code = 'default')`,
	// Same backfill as migrations/015_tenant_domains.sql. Runs after the
	// default tenant exists so a fresh database gets slug 'default' too.
	`DO $$
DECLARE
  rec record;
  candidate text;
BEGIN
  UPDATE sys_tenant
  SET slug = 'default', updated_at = NOW()
  WHERE id = 1 AND slug = '';

  FOR rec IN
    SELECT id, code FROM sys_tenant
    WHERE id <> 1 AND slug = '' AND deleted_at IS NULL
    ORDER BY id
  LOOP
    candidate := left(regexp_replace(lower(rec.code), '[^a-z0-9-]', '', 'g'), 63);
    IF candidate !~ '^[a-z][a-z0-9-]{1,62}$'
       OR EXISTS (
         SELECT 1 FROM sys_tenant
         WHERE deleted_at IS NULL AND slug = candidate AND id <> rec.id
       )
    THEN
      candidate := 't' || rec.id::text;
    END IF;
    IF EXISTS (
      SELECT 1 FROM sys_tenant
      WHERE deleted_at IS NULL AND slug = candidate AND id <> rec.id
    ) THEN
      candidate := 't' || rec.id::text || 'x';
    END IF;
    UPDATE sys_tenant
    SET slug = candidate, updated_at = NOW()
    WHERE id = rec.id AND slug = '';
  END LOOP;
END
$$`,
	`ALTER TABLE sys_tenant DROP CONSTRAINT IF EXISTS uk_tenant_slug`,
	`DROP INDEX IF EXISTS uk_tenant_slug`,
	`CREATE UNIQUE INDEX IF NOT EXISTS uk_tenant_slug ON sys_tenant (slug) WHERE deleted_at IS NULL AND slug <> ''`,
	`CREATE TABLE IF NOT EXISTS sys_tenant_domain (
  id BIGSERIAL PRIMARY KEY,
  tenant_id BIGINT NOT NULL,
  host VARCHAR(255) NOT NULL,
  verified_at TIMESTAMPTZ NULL,
  is_primary SMALLINT NOT NULL DEFAULT 0,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
)`,
	`CREATE UNIQUE INDEX IF NOT EXISTS uk_tenant_domain_host ON sys_tenant_domain (host)`,
	`CREATE INDEX IF NOT EXISTS idx_tenant_domain_tenant ON sys_tenant_domain (tenant_id)`,
	`INSERT INTO sys_tenant_subscription (tenant_id, plan_id, status)
SELECT t.id, p.id, 'active'
FROM sys_tenant t
CROSS JOIN sys_plan p
WHERE t.id = 1 AND p.code = 'free'
  AND NOT EXISTS (
    SELECT 1 FROM sys_tenant_subscription s
    WHERE s.tenant_id = t.id AND s.status = 'active'
  )`,
	`INSERT INTO sys_tenant_member (
  tenant_id, user_id, department_id, is_tenant_admin, is_platform_operator
)
SELECT 1, u.id, u.department_id, u.is_super_admin, (u.is_super_admin <> 0)
FROM sys_user u
ON CONFLICT (tenant_id, user_id) DO NOTHING`,
	// ---------------------------------------------------------------------
	// 019_tenant_lifecycle, repeated here because migrations only run against
	// an empty data directory: an existing deployment reaches its schema
	// through this list or not at all.
	// ---------------------------------------------------------------------
	// Platform staff are people, not a membership flag. No tenant_id and no
	// RLS: the directory is global, like sys_tenant_domain.
	`CREATE TABLE IF NOT EXISTS sys_platform_operator (
  user_id BIGINT PRIMARY KEY,
  granted_by BIGINT NOT NULL DEFAULT 0,
  note VARCHAR(255) NOT NULL DEFAULT '',
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
)`,
	`INSERT INTO sys_platform_operator (user_id, note)
SELECT m.user_id, 'migrated from sys_tenant_member.is_platform_operator'
FROM sys_tenant_member m
WHERE m.is_platform_operator
ON CONFLICT (user_id) DO NOTHING`,
	`INSERT INTO sys_platform_operator (user_id, note)
SELECT u.id, 'migrated from sys_user.is_super_admin'
FROM sys_user u
WHERE u.is_super_admin <> 0 AND u.deleted_at IS NULL
ON CONFLICT (user_id) DO NOTHING`,
	`CREATE TABLE IF NOT EXISTS sys_tenant_invite (
  id BIGSERIAL PRIMARY KEY,
  tenant_id BIGINT NOT NULL,
  email VARCHAR(255) NOT NULL DEFAULT '',
  role_ids BIGINT[] NOT NULL DEFAULT '{}'::bigint[],
  department_id BIGINT NULL,
  is_tenant_admin SMALLINT NOT NULL DEFAULT 0,
  token_hash CHAR(64) NOT NULL,
  status VARCHAR(16) NOT NULL DEFAULT 'pending',
  invited_by BIGINT NOT NULL,
  accepted_by BIGINT NULL,
  expires_at TIMESTAMPTZ NOT NULL,
  accepted_at TIMESTAMPTZ NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
)`,
	`CREATE UNIQUE INDEX IF NOT EXISTS uk_tenant_invite_token ON sys_tenant_invite (token_hash)`,
	`CREATE INDEX IF NOT EXISTS idx_tenant_invite_tenant ON sys_tenant_invite (tenant_id, id DESC)`,
	`CREATE UNIQUE INDEX IF NOT EXISTS uk_tenant_invite_pending_email
  ON sys_tenant_invite (tenant_id, lower(email))
  WHERE status = 'pending' AND email <> ''`,
	// The FK needs a unique key on the parent pair. Creating it here also
	// repairs a database that predates 016 on the upgrade path.
	`CREATE UNIQUE INDEX IF NOT EXISTS uk_department_tenant_id ON sys_department (tenant_id, id)`,
	`DO $$
BEGIN
  ALTER TABLE sys_tenant_invite
    ADD CONSTRAINT fk_tenant_invite_department
    FOREIGN KEY (tenant_id, department_id) REFERENCES sys_department (tenant_id, id)
    DEFERRABLE INITIALLY DEFERRED;
EXCEPTION
  WHEN duplicate_object THEN NULL;
END
$$`,
	`ALTER TABLE sys_plan ADD COLUMN IF NOT EXISTS version INT NOT NULL DEFAULT 1`,
	`CREATE TABLE IF NOT EXISTS sys_plan_version (
  id BIGSERIAL PRIMARY KEY,
  plan_id BIGINT NOT NULL,
  version INT NOT NULL,
  snapshot JSONB NOT NULL DEFAULT '{}'::jsonb,
  created_by BIGINT NOT NULL DEFAULT 0,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
)`,
	`CREATE UNIQUE INDEX IF NOT EXISTS uk_plan_version ON sys_plan_version (plan_id, version)`,
	`CREATE INDEX IF NOT EXISTS idx_plan_version_plan ON sys_plan_version (plan_id, version DESC)`,
	`ALTER TABLE sys_tenant_subscription ADD COLUMN IF NOT EXISTS plan_version_id BIGINT NULL`,
	`ALTER TABLE sys_tenant_subscription ADD COLUMN IF NOT EXISTS plan_snapshot JSONB NOT NULL DEFAULT '{}'::jsonb`,
	// Pin existing plans and subscriptions to a snapshot so a later catalogue
	// edit cannot retroactively change what a subscriber bought.
	`INSERT INTO sys_plan_version (plan_id, version, snapshot)
SELECT p.id,
       p.version,
       jsonb_build_object(
         'planId', p.id,
         'code', p.code,
         'name', p.name,
         'version', p.version,
         'entitlements', COALESCE((
           SELECT jsonb_object_agg(e.entitlement_key, e.entitlement_value)
           FROM sys_plan_entitlement e
           WHERE e.plan_id = p.id
         ), '{}'::jsonb)
       )
FROM sys_plan p
WHERE NOT EXISTS (
  SELECT 1 FROM sys_plan_version v WHERE v.plan_id = p.id AND v.version = p.version
)`,
	`UPDATE sys_tenant_subscription s
SET plan_version_id = v.id,
    plan_snapshot = v.snapshot
FROM sys_plan_version v
WHERE v.plan_id = s.plan_id
  AND s.plan_snapshot = '{}'::jsonb
  AND v.version = (SELECT MAX(v2.version) FROM sys_plan_version v2 WHERE v2.plan_id = s.plan_id)`,
	`CREATE TABLE IF NOT EXISTS sys_payment_event (
  id BIGSERIAL PRIMARY KEY,
  provider VARCHAR(64) NOT NULL,
  event_id VARCHAR(128) NOT NULL,
  event_type VARCHAR(128) NOT NULL DEFAULT '',
  tenant_id BIGINT NOT NULL DEFAULT 0,
  payload JSONB NOT NULL DEFAULT '{}'::jsonb,
  status VARCHAR(16) NOT NULL DEFAULT 'received',
  attempts INT NOT NULL DEFAULT 0,
  last_error VARCHAR(1000) NOT NULL DEFAULT '',
  received_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  processed_at TIMESTAMPTZ NULL
)`,
	`CREATE UNIQUE INDEX IF NOT EXISTS uk_payment_event ON sys_payment_event (provider, event_id)`,
	`CREATE INDEX IF NOT EXISTS idx_payment_event_pending ON sys_payment_event (status, id)`,
	// No app_fill_tenant_id trigger here on purpose: an event may arrive before
	// its tenant is resolvable and has to stay tenant_id = 0.
	`ALTER TABLE sys_payment_event ENABLE ROW LEVEL SECURITY`,
	`ALTER TABLE sys_payment_event FORCE ROW LEVEL SECURITY`,
	`DROP POLICY IF EXISTS tenant_isolation ON sys_payment_event`,
	`CREATE POLICY tenant_isolation ON sys_payment_event USING (app_rls_bypass() OR tenant_id = app_current_tenant_id()) WITH CHECK (app_rls_bypass() OR tenant_id = app_current_tenant_id())`,
	`CREATE INDEX IF NOT EXISTS idx_usage_counter_tenant ON sys_usage_counter (tenant_id, quota_key, period_key)`,
	// The seat quota is the one limit that predates usage metering, so its
	// used_value can disagree with the real member count. Align it once.
	`UPDATE sys_tenant_quota q
SET used_value = m.total, updated_at = NOW()
FROM (
  SELECT tenant_id, COUNT(*) AS total
  FROM sys_tenant_member
  WHERE status = 1
  GROUP BY tenant_id
) m
WHERE q.quota_key = 'seats'
  AND q.tenant_id = m.tenant_id
  AND q.used_value <> m.total`,
	`DO $$
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
    'sys_tenant_entitlement', 'sys_tenant_quota', 'sys_outbox', 'sys_tenant_impersonation',
    'sys_invoice', 'sys_tenant_invite'
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
$$`,
	`DROP TRIGGER IF EXISTS trg_fill_tenant_id ON sys_config`,
	`DROP TRIGGER IF EXISTS trg_fill_config_tenant_id ON sys_config`,
	`CREATE TRIGGER trg_fill_config_tenant_id BEFORE INSERT ON sys_config FOR EACH ROW EXECUTE FUNCTION app_fill_config_tenant_id()`,
	`ALTER TABLE sys_config ENABLE ROW LEVEL SECURITY`,
	`ALTER TABLE sys_config FORCE ROW LEVEL SECURITY`,
	`DROP POLICY IF EXISTS tenant_isolation ON sys_config`,
	`CREATE POLICY tenant_isolation ON sys_config USING (app_rls_bypass() OR tenant_id = 0 OR tenant_id = app_current_tenant_id()) WITH CHECK (app_rls_bypass() OR tenant_id = 0 OR tenant_id = app_current_tenant_id())`,
	`ALTER TABLE sys_tenant ENABLE ROW LEVEL SECURITY`,
	`ALTER TABLE sys_tenant FORCE ROW LEVEL SECURITY`,
	`DROP POLICY IF EXISTS tenant_isolation ON sys_tenant`,
	`CREATE POLICY tenant_isolation ON sys_tenant USING (app_rls_bypass() OR id = app_current_tenant_id()) WITH CHECK (app_rls_bypass() OR id = app_current_tenant_id())`,
	// Platform console. Super administrators see every visible menu
	// (ListAllVisibleMenus). These rows are not granted in sys_role_menu, so
	// other roles do not see them.
	`INSERT INTO sys_menu
  (parent_id, type, name, code, route_name, route_path, component, redirect, icon, sort, status, visible, keep_alive, affix_tab, plugin_id)
SELECT 0, 'directory', '平台管理', 'platform', 'Platform', '/platform', '', '/platform/tenants', 'lucide:building-2', 70, 1, 1, 0, 0, ''
WHERE NOT EXISTS (
  SELECT 1 FROM sys_menu WHERE code = 'platform' AND deleted_at IS NULL
)`,
	`INSERT INTO sys_menu
  (parent_id, type, name, code, route_name, route_path, component, redirect, icon, sort, status, visible, keep_alive, affix_tab, plugin_id)
SELECT p.id, 'page', '租户管理', 'platform_tenants', 'PlatformTenants', '/platform/tenants', '/platform/tenants/index', '', 'lucide:building', 71, 1, 1, 1, 0, ''
FROM sys_menu p
WHERE p.code = 'platform' AND p.deleted_at IS NULL
  AND NOT EXISTS (
    SELECT 1 FROM sys_menu WHERE code = 'platform_tenants' AND deleted_at IS NULL
  )`,
	`INSERT INTO sys_menu
  (parent_id, type, name, code, route_name, route_path, component, redirect, icon, sort, status, visible, keep_alive, affix_tab, plugin_id)
SELECT p.id, 'page', '套餐管理', 'platform_plans', 'PlatformPlans', '/platform/plans', '/platform/plans/index', '', 'lucide:package', 72, 1, 1, 1, 0, ''
FROM sys_menu p
WHERE p.code = 'platform' AND p.deleted_at IS NULL
  AND NOT EXISTS (
    SELECT 1 FROM sys_menu WHERE code = 'platform_plans' AND deleted_at IS NULL
  )`,
	`INSERT INTO sys_menu
  (parent_id, type, name, code, route_name, route_path, component, redirect, icon, sort, status, visible, keep_alive, affix_tab, plugin_id)
SELECT p.id, 'page', '账单管理', 'platform_invoices', 'PlatformInvoices', '/platform/invoices', '/platform/invoices/index', '', 'lucide:receipt', 73, 1, 1, 1, 0, ''
FROM sys_menu p
WHERE p.code = 'platform' AND p.deleted_at IS NULL
  AND NOT EXISTS (
    SELECT 1 FROM sys_menu WHERE code = 'platform_invoices' AND deleted_at IS NULL
  )`,
	// Tenant billing sits under 系统设置 (code settings). Super administrators
	// already see every menu through ListAllVisibleMenus. The role_menu row
	// grants it to role 1, the seeded administrator of the default tenant, so
	// a fresh install shows the page. Other roles see it after an administrator
	// grants the menu in the role UI. tenant_id defaults to 1.
	`INSERT INTO sys_menu
  (parent_id, type, name, code, route_name, route_path, component, redirect, icon, sort, status, visible, keep_alive, affix_tab, plugin_id)
SELECT p.id, 'page', '我的账单', 'system_billing', 'SystemBilling', '/settings/billing', '/billing/index', '', 'lucide:wallet', 25, 1, 1, 1, 0, ''
FROM sys_menu p
WHERE p.code = 'settings' AND p.deleted_at IS NULL
  AND NOT EXISTS (
    SELECT 1 FROM sys_menu WHERE code = 'system_billing' AND deleted_at IS NULL
  )`,
	`INSERT INTO sys_role_menu (role_id, menu_id)
SELECT 1, id FROM sys_menu
WHERE code = 'system_billing' AND deleted_at IS NULL
ON CONFLICT DO NOTHING`,
	// Membership management is its own page under 组织权限. It must exist as a
	// page before the permission sync runs, because system:member:* buttons
	// declare MenuCode=system_member: an unresolvable parent would abort the
	// boot with ErrMenuCodeTaken-style noise for a core route.
	`INSERT INTO sys_menu
  (parent_id, type, name, code, route_name, route_path, component, redirect, icon, sort, status, visible, keep_alive, affix_tab, plugin_id)
SELECT p.id, 'page', '成员管理', 'system_member', 'SystemMember', '/organization/members', '/system/member/index', '', 'lucide:users-round', 16, 1, 1, 1, 0, ''
FROM sys_menu p
WHERE p.code = 'organization' AND p.deleted_at IS NULL
  AND NOT EXISTS (
    SELECT 1 FROM sys_menu WHERE code = 'system_member' AND deleted_at IS NULL
  )`,
	`INSERT INTO sys_role_menu (role_id, menu_id)
SELECT 1, id FROM sys_menu
WHERE code = 'system_member' AND deleted_at IS NULL
ON CONFLICT DO NOTHING`,
	// Platform staff directory. Not granted in sys_role_menu: the platform
	// console is gated by the platform operator flag, not by a role grant.
	`INSERT INTO sys_menu
  (parent_id, type, name, code, route_name, route_path, component, redirect, icon, sort, status, visible, keep_alive, affix_tab, plugin_id)
SELECT p.id, 'page', '平台人员', 'platform_operators', 'PlatformOperators', '/platform/operators', '/platform/operators/index', '', 'lucide:user-shield', 74, 1, 1, 1, 0, ''
FROM sys_menu p
WHERE p.code = 'platform' AND p.deleted_at IS NULL
  AND NOT EXISTS (
    SELECT 1 FROM sys_menu WHERE code = 'platform_operators' AND deleted_at IS NULL
  )`,
	`INSERT INTO sys_translation (entity_key, lang, value)
SELECT v.entity_key, 'en', v.value
FROM (VALUES
  ('menu.platform', 'Platform'),
  ('menu.platform_tenants', 'Tenants'),
  ('menu.platform_plans', 'Plans'),
  ('menu.platform_invoices', 'Invoice Management'),
  ('menu.platform_operators', 'Platform Staff'),
  ('menu.system_billing', 'Billing'),
  ('menu.system_member', 'Members')
) AS v(entity_key, value)
WHERE NOT EXISTS (
  SELECT 1 FROM sys_translation t
  WHERE t.entity_key = v.entity_key AND t.lang = 'en'
)`,
}

// retiredModuleStatements delete rows belonging to product modules that were
// removed from the code base.
//
// They exist because there is no migration runner: the Postgres image only
// executes migrations/*.sql against an empty data directory, so deleting a seed
// file does nothing for databases that are already initialised. Running these
// on every boot is safe — once the rows are gone the statements match nothing.
//
// Every retired module must be listed here at removal time, and the block can be
// dropped once no deployment can still hold the old rows.
var retiredModuleStatements = []string{
	// AI 创作 (removed 2026-10-29). It was a frontend-only module: menu rows
	// only, no API endpoints and no permission codes.
	`DELETE FROM sys_role_menu WHERE menu_id IN (
  SELECT id FROM sys_menu
  WHERE code IN ('ai_creation', 'ai_image', 'ai_video', 'ai_works', 'ai_ops')
)`,
	`DELETE FROM sys_menu
  WHERE code IN ('ai_creation', 'ai_image', 'ai_video', 'ai_works', 'ai_ops')`,
	// 接口管理 (removed 2026-10-29). Permission codes now live as button nodes
	// under the page they belong to, so the standalone page and its own
	// permission codes are gone.
	`DELETE FROM sys_role_menu WHERE menu_id IN (
  SELECT id FROM sys_menu
  WHERE code = 'system_api_permission' OR code LIKE 'system:api-permission:%'
)`,
	`DELETE FROM sys_menu
  WHERE code = 'system_api_permission' OR code LIKE 'system:api-permission:%'`,
}

// seedStatements are idempotent DML. They run after bootstrap, under the same
// advisory lock as everything else.
var seedStatements = []string{
	// migrations/001_system_init.sql seeds rows with EXPLICIT ids, which leaves
	// the owning sequence at its initial value. The first row created through
	// the UI then asks for id 1 again and dies with
	// `duplicate key value violates unique constraint "sys_x_pkey"` — the
	// reason "delete a record, then create another one" appeared to fail on
	// almost every table.
	//
	// This used to be a single setval() for sys_menu only, so every other table
	// stayed broken until someone happened to trigger (and absorb) its
	// collision. Walking every serial id column instead means a table added
	// later cannot be forgotten. GREATEST keeps each sequence monotonic, so the
	// whole block is a no-op once the values line up and is safe on every boot.
	`DO $$
DECLARE
  target RECORD;
BEGIN
  FOR target IN
    SELECT c.relname AS tbl, a.attname AS col, s.relname AS seq
    FROM pg_class c
    JOIN pg_namespace n ON n.oid = c.relnamespace
    JOIN pg_depend d ON d.refobjid = c.oid AND d.deptype = 'a'
    JOIN pg_class s ON s.oid = d.objid AND s.relkind = 'S'
    JOIN pg_attribute a ON a.attrelid = c.oid AND a.attnum = d.refobjsubid
    WHERE n.nspname = current_schema() AND c.relkind = 'r'
  LOOP
    EXECUTE format(
      'SELECT setval(%L, GREATEST((SELECT COALESCE(MAX(%I), 0) FROM %I), (SELECT last_value FROM %I)))',
      target.seq, target.col, target.tbl, target.seq
    );
  END LOOP;
END
$$`,
}

// withRLSBypass runs fn on one pooled connection with app.rls_bypass=on.
//
// migrations/013_multitenant.sql turns FORCE ROW LEVEL SECURITY on. The table
// owner does not skip it, so bootstrap must SET app.rls_bypass=on. The setting
// is cleared before the connection goes back to the pool. BeginTx also copies
// ctxkey.RLSBypass into the transaction; statements that use the pool directly
// need this helper.
func (r *PluginRepository) withRLSBypass(ctx context.Context, fn func(context.Context, *sql.Conn) error) error {
	conn, err := r.db.Conn(ctx)
	if err != nil {
		return err
	}
	defer conn.Close()
	if _, err := conn.ExecContext(ctx, `SELECT set_config('app.rls_bypass', 'on', false)`); err != nil {
		return err
	}
	defer func() {
		_, _ = conn.ExecContext(context.Background(), `SELECT set_config('app.rls_bypass', 'off', false)`)
	}()
	return fn(ctx, conn)
}

// Bootstrap creates the plugin tables/columns if they are missing.
// The context is marked to bypass row-level tenant filters: schema repair
// has to see every tenant, and BeginTx copies that flag into the transaction.
func (r *PluginRepository) Bootstrap(ctx context.Context) error {
	ctx = ctxkey.WithRLSBypass(ctx, true)
	if err := r.withRLSBypass(ctx, func(ctx context.Context, conn *sql.Conn) error {
		for _, statement := range bootstrapStatements {
			if _, err := conn.ExecContext(ctx, statement); err != nil {
				return err
			}
		}
		return nil
	}); err != nil {
		return fmt.Errorf("bootstrap plugin schema: %w", err)
	}
	return r.migrateLegacyAPIPermissions(ctx)
}

// legacyAPIMigrationStatements convert the old dedicated permission tables into
// menu buttons and then drop them.
//
// The grant rows are the only data that cannot be re-derived, so they are copied
// first: sys_role_api_permission becomes sys_role_menu pointing at the matching
// button. parent_id starts at 0 and is corrected by SyncPermissionMenus, which
// runs after every plugin migration has created its pages.
var legacyAPIMigrationStatements = []string{
	`INSERT INTO sys_menu
  (parent_id, type, name, code, method, path, is_sensitive, sort, status, visible, plugin_id)
SELECT 0, 'button', p.name, p.code, p.method, p.path, p.is_sensitive, 100, 1, 0, ''
FROM sys_api_permission p
ON CONFLICT DO NOTHING`,
	`INSERT INTO sys_role_menu (role_id, menu_id)
SELECT rp.role_id, m.id
FROM sys_role_api_permission rp
INNER JOIN sys_api_permission p ON p.id = rp.permission_id
INNER JOIN sys_menu m ON m.code = p.code AND m.type = 'button'
ON CONFLICT DO NOTHING`,
	// Dependents first: the join table, then the grant table, then the catalog.
	`DROP TABLE IF EXISTS sys_menu_api_permission`,
	`DROP TABLE IF EXISTS sys_role_api_permission`,
	`DROP TABLE IF EXISTS sys_api_permission`,
}

// migrateLegacyAPIPermissions is a one-shot upgrade path from the separate
// "API permission" model to permission buttons.
//
// The guard is the presence of the legacy table, so the block is a no-op on
// every boot after the first. PostgreSQL runs DDL transactionally: a crash
// rolls back the backfill and the drops together, never leaving a database with
// the tables gone and the grants lost.
//
// The backfill copies every legacy code, including codes of modules that have
// since been deleted (接口管理 itself). Seed runs right after Bootstrap and its
// retiredModuleStatements delete exactly those rows again, so the net result is
// "grants for permissions that still exist survive, the rest disappear".
func (r *PluginRepository) migrateLegacyAPIPermissions(ctx context.Context) error {
	var legacy bool
	if err := r.db.QueryRowContext(ctx,
		`SELECT to_regclass('public.sys_api_permission') IS NOT NULL`).Scan(&legacy); err != nil {
		return fmt.Errorf("check legacy permission table: %w", err)
	}
	if !legacy {
		return nil
	}

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	for _, statement := range legacyAPIMigrationStatements {
		if _, err := tx.ExecContext(ctx, statement); err != nil {
			return fmt.Errorf("migrate legacy api permissions: %w", err)
		}
	}
	return tx.Commit()
}

// Seed installs the plugin-center permission codes and menu row, then repairs
// the menu sequence. Every statement is idempotent.
//
// Retired-module cleanup runs first so a removed module's menu rows cannot
// outlive the code that rendered them.
func (r *PluginRepository) Seed(ctx context.Context) error {
	ctx = ctxkey.WithRLSBypass(ctx, true)
	return r.withRLSBypass(ctx, func(ctx context.Context, conn *sql.Conn) error {
		for _, statement := range retiredModuleStatements {
			if _, err := conn.ExecContext(ctx, statement); err != nil {
				return fmt.Errorf("retire removed module: %w", err)
			}
		}
		q := sqlcstore.New(conn)
		if err := q.SeedPluginMenu(ctx); err != nil {
			return fmt.Errorf("seed plugin menu: %w", err)
		}
		if err := q.SeedPluginRoleMenu(ctx); err != nil {
			return fmt.Errorf("grant plugin menu to role 1: %w", err)
		}
		for _, statement := range seedStatements {
			if _, err := conn.ExecContext(ctx, statement); err != nil {
				return fmt.Errorf("repair id sequences: %w", err)
			}
		}
		return nil
	})
}

// SyncCatalog upserts one row per compiled-in plugin.
//
// A newly inserted non-builtin plugin starts disabled (ship dark): its
// migrations only run when an administrator enables it. Builtins are forced
// enabled so the platform can never be switched off. An existing row keeps the
// administrator's choice.
func (r *PluginRepository) SyncCatalog(ctx context.Context, entries []PluginCatalogEntry) error {
	for _, entry := range entries {
		if err := r.queries.UpsertPlugin(ctx, sqlcstore.UpsertPluginParams{
			PluginID:    entry.ID,
			Name:        entry.Name,
			Version:     entry.Version,
			Description: entry.Description,
			IsBuiltin:   entry.Builtin,
		}); err != nil {
			return fmt.Errorf("sync plugin %q: %w", entry.ID, err)
		}
	}
	return nil
}

// ListPlugins returns every persisted plugin row ordered by ID.
func (r *PluginRepository) ListPlugins(ctx context.Context) ([]PluginRecord, error) {
	rows, err := r.queries.ListPlugins(ctx)
	if err != nil {
		return nil, err
	}
	records := make([]PluginRecord, 0, len(rows))
	for _, row := range rows {
		records = append(records, PluginRecord{
			ID:          row.PluginID,
			Name:        row.Name,
			Version:     row.Version,
			Description: row.Description,
			Builtin:     row.IsBuiltin,
			Enabled:     row.Enabled,
			LastError:   row.LastError,
			EnabledAt:   row.EnabledAt.Time,
			InstalledAt: row.InstalledAt.Time,
			UpdatedAt:   row.UpdatedAt,
		})
	}
	return records, nil
}

// MarkPluginInstalled stamps installed_at without flipping enabled.
func (r *PluginRepository) MarkPluginInstalled(ctx context.Context, pluginID string) (bool, error) {
	affected, err := r.queries.MarkPluginInstalled(ctx, pluginID)
	if err != nil {
		return false, err
	}
	return affected > 0, nil
}

// SetPluginEnabled flips the flag. It reports false when no row was updated,
// which happens for an unknown plugin or for a builtin (the SQL excludes
// builtins as defense in depth).
func (r *PluginRepository) SetPluginEnabled(ctx context.Context, pluginID string, enabled bool) (bool, error) {
	var (
		affected int64
		err      error
	)
	if enabled {
		affected, err = r.queries.EnablePlugin(ctx, pluginID)
	} else {
		affected, err = r.queries.DisablePlugin(ctx, pluginID)
	}
	if err != nil {
		return false, err
	}
	return affected > 0, nil
}

// UninstallPlugin removes a plugin completely.
//
// Everything runs in one transaction, so a failure leaves the plugin exactly as
// it was. Order matters only for foreign keys: grants before menus, and the
// tables before the catalog row that names them.
//
// Table names are interpolated (SQL cannot bind identifiers), so they are
// validated against a strict identifier pattern first. A plugin that declares
// no tables is still uninstallable: it just has menus and migrations.
func (r *PluginRepository) UninstallPlugin(ctx context.Context, pluginID string, dropTables []string) error {
	if !pluginIDPattern.MatchString(pluginID) {
		return fmt.Errorf("uninstall plugin: invalid plugin id %q", pluginID)
	}
	for _, table := range dropTables {
		if !tableNamePattern.MatchString(table) {
			return fmt.Errorf("uninstall plugin %q: invalid table name %q", pluginID, table)
		}
	}

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	for _, table := range dropTables {
		// No CASCADE: a table still referenced by another plugin's foreign key
		// must fail loudly instead of silently dragging data along.
		if _, err := tx.ExecContext(ctx, `DROP TABLE IF EXISTS "`+table+`"`); err != nil {
			return fmt.Errorf("uninstall plugin %q: drop table %s: %w", pluginID, table, err)
		}
	}
	statements := []struct {
		sql  string
		args []any
	}{
		{`DELETE FROM sys_plugin_migration WHERE plugin_id = $1`, []any{pluginID}},
		{`DELETE FROM sys_role_menu WHERE menu_id IN (SELECT id FROM sys_menu WHERE plugin_id = $1)`, []any{pluginID}},
		{`DELETE FROM sys_menu WHERE plugin_id = $1`, []any{pluginID}},
		{`DELETE FROM sys_plugin WHERE plugin_id = $1`, []any{pluginID}},
	}
	for _, statement := range statements {
		if _, err := tx.ExecContext(ctx, statement.sql, statement.args...); err != nil {
			return fmt.Errorf("uninstall plugin %q: %w", pluginID, err)
		}
	}
	return tx.Commit()
}

// SetPluginError records why a plugin is not running.
func (r *PluginRepository) SetPluginError(ctx context.Context, pluginID, message string) error {
	if len(message) > maxLastErrorLen {
		message = message[:maxLastErrorLen]
	}
	return r.queries.SetPluginError(ctx, sqlcstore.SetPluginErrorParams{
		PluginID:  pluginID,
		LastError: message,
	})
}

// AppliedMigrations returns version → checksum for one plugin.
func (r *PluginRepository) AppliedMigrations(ctx context.Context, pluginID string) (map[string]string, error) {
	rows, err := r.queries.ListPluginMigrations(ctx, pluginID)
	if err != nil {
		return nil, err
	}
	applied := make(map[string]string, len(rows))
	for _, row := range rows {
		applied[row.Version] = row.Checksum
	}
	return applied, nil
}

// ApplyMigration runs one migration file and records it in the SAME
// transaction, so the presence of the bookkeeping row means "committed".
//
// The SQL is executed with no bind parameters, which makes pgx use the simple
// protocol and therefore accept multiple statements per file. Migration files
// must not contain $1-style placeholders.
func (r *PluginRepository) ApplyMigration(ctx context.Context, pluginID, version, name, checksum, script string) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	if _, err := tx.ExecContext(ctx, script); err != nil {
		return fmt.Errorf("execute migration %s/%s: %w", pluginID, version, err)
	}
	if err := r.queries.WithTx(tx.Tx).RecordPluginMigration(ctx, sqlcstore.RecordPluginMigrationParams{
		PluginID: pluginID,
		Version:  version,
		Name:     name,
		Checksum: checksum,
	}); err != nil {
		return fmt.Errorf("record migration %s/%s: %w", pluginID, version, err)
	}
	return tx.Commit()
}

// AcquireMigrationLock takes the session-level advisory lock and returns the
// release function. Session level (not transaction level) because each
// migration file commits on its own; PostgreSQL drops the lock automatically
// if the process dies, because the dedicated connection closes.
func (r *PluginRepository) AcquireMigrationLock(ctx context.Context, timeout time.Duration) (func(), error) {
	conn, err := r.db.Conn(ctx)
	if err != nil {
		return nil, fmt.Errorf("acquire migration lock connection: %w", err)
	}

	deadline := time.Now().Add(timeout)
	for {
		var acquired bool
		if err := conn.QueryRowContext(ctx, "SELECT pg_try_advisory_lock($1)", migrationLockKey).Scan(&acquired); err != nil {
			_ = conn.Close()
			return nil, fmt.Errorf("try migration lock: %w", err)
		}
		if acquired {
			return func() {
				// context.Background: the lock must be released even when the
				// request that took it has already been cancelled.
				_, _ = conn.ExecContext(context.Background(), "SELECT pg_advisory_unlock($1)", migrationLockKey)
				_ = conn.Close()
			}, nil
		}
		if time.Now().After(deadline) {
			_ = conn.Close()
			return nil, ErrMigrationBusy
		}
		select {
		case <-ctx.Done():
			_ = conn.Close()
			return nil, ctx.Err()
		case <-time.After(200 * time.Millisecond):
		}
	}
}

// CountPluginMigrations is used by diagnostics/tests.
func (r *PluginRepository) CountPluginMigrations(ctx context.Context) (int64, error) {
	return r.queries.CountPluginMigrations(ctx)
}
