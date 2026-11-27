CREATE TABLE sys_department (
  id BIGSERIAL,
  tenant_id BIGINT NOT NULL,
  parent_id BIGINT NOT NULL DEFAULT 0,
  ancestors VARCHAR(1000) NOT NULL DEFAULT '',
  code VARCHAR(64) NOT NULL,
  name VARCHAR(128) NOT NULL,
  remark VARCHAR(500) NOT NULL DEFAULT '',
  leader_user_id BIGINT NULL,
  sort INT NOT NULL DEFAULT 0,
  status SMALLINT NOT NULL DEFAULT 1,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  created_by BIGINT NOT NULL DEFAULT 0,
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_by BIGINT NOT NULL DEFAULT 0,
  deleted_at TIMESTAMPTZ NULL,
  PRIMARY KEY (id)
);

CREATE TABLE sys_position (
  id BIGSERIAL,
  tenant_id BIGINT NOT NULL,
  code VARCHAR(64) NOT NULL,
  name VARCHAR(128) NOT NULL,
  sort INT NOT NULL DEFAULT 0,
  status SMALLINT NOT NULL DEFAULT 1,
  remark VARCHAR(500) NOT NULL DEFAULT '',
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  created_by BIGINT NOT NULL DEFAULT 0,
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_by BIGINT NOT NULL DEFAULT 0,
  deleted_at TIMESTAMPTZ NULL,
  PRIMARY KEY (id)
);

CREATE TABLE sys_user (
  id BIGSERIAL,
  department_id BIGINT NULL,
  username VARCHAR(64) NOT NULL,
  password_hash VARCHAR(255) NOT NULL,
  display_name VARCHAR(128) NOT NULL,
  email VARCHAR(255) NOT NULL DEFAULT '',
  mobile VARCHAR(32) NOT NULL DEFAULT '',
  avatar_file_id BIGINT NULL,
  gender SMALLINT NOT NULL DEFAULT 0,
  status SMALLINT NOT NULL DEFAULT 1,
  is_super_admin BOOLEAN NOT NULL DEFAULT FALSE,

  must_change_password SMALLINT NOT NULL DEFAULT 1,
  password_changed_at TIMESTAMPTZ NULL,
  last_login_at TIMESTAMPTZ NULL,
  last_login_ip VARCHAR(64) NOT NULL DEFAULT '',
  failed_login_count INT NOT NULL DEFAULT 0,
  locked_until TIMESTAMPTZ NULL,
  token_version BIGINT NOT NULL DEFAULT 0,
  remark VARCHAR(500) NOT NULL DEFAULT '',
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  created_by BIGINT NOT NULL DEFAULT 0,
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_by BIGINT NOT NULL DEFAULT 0,
  deleted_at TIMESTAMPTZ NULL,
  PRIMARY KEY (id)
);

CREATE TABLE sys_user_position (
  tenant_id BIGINT NOT NULL,
  user_id BIGINT NOT NULL,
  position_id BIGINT NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  created_by BIGINT NOT NULL DEFAULT 0,
  PRIMARY KEY (user_id, position_id)
);

CREATE TABLE sys_role (
  id BIGSERIAL,
  tenant_id BIGINT NOT NULL,
  code VARCHAR(64) NOT NULL,
  name VARCHAR(128) NOT NULL,
  data_scope SMALLINT NOT NULL DEFAULT 1,
  sort INT NOT NULL DEFAULT 0,
  status SMALLINT NOT NULL DEFAULT 1,
  is_system BOOLEAN NOT NULL DEFAULT FALSE,

  remark VARCHAR(500) NOT NULL DEFAULT '',
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  created_by BIGINT NOT NULL DEFAULT 0,
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_by BIGINT NOT NULL DEFAULT 0,
  deleted_at TIMESTAMPTZ NULL,
  PRIMARY KEY (id)
);

CREATE TABLE sys_user_role (
  tenant_id BIGINT NOT NULL,
  user_id BIGINT NOT NULL,
  role_id BIGINT NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  created_by BIGINT NOT NULL DEFAULT 0,
  PRIMARY KEY (user_id, role_id)
);

CREATE TABLE sys_role_department (
  tenant_id BIGINT NOT NULL,
  role_id BIGINT NOT NULL,
  department_id BIGINT NOT NULL,
  PRIMARY KEY (role_id, department_id)
);

CREATE TABLE sys_menu (
  id BIGSERIAL,
  parent_id BIGINT NOT NULL DEFAULT 0,
  type VARCHAR(16) NOT NULL,
  name VARCHAR(128) NOT NULL,
  code VARCHAR(128) NOT NULL,
  route_name VARCHAR(128) NOT NULL DEFAULT '',
  route_path VARCHAR(255) NOT NULL DEFAULT '',
  component VARCHAR(255) NOT NULL DEFAULT '',
  redirect VARCHAR(255) NOT NULL DEFAULT '',
  icon VARCHAR(128) NOT NULL DEFAULT '',
  sort INT NOT NULL DEFAULT 0,
  status SMALLINT NOT NULL DEFAULT 1,
  -- Owning plugin. '' means a core menu, which is always visible; any other
  -- value is only visible while that plugin is enabled.
  plugin_id VARCHAR(16) NOT NULL DEFAULT '',
  -- Permission buttons only: the endpoint the code guards. The startup sync
  -- owns these two columns and keeps them aligned with the route table; they
  -- are informational (enforcement is by code), so a code guarding several
  -- routes shows the first one.
  method VARCHAR(16) NOT NULL DEFAULT '',
  path VARCHAR(255) NOT NULL DEFAULT '',
  -- 1 = the row is owned by the route-table sync (a declared permission code).
  -- Those rows may only be renamed and re-sorted, and the sync retires them
  -- when the route disappears. 0 = an ordinary row, including buttons an
  -- administrator created by hand, which the sync never touches.
  managed SMALLINT NOT NULL DEFAULT 0,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  created_by BIGINT NOT NULL DEFAULT 0,
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_by BIGINT NOT NULL DEFAULT 0,
  deleted_at TIMESTAMPTZ NULL,
  PRIMARY KEY (id)
);

CREATE TABLE sys_role_menu (
  tenant_id BIGINT NOT NULL,
  role_id BIGINT NOT NULL,
  menu_id BIGINT NOT NULL,
  PRIMARY KEY (role_id, menu_id)
);

CREATE TABLE sys_dict_type (
  id BIGSERIAL,
  tenant_id BIGINT NOT NULL,
  code VARCHAR(128) NOT NULL,
  name VARCHAR(128) NOT NULL,
  display_type VARCHAR(32) NOT NULL DEFAULT 'segmented',
  status SMALLINT NOT NULL DEFAULT 1,
  is_system BOOLEAN NOT NULL DEFAULT FALSE,

  remark VARCHAR(500) NOT NULL DEFAULT '',
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  created_by BIGINT NOT NULL DEFAULT 0,
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_by BIGINT NOT NULL DEFAULT 0,
  deleted_at TIMESTAMPTZ NULL,
  PRIMARY KEY (id)
);

CREATE TABLE sys_dict_item (
  id BIGSERIAL,
  tenant_id BIGINT NOT NULL,
  dict_type_id BIGINT NOT NULL,
  label VARCHAR(128) NOT NULL,
  value VARCHAR(255) NOT NULL,
  color VARCHAR(32) NOT NULL DEFAULT '',
  css_class VARCHAR(128) NOT NULL DEFAULT '',
  sort INT NOT NULL DEFAULT 0,
  status SMALLINT NOT NULL DEFAULT 1,
  is_default BOOLEAN NOT NULL DEFAULT FALSE,

  remark VARCHAR(500) NOT NULL DEFAULT '',
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  created_by BIGINT NOT NULL DEFAULT 0,
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_by BIGINT NOT NULL DEFAULT 0,
  deleted_at TIMESTAMPTZ NULL,
  PRIMARY KEY (id)
);

CREATE TABLE sys_config (
  id BIGSERIAL,
  -- 0 is the platform-global value. A positive id overrides it for that tenant.
  tenant_id BIGINT NOT NULL DEFAULT 0,
  config_group VARCHAR(64) NOT NULL,
  config_key VARCHAR(128) NOT NULL,
  name VARCHAR(128) NOT NULL,
  value_type VARCHAR(32) NOT NULL DEFAULT 'string',
  config_value TEXT NOT NULL,
  default_value TEXT NULL,
  is_secret BOOLEAN NOT NULL DEFAULT FALSE,

  is_readonly BOOLEAN NOT NULL DEFAULT FALSE,

  effect_mode VARCHAR(32) NOT NULL DEFAULT 'immediate',
  remark VARCHAR(500) NOT NULL DEFAULT '',
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  created_by BIGINT NOT NULL DEFAULT 0,
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_by BIGINT NOT NULL DEFAULT 0,
  PRIMARY KEY (id),
  CONSTRAINT uk_config_key UNIQUE (tenant_id, config_key)
);

CREATE TABLE sys_auth_provider (
  id BIGSERIAL,
  provider_type VARCHAR(32) NOT NULL,
  name VARCHAR(128) NOT NULL,
  icon VARCHAR(255) NOT NULL DEFAULT '',
  client_id VARCHAR(255) NOT NULL,
  client_secret_encrypted TEXT NOT NULL,
  authorization_url VARCHAR(500) NOT NULL DEFAULT '',
  token_url VARCHAR(500) NOT NULL DEFAULT '',
  user_info_url VARCHAR(500) NOT NULL DEFAULT '',
  issuer_url VARCHAR(500) NOT NULL DEFAULT '',
  callback_url VARCHAR(500) NOT NULL,
  scopes VARCHAR(1000) NOT NULL DEFAULT '',
  auto_register SMALLINT NOT NULL DEFAULT 0,
  default_role_id BIGINT NULL,
  binding_rule VARCHAR(32) NOT NULL DEFAULT 'manual',
  status SMALLINT NOT NULL DEFAULT 0,
  sort INT NOT NULL DEFAULT 0,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  created_by BIGINT NOT NULL DEFAULT 0,
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_by BIGINT NOT NULL DEFAULT 0,
  deleted_at TIMESTAMPTZ NULL,
  PRIMARY KEY (id)
);

CREATE TABLE sys_user_identity (
  id BIGSERIAL,
  user_id BIGINT NOT NULL,
  provider_id BIGINT NOT NULL,
  external_user_id VARCHAR(255) NOT NULL,
  external_username VARCHAR(255) NOT NULL DEFAULT '',
  external_email VARCHAR(255) NOT NULL DEFAULT '',
  profile_json JSONB NULL,
  bound_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  last_login_at TIMESTAMPTZ NULL,
  PRIMARY KEY (id),
  CONSTRAINT uk_user_identity_external UNIQUE (provider_id, external_user_id),
  CONSTRAINT uk_user_identity_user UNIQUE (user_id, provider_id)
);

CREATE TABLE sys_storage_config (
  id BIGSERIAL,
  name VARCHAR(128) NOT NULL,
  storage_type VARCHAR(32) NOT NULL,
  endpoint VARCHAR(500) NOT NULL DEFAULT '',
  bucket VARCHAR(255) NOT NULL DEFAULT '',
  region VARCHAR(128) NOT NULL DEFAULT '',
  access_key_encrypted TEXT NULL,
  secret_key_encrypted TEXT NULL,
  base_path VARCHAR(255) NOT NULL DEFAULT '',
  public_base_url VARCHAR(500) NOT NULL DEFAULT '',
  use_https SMALLINT NOT NULL DEFAULT 1,
  private_access SMALLINT NOT NULL DEFAULT 1,
  max_file_size BIGINT NOT NULL DEFAULT 104857600,
  allowed_extensions TEXT NULL,
  is_default BOOLEAN NOT NULL DEFAULT FALSE,

  status SMALLINT NOT NULL DEFAULT 1,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  created_by BIGINT NOT NULL DEFAULT 0,
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_by BIGINT NOT NULL DEFAULT 0,
  deleted_at TIMESTAMPTZ NULL,
  PRIMARY KEY (id)
);

CREATE TABLE sys_file (
  id BIGSERIAL,
  tenant_id BIGINT NOT NULL,
  storage_id BIGINT NOT NULL,
  original_name VARCHAR(500) NOT NULL,
  object_key VARCHAR(1000) NOT NULL,
  extension VARCHAR(32) NOT NULL DEFAULT '',
  mime_type VARCHAR(255) NOT NULL DEFAULT 'application/octet-stream',
  size BIGINT NOT NULL DEFAULT 0,
  sha256 CHAR(64) NOT NULL DEFAULT '',
  access_type VARCHAR(16) NOT NULL DEFAULT 'private',
  status VARCHAR(32) NOT NULL DEFAULT 'pending',
  uploader_id BIGINT NOT NULL,
  uploaded_at TIMESTAMPTZ NULL,
  recycle_at TIMESTAMPTZ NULL,
  purge_after TIMESTAMPTZ NULL,
  purge_error VARCHAR(1000) NOT NULL DEFAULT '',
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  created_by BIGINT NOT NULL DEFAULT 0,
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_by BIGINT NOT NULL DEFAULT 0,
  deleted_at TIMESTAMPTZ NULL,
  PRIMARY KEY (id),
  CONSTRAINT uk_file_object UNIQUE (storage_id, object_key)
);

CREATE TABLE sys_file_reference (
  id BIGSERIAL,
  tenant_id BIGINT NOT NULL,
  file_id BIGINT NOT NULL,
  business_type VARCHAR(64) NOT NULL,
  business_id VARCHAR(128) NOT NULL,
  field_name VARCHAR(64) NOT NULL DEFAULT 'attachments',
  created_by BIGINT NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  PRIMARY KEY (id),
  CONSTRAINT uk_file_reference UNIQUE (file_id, business_type, business_id, field_name)
);

CREATE TABLE sys_notice (
  id BIGSERIAL,
  tenant_id BIGINT NOT NULL,
  title VARCHAR(255) NOT NULL,
  notice_type VARCHAR(32) NOT NULL DEFAULT 'notice',
  priority VARCHAR(16) NOT NULL DEFAULT 'normal',
  content TEXT NOT NULL,
  target_type VARCHAR(32) NOT NULL DEFAULT 'all',
  target_ids JSONB NULL,
  status VARCHAR(32) NOT NULL DEFAULT 'draft',
  is_pinned BOOLEAN NOT NULL DEFAULT FALSE,

  publish_at TIMESTAMPTZ NULL,
  notified_at TIMESTAMPTZ NULL,
  published_by BIGINT NULL,
  revoked_at TIMESTAMPTZ NULL,
  revoked_by BIGINT NOT NULL DEFAULT 0,
  created_by BIGINT NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_by BIGINT NOT NULL DEFAULT 0,
  deleted_at TIMESTAMPTZ NULL,
  PRIMARY KEY (id)
);

CREATE TABLE sys_notice_read (
  tenant_id BIGINT NOT NULL,
  notice_id BIGINT NOT NULL,
  user_id BIGINT NOT NULL,
  read_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  PRIMARY KEY (notice_id, user_id)
);

CREATE TABLE sys_notification (
  id BIGSERIAL,
  tenant_id BIGINT NOT NULL,
  user_id BIGINT NOT NULL,
  channel VARCHAR(32) NOT NULL,
  title VARCHAR(255) NOT NULL,
  content TEXT NOT NULL DEFAULT '',
  biz_type VARCHAR(64) NOT NULL DEFAULT '',
  biz_id VARCHAR(64) NOT NULL DEFAULT '',
  read_at TIMESTAMPTZ NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  created_by BIGINT NOT NULL DEFAULT 0,
  PRIMARY KEY (id)
);
CREATE INDEX idx_notification_user_created ON sys_notification (user_id, created_at DESC);

CREATE TABLE sys_user_notify_pref (
  tenant_id BIGINT NOT NULL,
  user_id BIGINT NOT NULL,
  channel VARCHAR(32) NOT NULL,
  enabled SMALLINT NOT NULL DEFAULT 1,
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_by BIGINT NOT NULL DEFAULT 0,
  PRIMARY KEY (user_id, channel)
);

CREATE TABLE sys_refresh_token (
  id BIGSERIAL,
  user_id BIGINT NOT NULL,
  -- Tenant the session was opened in. Without it a refresh cannot tell which
  -- tenant the client was using, so a multi-tenant user is silently moved back
  -- to their first membership on every token refresh.
  tenant_id BIGINT NULL,
  token_hash CHAR(64) NOT NULL,
  family_id CHAR(36) NOT NULL,
  device_id VARCHAR(128) NOT NULL DEFAULT '',
  device_name VARCHAR(255) NOT NULL DEFAULT '',
  login_method VARCHAR(32) NOT NULL DEFAULT 'password',
  ip VARCHAR(64) NOT NULL DEFAULT '',
  user_agent VARCHAR(1000) NOT NULL DEFAULT '',
  expires_at TIMESTAMPTZ NOT NULL,
  last_used_at TIMESTAMPTZ NULL,
  revoked_at TIMESTAMPTZ NULL,
  revoke_reason VARCHAR(255) NOT NULL DEFAULT '',
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  created_by BIGINT NOT NULL DEFAULT 0,
  PRIMARY KEY (id),
  CONSTRAINT uk_refresh_token_hash UNIQUE (token_hash)
);

CREATE TABLE sys_api_token (
  id BIGSERIAL,
  tenant_id BIGINT NOT NULL,
  user_id BIGINT NOT NULL,
  name VARCHAR(128) NOT NULL,
  token_prefix VARCHAR(32) NOT NULL,
  token_hash CHAR(64) NOT NULL,
  scopes JSONB NOT NULL,
  ip_allowlist JSONB NULL,
  status SMALLINT NOT NULL DEFAULT 1,
  expires_at TIMESTAMPTZ NULL,
  last_used_at TIMESTAMPTZ NULL,
  last_used_ip VARCHAR(64) NOT NULL DEFAULT '',
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  created_by BIGINT NOT NULL DEFAULT 0,
  revoked_at TIMESTAMPTZ NULL,
  PRIMARY KEY (id),
  CONSTRAINT uk_api_token_hash UNIQUE (token_hash)
);

CREATE TABLE sys_login_log (
  id BIGSERIAL,
  user_id BIGINT NULL,
  username VARCHAR(64) NOT NULL,
  login_method VARCHAR(32) NOT NULL DEFAULT 'password',
  success SMALLINT NOT NULL,
  failure_reason VARCHAR(255) NOT NULL DEFAULT '',
  ip VARCHAR(64) NOT NULL DEFAULT '',
  location VARCHAR(255) NOT NULL DEFAULT '',
  user_agent VARCHAR(1000) NOT NULL DEFAULT '',
  device_type VARCHAR(64) NOT NULL DEFAULT '',
  os VARCHAR(128) NOT NULL DEFAULT '',
  browser VARCHAR(128) NOT NULL DEFAULT '',
  is_suspicious BOOLEAN NOT NULL DEFAULT FALSE,

  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  created_by BIGINT NOT NULL DEFAULT 0,
  PRIMARY KEY (id)
);

CREATE TABLE sys_operation_log (
  id BIGSERIAL,
  tenant_id BIGINT NOT NULL,
  user_id BIGINT NULL,
  username VARCHAR(64) NOT NULL DEFAULT '',
  module VARCHAR(64) NOT NULL,
  action VARCHAR(64) NOT NULL,
  permission VARCHAR(128) NOT NULL DEFAULT '',
  method VARCHAR(16) NOT NULL,
  path VARCHAR(500) NOT NULL,
  target_type VARCHAR(64) NOT NULL DEFAULT '',
  target_id VARCHAR(128) NOT NULL DEFAULT '',
  target_name VARCHAR(255) NOT NULL DEFAULT '',
  request_id VARCHAR(64) NOT NULL DEFAULT '',
  ip VARCHAR(64) NOT NULL DEFAULT '',
  user_agent VARCHAR(1000) NOT NULL DEFAULT '',
  success SMALLINT NOT NULL,
  status_code INT NOT NULL DEFAULT 200,
  duration_ms BIGINT NOT NULL DEFAULT 0,
  before_json JSONB NULL,
  after_json JSONB NULL,
  error_code VARCHAR(64) NOT NULL DEFAULT '',
  error_message VARCHAR(1000) NOT NULL DEFAULT '',
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  created_by BIGINT NOT NULL DEFAULT 0,
  PRIMARY KEY (id)
);

CREATE TABLE sys_error_log (
  id BIGSERIAL,
  tenant_id BIGINT NOT NULL,
  request_id VARCHAR(64) NOT NULL DEFAULT '',
  user_id BIGINT NULL,
  module VARCHAR(64) NOT NULL DEFAULT '',
  method VARCHAR(16) NOT NULL DEFAULT '',
  path VARCHAR(500) NOT NULL DEFAULT '',
  error_code VARCHAR(64) NOT NULL DEFAULT '',
  message VARCHAR(1000) NOT NULL,
  stack_trace TEXT NULL,
  ip VARCHAR(64) NOT NULL DEFAULT '',
  user_agent VARCHAR(1000) NOT NULL DEFAULT '',
  resolved SMALLINT NOT NULL DEFAULT 0,
  resolved_by BIGINT NULL,
  resolved_at TIMESTAMPTZ NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  created_by BIGINT NOT NULL DEFAULT 0,
  PRIMARY KEY (id)
);

CREATE TABLE sys_config_log (
  id BIGSERIAL,
  config_type VARCHAR(32) NOT NULL,
  config_key VARCHAR(128) NOT NULL,
  operator_id BIGINT NOT NULL,
  before_json JSONB NULL,
  after_json JSONB NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  created_by BIGINT NOT NULL DEFAULT 0,
  PRIMARY KEY (id)
);

CREATE TABLE sys_webhook (
  id BIGSERIAL,
  tenant_id BIGINT NOT NULL,
  name VARCHAR(128) NOT NULL,
  url VARCHAR(1000) NOT NULL,
  secret_encrypted TEXT NOT NULL,
  events JSONB NOT NULL,
	  headers_encrypted TEXT NULL,
	  allow_private_network SMALLINT NOT NULL DEFAULT 0,
  timeout_seconds INT NOT NULL DEFAULT 10,
  max_retries INT NOT NULL DEFAULT 3,
  status SMALLINT NOT NULL DEFAULT 1,
  last_success_at TIMESTAMPTZ NULL,
  last_failure_at TIMESTAMPTZ NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  created_by BIGINT NOT NULL DEFAULT 0,
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_by BIGINT NOT NULL DEFAULT 0,
  deleted_at TIMESTAMPTZ NULL,
  PRIMARY KEY (id)
);

CREATE TABLE sys_scheduled_job (
  id BIGSERIAL,
  tenant_id BIGINT NOT NULL,
  name VARCHAR(128) NOT NULL,
  code VARCHAR(128) NOT NULL,
  handler VARCHAR(128) NOT NULL,
  cron_expression VARCHAR(128) NOT NULL,
  payload JSONB NULL,
  timeout_seconds INT NOT NULL DEFAULT 60,
  max_retries INT NOT NULL DEFAULT 0,
  status SMALLINT NOT NULL DEFAULT 0,
  last_run_at TIMESTAMPTZ NULL,
  next_run_at TIMESTAMPTZ NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  created_by BIGINT NOT NULL DEFAULT 0,
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_by BIGINT NOT NULL DEFAULT 0,
  deleted_at TIMESTAMPTZ NULL,
  PRIMARY KEY (id)
);

CREATE TABLE sys_job_log (
  id BIGSERIAL,
  tenant_id BIGINT NOT NULL,
  job_id BIGINT NOT NULL,
  trigger_type VARCHAR(32) NOT NULL,
  started_at TIMESTAMPTZ NOT NULL,
  finished_at TIMESTAMPTZ NULL,
  success SMALLINT NULL,
  attempt INT NOT NULL DEFAULT 1,
  output TEXT NULL,
  error_message TEXT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  created_by BIGINT NOT NULL DEFAULT 0,
  PRIMARY KEY (id)
);

-- Business keys are unique among LIVE rows only.
--
-- Deletes in this schema are soft: the row stays behind with deleted_at set, so
-- a plain UNIQUE constraint would keep its code occupied forever and a record
-- that was deleted by mistake could never be recreated. A partial unique index
-- gives both: the audit trail keeps the tombstone (and its old code), while the
-- code is free again for the next record. Postgres cannot express this as a
-- table constraint, so these live next to the other indexes — and
-- store.bootstrapStatements carries the same DDL for databases that were
-- created before the constraints were dropped.
-- Department, position, role, dictionary and job codes are unique per tenant.
-- Username and menu code stay global.
CREATE UNIQUE INDEX uk_department_code ON sys_department (tenant_id, code) WHERE deleted_at IS NULL;

CREATE UNIQUE INDEX uk_position_code ON sys_position (tenant_id, code) WHERE deleted_at IS NULL;

CREATE UNIQUE INDEX uk_user_username ON sys_user (username) WHERE deleted_at IS NULL;

CREATE UNIQUE INDEX uk_role_code ON sys_role (tenant_id, code) WHERE deleted_at IS NULL;

CREATE UNIQUE INDEX uk_menu_code ON sys_menu (code) WHERE deleted_at IS NULL;

CREATE UNIQUE INDEX uk_dict_type_code ON sys_dict_type (tenant_id, code) WHERE deleted_at IS NULL;

CREATE UNIQUE INDEX uk_dict_item_value ON sys_dict_item (tenant_id, dict_type_id, value) WHERE deleted_at IS NULL;

-- A dictionary type has at most one default item among LIVE rows. The write
-- path enforces the same rule with a two-step save (clear the type's other
-- defaults first, then set the new one), so the immediate constraint is never
-- tripped in normal operation; it stands as the concurrency guard.
CREATE UNIQUE INDEX uk_dict_item_default ON sys_dict_item (tenant_id, dict_type_id) WHERE is_default = TRUE AND deleted_at IS NULL;

CREATE UNIQUE INDEX uk_scheduled_job_code ON sys_scheduled_job (tenant_id, code) WHERE deleted_at IS NULL;

CREATE INDEX idx_department_parent ON sys_department (parent_id);

CREATE INDEX idx_user_department ON sys_user (department_id);

-- Addresses are login/invite identities, so they must not repeat. The partial
-- predicate keeps "not set" (empty string) unconstrained.
CREATE UNIQUE INDEX uk_user_email ON sys_user (lower(email))
  WHERE email <> '' AND deleted_at IS NULL;

CREATE UNIQUE INDEX uk_user_mobile ON sys_user (mobile)
  WHERE mobile <> '' AND deleted_at IS NULL;

CREATE INDEX idx_user_position_position ON sys_user_position (position_id);

CREATE INDEX idx_user_role_role ON sys_user_role (role_id);

CREATE INDEX idx_role_department_department ON sys_role_department (department_id);

CREATE INDEX idx_menu_parent ON sys_menu (parent_id);

CREATE INDEX idx_role_menu_menu ON sys_role_menu (menu_id);

CREATE INDEX idx_config_group ON sys_config (config_group);

CREATE INDEX idx_auth_provider_type ON sys_auth_provider (provider_type);

CREATE INDEX idx_storage_default ON sys_storage_config (is_default, status);

CREATE INDEX idx_file_uploader ON sys_file (uploader_id);

CREATE INDEX idx_file_status ON sys_file (status);

CREATE INDEX idx_file_tenant_created ON sys_file (tenant_id, created_at DESC);

CREATE INDEX idx_file_reference_tenant_business
  ON sys_file_reference (tenant_id, business_type, business_id);

CREATE INDEX idx_notice_tenant_status ON sys_notice (tenant_id, status, publish_at);

CREATE INDEX idx_notice_status_publish ON sys_notice (status, publish_at);
CREATE INDEX idx_notice_due_notify ON sys_notice (publish_at)
  WHERE deleted_at IS NULL AND status = 'published' AND notified_at IS NULL;

CREATE INDEX idx_notice_read_user ON sys_notice_read (user_id, read_at);

CREATE INDEX idx_refresh_token_user ON sys_refresh_token (user_id, revoked_at, expires_at);

CREATE INDEX idx_refresh_token_family ON sys_refresh_token (family_id);

CREATE INDEX idx_api_token_user ON sys_api_token (user_id, status);

CREATE INDEX idx_api_token_tenant_user ON sys_api_token (tenant_id, user_id, status);

CREATE INDEX idx_login_log_user ON sys_login_log (user_id, created_at);

CREATE INDEX idx_login_log_username ON sys_login_log (username, created_at);

CREATE INDEX idx_login_log_success ON sys_login_log (success, created_at);

CREATE INDEX idx_operation_log_user ON sys_operation_log (user_id, created_at);

CREATE INDEX idx_operation_log_module ON sys_operation_log (module, created_at);

CREATE INDEX idx_operation_log_target ON sys_operation_log (target_type, target_id);

CREATE INDEX idx_operation_log_request ON sys_operation_log (request_id);

CREATE INDEX idx_operation_log_tenant_created ON sys_operation_log (tenant_id, created_at DESC);

CREATE INDEX idx_error_log_created ON sys_error_log (created_at);

CREATE INDEX idx_error_log_resolved ON sys_error_log (resolved, created_at);

CREATE INDEX idx_error_log_request ON sys_error_log (request_id);

CREATE INDEX idx_error_log_tenant_created ON sys_error_log (tenant_id, created_at DESC);

CREATE INDEX idx_config_log_key ON sys_config_log (config_type, config_key, created_at);

CREATE INDEX idx_job_log_job ON sys_job_log (job_id, started_at);

CREATE INDEX idx_job_log_success ON sys_job_log (success, started_at);

CREATE INDEX idx_job_log_tenant_job ON sys_job_log (tenant_id, job_id, started_at DESC);

-- Notification history is read per tenant+user in reverse time order.
CREATE INDEX idx_notification_tenant_user ON sys_notification (tenant_id, user_id, created_at DESC);

-- Runtime state of every compiled-in plugin. Rows are upserted from the
-- plugin registry on startup; a plugin that is compiled in but has no row is
-- treated as disabled.
CREATE TABLE sys_plugin (
  -- The plugin ID is the natural key: it is what sys_menu.plugin_id, the API
  -- paths, the logs and the registry all use, so a surrogate would only add a
  -- join. The length matches the plugin.ID contract.
  plugin_id VARCHAR(16) PRIMARY KEY,
  name VARCHAR(128) NOT NULL DEFAULT '',
  version VARCHAR(32) NOT NULL DEFAULT '',
  description VARCHAR(500) NOT NULL DEFAULT '',
  -- Mirrors the catalog so SQL-only consumers can reason about it.
  is_builtin BOOLEAN NOT NULL DEFAULT FALSE,
  enabled BOOLEAN NOT NULL DEFAULT FALSE,
  -- Why the plugin is disabled (auto-disabled after a failed migration) or why
  -- the last enable attempt failed. Truncated to 1000 chars by the store.
  last_error TEXT NOT NULL DEFAULT '',
  enabled_at TIMESTAMPTZ NULL,
  -- Set when Install finishes (migrations applied) without enabling. Enable
  -- also stamps this via COALESCE so enable-without-prior-install stays consistent.
  installed_at TIMESTAMPTZ NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  created_by BIGINT NOT NULL DEFAULT 0,
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_by BIGINT NOT NULL DEFAULT 0
);

-- Forward-only migration bookkeeping. A row is inserted in the SAME
-- transaction as the migration SQL, so its presence means "committed".
CREATE TABLE sys_plugin_migration (
  plugin_id VARCHAR(16) NOT NULL,
  version VARCHAR(32) NOT NULL,
  name VARCHAR(128) NOT NULL DEFAULT '',
  -- SHA-256 hex of the file body; a mismatch fails the enable closed.
  checksum VARCHAR(64) NOT NULL,
  applied_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  PRIMARY KEY (plugin_id, version)
);

CREATE INDEX idx_menu_plugin_id ON sys_menu (plugin_id);

-- Returns true when p_parent_id is in the descendant tree of p_menu_id (cycle check for menu moves).
CREATE OR REPLACE FUNCTION menu_parent_is_descendant(p_menu_id BIGINT, p_parent_id BIGINT)
RETURNS BOOLEAN LANGUAGE sql STABLE AS $$
  WITH RECURSIVE descendants AS (
    SELECT id FROM sys_menu WHERE parent_id = p_menu_id AND deleted_at IS NULL
    UNION ALL
    SELECT m.id FROM sys_menu m INNER JOIN descendants d ON m.parent_id = d.id
    WHERE m.deleted_at IS NULL
  )
  SELECT EXISTS(SELECT 1 FROM descendants WHERE id = p_parent_id);
$$;

-- Admin-maintainable translations for preset (seeded) data. The default
-- language (Chinese) lives in the owning table's text column; this table only
-- stores other languages, keyed by a stable entity key that survives renames:
--   menu.<menu code>          page/directory rows of sys_menu
--   perm.<permission code>    button rows of sys_menu (written by the sync)
--   role.<role code>[.remark] sys_role
--   dept.<department code>    sys_department
--   post.<position code>      sys_position
--   job.<job code>            sys_scheduled_job
--   storage.<storage type>    sys_storage_config
--   dicttype.<dict code>      sys_dict_type
--   dict.<dict code>.<value>  sys_dict_item
--
-- tenant_id 0 is the platform default. Entity keys are built from business
-- codes, and those codes repeat per tenant (role.admin exists in every
-- tenant), so a translation without tenant_id would make one tenant's label
-- change another's. A tenant row wins over the platform row of the same
-- (entity_key, lang); deleting a tenant row falls back to the platform text.
CREATE TABLE sys_translation (
  id BIGSERIAL,
  tenant_id BIGINT NOT NULL DEFAULT 0,
  entity_key VARCHAR(191) NOT NULL,
  lang VARCHAR(16) NOT NULL,
  value TEXT NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  created_by BIGINT NOT NULL DEFAULT 0,
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_by BIGINT NOT NULL DEFAULT 0,
  PRIMARY KEY (id),
  CONSTRAINT uk_translation_entity_lang UNIQUE (tenant_id, entity_key, lang),
  CONSTRAINT ck_sys_translation_tenant CHECK (tenant_id >= 0)
);

CREATE INDEX idx_translation_lang ON sys_translation (tenant_id, lang);

-- Every preset entity that can carry a translation, as
-- (entity_key, source text, updated_at). One view so a translatable namespace is
-- registered in exactly one place: the translation page's lists, its candidates
-- and the source-language write-back all read from here. Adding a namespace is
-- one UNION here plus one case in store.(*AuthRepository).writeSourceText.
CREATE VIEW v_preset_entity AS
  -- Only the first branch carries the column names: a CREATE VIEW column list
  -- is legal SQL but sqlc cannot read it, so the aliases live on the SELECT.
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
    WHERE deleted_at IS NULL
  UNION ALL
  SELECT 'dicttype.' || code, name, updated_at FROM sys_dict_type
    WHERE deleted_at IS NULL
  UNION ALL
  SELECT 'dict.' || dt.code || '.' || di.value, di.label, di.updated_at
    FROM sys_dict_item di JOIN sys_dict_type dt ON dt.id = di.dict_type_id
    WHERE di.deleted_at IS NULL AND dt.deleted_at IS NULL;

-- The same catalog addressed by row id instead of entity key, for the write
-- side: a save made in another language has to keep the source columns intact
-- and refile the entity's translations when its code changed, so it needs the
-- row's current code and text before it writes. namespace names the owning
-- table (menu covers permission buttons too; they are sys_menu rows as well).
CREATE VIEW v_preset_source_row AS
  -- Only the first branch carries the column names: a CREATE VIEW column list
  -- is legal SQL but sqlc cannot read it, so the aliases live on the SELECT.
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
    WHERE deleted_at IS NULL;

-- Shared-database tenants. Row level security for these tables, and the
-- BEFORE INSERT trigger that fills tenant_id, live in migrations/013_multitenant.sql.
-- Bootstrap must SET app.rls_bypass=on before it reads or writes them.
CREATE TABLE sys_tenant (
  id BIGSERIAL,
  code VARCHAR(64) NOT NULL,
  name VARCHAR(128) NOT NULL,
  status SMALLINT NOT NULL DEFAULT 1,
  remark VARCHAR(500) NOT NULL DEFAULT '',
  -- purge_status is '' | pending_delete | purged. purge_after is when a
  -- pending delete becomes eligible for tenant.purge_due. Tenant id 1 is
  -- never purged.
  purge_after TIMESTAMPTZ NULL,
  purge_status VARCHAR(32) NOT NULL DEFAULT '',
  -- Public host label for {slug}.{base}. Empty means no subdomain.
  -- Unique among live tenants when non-empty. id 1 is backfilled to 'default'.
  slug VARCHAR(64) NOT NULL DEFAULT '',
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  created_by BIGINT NOT NULL DEFAULT 0,
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_by BIGINT NOT NULL DEFAULT 0,
  deleted_at TIMESTAMPTZ NULL,
  PRIMARY KEY (id)
);

CREATE UNIQUE INDEX uk_tenant_code ON sys_tenant (code) WHERE deleted_at IS NULL;

CREATE UNIQUE INDEX uk_tenant_slug ON sys_tenant (slug) WHERE deleted_at IS NULL AND slug <> '';

-- Custom hosts. No row-level security: host resolution runs before a tenant
-- is selected, and the lookup has to see every live tenant.
CREATE TABLE sys_tenant_domain (
  id BIGSERIAL PRIMARY KEY,
  tenant_id BIGINT NOT NULL,
  host VARCHAR(255) NOT NULL,
  verified_at TIMESTAMPTZ NULL,
  is_primary BOOLEAN NOT NULL DEFAULT FALSE,

  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  created_by BIGINT NOT NULL DEFAULT 0,
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_by BIGINT NOT NULL DEFAULT 0
);

CREATE UNIQUE INDEX uk_tenant_domain_host ON sys_tenant_domain (host);

CREATE INDEX idx_tenant_domain_tenant ON sys_tenant_domain (tenant_id);

CREATE TABLE sys_tenant_member (
  tenant_id BIGINT NOT NULL,
  user_id BIGINT NOT NULL,
  department_id BIGINT NULL,
  is_tenant_admin BOOLEAN NOT NULL DEFAULT FALSE,

  -- Kept for the membership queries: true when the member was copied from a
  -- super administrator.
  is_platform_operator BOOLEAN NOT NULL DEFAULT FALSE,
  status SMALLINT NOT NULL DEFAULT 1,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  created_by BIGINT NOT NULL DEFAULT 0,
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_by BIGINT NOT NULL DEFAULT 0,
  PRIMARY KEY (tenant_id, user_id)
);

CREATE INDEX idx_tenant_member_user ON sys_tenant_member (user_id);

-- Platform staff, independent of any tenant. A platform operator may hold a
-- membership, but does not need one: this table is the source of truth for
-- "may enter the operator console".
CREATE TABLE sys_platform_operator (
  user_id BIGINT PRIMARY KEY,
  granted_by BIGINT NOT NULL DEFAULT 0,
  note VARCHAR(255) NOT NULL DEFAULT '',
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  created_by BIGINT NOT NULL DEFAULT 0
);

CREATE TABLE sys_tenant_invite (
  id BIGSERIAL PRIMARY KEY,
  tenant_id BIGINT NOT NULL,
  email VARCHAR(255) NOT NULL DEFAULT '',
  role_ids BIGINT[] NOT NULL DEFAULT '{}'::bigint[],
  department_id BIGINT NULL,
  is_tenant_admin BOOLEAN NOT NULL DEFAULT FALSE,

  token_hash CHAR(64) NOT NULL,
  -- pending | accepted | revoked | expired
  status VARCHAR(16) NOT NULL DEFAULT 'pending',
  invited_by BIGINT NOT NULL,
  accepted_by BIGINT NULL,
  expires_at TIMESTAMPTZ NOT NULL,
  accepted_at TIMESTAMPTZ NULL,
  revoked_at TIMESTAMPTZ NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  created_by BIGINT NOT NULL DEFAULT 0,
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_by BIGINT NOT NULL DEFAULT 0
);

CREATE UNIQUE INDEX uk_tenant_invite_token ON sys_tenant_invite (token_hash);

CREATE INDEX idx_tenant_invite_tenant ON sys_tenant_invite (tenant_id, id DESC);

CREATE UNIQUE INDEX uk_tenant_invite_pending_email
  ON sys_tenant_invite (tenant_id, lower(email))
  WHERE status = 'pending' AND email <> '';

CREATE TABLE sys_plan (
  id BIGSERIAL,
  code VARCHAR(64) NOT NULL,
  name VARCHAR(128) NOT NULL,
  remark VARCHAR(500) NOT NULL DEFAULT '',
  status SMALLINT NOT NULL DEFAULT 1,
  sort INT NOT NULL DEFAULT 0,
  -- Bumped on every entitlement change. Subscription snapshots record the
  -- version they were sold at.
  version INT NOT NULL DEFAULT 1,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  created_by BIGINT NOT NULL DEFAULT 0,
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_by BIGINT NOT NULL DEFAULT 0,
  PRIMARY KEY (id)
);

CREATE TABLE sys_plan_version (
  id BIGSERIAL PRIMARY KEY,
  plan_id BIGINT NOT NULL,
  version INT NOT NULL,
  snapshot JSONB NOT NULL DEFAULT '{}'::jsonb,
  created_by BIGINT NOT NULL DEFAULT 0,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX uk_plan_code ON sys_plan (code);

CREATE UNIQUE INDEX uk_plan_version ON sys_plan_version (plan_id, version);

CREATE INDEX idx_plan_version_plan ON sys_plan_version (plan_id, version DESC);

CREATE TABLE sys_plan_entitlement (
  id BIGSERIAL,
  plan_id BIGINT NOT NULL,
  entitlement_key VARCHAR(128) NOT NULL,
  entitlement_value TEXT NOT NULL DEFAULT '',
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  created_by BIGINT NOT NULL DEFAULT 0,
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_by BIGINT NOT NULL DEFAULT 0,
  PRIMARY KEY (id)
);

CREATE UNIQUE INDEX uk_plan_entitlement ON sys_plan_entitlement (plan_id, entitlement_key);

CREATE TABLE sys_tenant_subscription (
  id BIGSERIAL,
  tenant_id BIGINT NOT NULL,
  plan_id BIGINT NOT NULL,
  -- trial | active | overdue | cancelled. cancelled is terminal.
  status VARCHAR(32) NOT NULL DEFAULT 'active',
  started_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  ended_at TIMESTAMPTZ NULL,
  trial_ends_at TIMESTAMPTZ NULL,
  -- When an overdue subscription must become cancelled.
  grace_ends_at TIMESTAMPTZ NULL,
  -- The plan version this subscription was sold at, plus its full snapshot.
  -- Entitlement and quota lookups read the snapshot so a later plan edit does
  -- not retroactively change what this tenant is entitled to.
  plan_version_id BIGINT NULL,
  plan_snapshot JSONB NOT NULL DEFAULT '{}'::jsonb,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  created_by BIGINT NOT NULL DEFAULT 0,
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_by BIGINT NOT NULL DEFAULT 0,
  PRIMARY KEY (id)
);

CREATE UNIQUE INDEX uk_tenant_subscription_active
  ON sys_tenant_subscription (tenant_id) WHERE status = 'active';

CREATE TABLE sys_usage_counter (
  tenant_id BIGINT NOT NULL,
  quota_key VARCHAR(128) NOT NULL,
  period_key VARCHAR(32) NOT NULL,
  used_value BIGINT NOT NULL DEFAULT 0,
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_by BIGINT NOT NULL DEFAULT 0,
  PRIMARY KEY (tenant_id, quota_key, period_key)
);

CREATE TABLE sys_tenant_quota (
  tenant_id BIGINT NOT NULL,
  quota_key VARCHAR(128) NOT NULL,
  limit_value BIGINT NOT NULL,
  used_value BIGINT NOT NULL DEFAULT 0,
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_by BIGINT NOT NULL DEFAULT 0,
  PRIMARY KEY (tenant_id, quota_key)
);

CREATE TABLE sys_outbox (
  id BIGSERIAL,
  tenant_id BIGINT NOT NULL,
  event_type VARCHAR(128) NOT NULL,
  payload JSONB NOT NULL DEFAULT '{}'::jsonb,
  -- 0 = pending, 1 = sent, 2 = dead, 3 = processing.
  status SMALLINT NOT NULL DEFAULT 0,
  attempts INT NOT NULL DEFAULT 0,
  last_error VARCHAR(1000) NOT NULL DEFAULT '',
  available_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  -- Set while a worker is delivering. FOR UPDATE SKIP LOCKED on claim means a
  -- second worker skips the row instead of delivering it again.
  claimed_at TIMESTAMPTZ NULL,
  claimed_by VARCHAR(64) NOT NULL DEFAULT '',
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  created_by BIGINT NOT NULL DEFAULT 0,
  sent_at TIMESTAMPTZ NULL,
  PRIMARY KEY (id)
);

CREATE INDEX idx_outbox_pending ON sys_outbox (status, id);

CREATE TABLE sys_tenant_impersonation (
  id BIGSERIAL,
  tenant_id BIGINT NOT NULL,
  actor_user_id BIGINT NOT NULL,
  target_user_id BIGINT NOT NULL,
  reason VARCHAR(500) NOT NULL DEFAULT '',
  started_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  expires_at TIMESTAMPTZ NOT NULL,
  ended_at TIMESTAMPTZ NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  created_by BIGINT NOT NULL DEFAULT 0,
  PRIMARY KEY (id)
);

CREATE INDEX idx_tenant_impersonation_open ON sys_tenant_impersonation (tenant_id, actor_user_id)
  WHERE ended_at IS NULL;

-- ---- tenant-composite referential integrity -------------------------------
-- RLS filters reads but says nothing about what a row may point at: a child
-- could reference a parent id owned by another tenant and pass WITH CHECK.
-- Every tenant-scoped parent is therefore unique on (tenant_id, id) and every
-- child references that pair. The same DDL runs from
-- store.bootstrapStatements, which is what actually creates it on existing
-- databases (migrations/022_schema_hardening.sql is the written record).
CREATE UNIQUE INDEX uk_department_tenant_id ON sys_department (tenant_id, id);
CREATE UNIQUE INDEX uk_position_tenant_id ON sys_position (tenant_id, id);
CREATE UNIQUE INDEX uk_role_tenant_id ON sys_role (tenant_id, id);
CREATE UNIQUE INDEX uk_dict_type_tenant_id ON sys_dict_type (tenant_id, id);
CREATE UNIQUE INDEX uk_file_tenant_id ON sys_file (tenant_id, id);
CREATE UNIQUE INDEX uk_notice_tenant_id ON sys_notice (tenant_id, id);
CREATE UNIQUE INDEX uk_scheduled_job_tenant_id ON sys_scheduled_job (tenant_id, id);

ALTER TABLE sys_user_role ADD CONSTRAINT fk_user_role_role_tenant
  FOREIGN KEY (tenant_id, role_id) REFERENCES sys_role (tenant_id, id) ON DELETE CASCADE;

ALTER TABLE sys_user_position ADD CONSTRAINT fk_user_position_position_tenant
  FOREIGN KEY (tenant_id, position_id) REFERENCES sys_position (tenant_id, id) ON DELETE CASCADE;

ALTER TABLE sys_role_department ADD CONSTRAINT fk_role_department_department_tenant
  FOREIGN KEY (tenant_id, department_id) REFERENCES sys_department (tenant_id, id) ON DELETE CASCADE;

ALTER TABLE sys_dict_item ADD CONSTRAINT fk_dict_item_type_tenant
  FOREIGN KEY (tenant_id, dict_type_id) REFERENCES sys_dict_type (tenant_id, id) ON DELETE CASCADE;

ALTER TABLE sys_file_reference ADD CONSTRAINT fk_file_reference_file_tenant
  FOREIGN KEY (tenant_id, file_id) REFERENCES sys_file (tenant_id, id) ON DELETE CASCADE;

ALTER TABLE sys_notice_read ADD CONSTRAINT fk_notice_read_notice_tenant
  FOREIGN KEY (tenant_id, notice_id) REFERENCES sys_notice (tenant_id, id) ON DELETE CASCADE;

ALTER TABLE sys_job_log ADD CONSTRAINT fk_job_log_job_tenant
  FOREIGN KEY (tenant_id, job_id) REFERENCES sys_scheduled_job (tenant_id, id) ON DELETE CASCADE;

-- 0/1 flag CHECK constraints (ck_<table>_<column>) are declared in
-- migrations/022_schema_hardening.sql and store.bootstrapStatements. They are
-- not repeated here: sqlc only needs tables, columns and keys.

