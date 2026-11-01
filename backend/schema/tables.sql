CREATE TABLE sys_department (
  id BIGSERIAL,
  parent_id BIGINT NOT NULL DEFAULT 0,
  ancestors VARCHAR(1000) NOT NULL DEFAULT '',
  code VARCHAR(64) NOT NULL,
  name VARCHAR(128) NOT NULL,
  leader_user_id BIGINT NULL,
  sort INT NOT NULL DEFAULT 0,
  status SMALLINT NOT NULL DEFAULT 1,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  deleted_at TIMESTAMPTZ NULL,
  PRIMARY KEY (id)
);

CREATE TABLE sys_position (
  id BIGSERIAL,
  code VARCHAR(64) NOT NULL,
  name VARCHAR(128) NOT NULL,
  sort INT NOT NULL DEFAULT 0,
  status SMALLINT NOT NULL DEFAULT 1,
  remark VARCHAR(500) NOT NULL DEFAULT '',
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
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
  is_super_admin SMALLINT NOT NULL DEFAULT 0,
  must_change_password SMALLINT NOT NULL DEFAULT 1,
  password_changed_at TIMESTAMPTZ NULL,
  last_login_at TIMESTAMPTZ NULL,
  last_login_ip VARCHAR(64) NOT NULL DEFAULT '',
  failed_login_count INT NOT NULL DEFAULT 0,
  locked_until TIMESTAMPTZ NULL,
  token_version BIGINT NOT NULL DEFAULT 0,
  remark VARCHAR(500) NOT NULL DEFAULT '',
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  deleted_at TIMESTAMPTZ NULL,
  PRIMARY KEY (id)
);

CREATE TABLE sys_user_position (
  user_id BIGINT NOT NULL,
  position_id BIGINT NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  PRIMARY KEY (user_id, position_id)
);

CREATE TABLE sys_role (
  id BIGSERIAL,
  code VARCHAR(64) NOT NULL,
  name VARCHAR(128) NOT NULL,
  data_scope VARCHAR(32) NOT NULL DEFAULT 'self',
  sort INT NOT NULL DEFAULT 0,
  status SMALLINT NOT NULL DEFAULT 1,
  is_system SMALLINT NOT NULL DEFAULT 0,
  remark VARCHAR(500) NOT NULL DEFAULT '',
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  deleted_at TIMESTAMPTZ NULL,
  PRIMARY KEY (id)
);

CREATE TABLE sys_user_role (
  user_id BIGINT NOT NULL,
  role_id BIGINT NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  PRIMARY KEY (user_id, role_id)
);

CREATE TABLE sys_role_department (
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
  visible SMALLINT NOT NULL DEFAULT 1,
  keep_alive SMALLINT NOT NULL DEFAULT 0,
  affix_tab SMALLINT NOT NULL DEFAULT 0,
  external_url VARCHAR(500) NOT NULL DEFAULT '',
  -- Owning plugin. '' means a core menu, which is always visible; any other
  -- value is only visible while that plugin is enabled.
  plugin_id VARCHAR(16) NOT NULL DEFAULT '',
  -- Permission buttons only: the endpoint the code guards. The startup sync
  -- owns these three columns and keeps them aligned with the route table;
  -- they are informational (enforcement is by code), so a code guarding
  -- several routes shows the first one.
  method VARCHAR(16) NOT NULL DEFAULT '',
  path VARCHAR(255) NOT NULL DEFAULT '',
  is_sensitive SMALLINT NOT NULL DEFAULT 0,
  -- 1 = the row is owned by the route-table sync (a declared permission code).
  -- Those rows may only be renamed and re-sorted, and the sync retires them
  -- when the route disappears. 0 = an ordinary row, including buttons an
  -- administrator created by hand, which the sync never touches.
  managed SMALLINT NOT NULL DEFAULT 0,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  deleted_at TIMESTAMPTZ NULL,
  PRIMARY KEY (id)
);

CREATE TABLE sys_role_menu (
  role_id BIGINT NOT NULL,
  menu_id BIGINT NOT NULL,
  PRIMARY KEY (role_id, menu_id)
);

CREATE TABLE sys_dict_type (
  id BIGSERIAL,
  code VARCHAR(128) NOT NULL,
  name VARCHAR(128) NOT NULL,
  display_type VARCHAR(32) NOT NULL DEFAULT 'segmented',
  status SMALLINT NOT NULL DEFAULT 1,
  is_system SMALLINT NOT NULL DEFAULT 0,
  remark VARCHAR(500) NOT NULL DEFAULT '',
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  deleted_at TIMESTAMPTZ NULL,
  PRIMARY KEY (id)
);

CREATE TABLE sys_dict_item (
  id BIGSERIAL,
  dict_type_id BIGINT NOT NULL,
  label VARCHAR(128) NOT NULL,
  value VARCHAR(255) NOT NULL,
  color VARCHAR(32) NOT NULL DEFAULT '',
  css_class VARCHAR(128) NOT NULL DEFAULT '',
  sort INT NOT NULL DEFAULT 0,
  status SMALLINT NOT NULL DEFAULT 1,
  is_default SMALLINT NOT NULL DEFAULT 0,
  remark VARCHAR(500) NOT NULL DEFAULT '',
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  deleted_at TIMESTAMPTZ NULL,
  PRIMARY KEY (id)
);

CREATE TABLE sys_config (
  id BIGSERIAL,
  config_group VARCHAR(64) NOT NULL,
  config_key VARCHAR(128) NOT NULL,
  name VARCHAR(128) NOT NULL,
  value_type VARCHAR(32) NOT NULL DEFAULT 'string',
  config_value TEXT NOT NULL,
  default_value TEXT NULL,
  is_secret SMALLINT NOT NULL DEFAULT 0,
  is_readonly SMALLINT NOT NULL DEFAULT 0,
  effect_mode VARCHAR(32) NOT NULL DEFAULT 'immediate',
  description VARCHAR(500) NOT NULL DEFAULT '',
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  PRIMARY KEY (id),
  CONSTRAINT uk_config_key UNIQUE (config_key)
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
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
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
  is_default SMALLINT NOT NULL DEFAULT 0,
  status SMALLINT NOT NULL DEFAULT 1,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  deleted_at TIMESTAMPTZ NULL,
  PRIMARY KEY (id)
);

CREATE TABLE sys_file (
  id BIGSERIAL,
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
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  deleted_at TIMESTAMPTZ NULL,
  PRIMARY KEY (id),
  CONSTRAINT uk_file_object UNIQUE (storage_id, object_key)
);

CREATE TABLE sys_file_reference (
  id BIGSERIAL,
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
  title VARCHAR(255) NOT NULL,
  notice_type VARCHAR(32) NOT NULL DEFAULT 'notice',
  priority VARCHAR(16) NOT NULL DEFAULT 'normal',
  content TEXT NOT NULL,
  target_type VARCHAR(32) NOT NULL DEFAULT 'all',
  target_ids JSONB NULL,
  status VARCHAR(32) NOT NULL DEFAULT 'draft',
  is_pinned SMALLINT NOT NULL DEFAULT 0,
  publish_at TIMESTAMPTZ NULL,
  published_by BIGINT NULL,
  revoked_at TIMESTAMPTZ NULL,
  created_by BIGINT NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  deleted_at TIMESTAMPTZ NULL,
  PRIMARY KEY (id)
);

CREATE TABLE sys_notice_read (
  notice_id BIGINT NOT NULL,
  user_id BIGINT NOT NULL,
  read_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  PRIMARY KEY (notice_id, user_id)
);

CREATE TABLE sys_refresh_token (
  id BIGSERIAL,
  user_id BIGINT NOT NULL,
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
  PRIMARY KEY (id),
  CONSTRAINT uk_refresh_token_hash UNIQUE (token_hash)
);

CREATE TABLE sys_api_token (
  id BIGSERIAL,
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
  is_suspicious SMALLINT NOT NULL DEFAULT 0,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  PRIMARY KEY (id)
);

CREATE TABLE sys_operation_log (
  id BIGSERIAL,
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
  PRIMARY KEY (id)
);

CREATE TABLE sys_error_log (
  id BIGSERIAL,
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
  PRIMARY KEY (id)
);

CREATE TABLE sys_webhook (
  id BIGSERIAL,
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
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  deleted_at TIMESTAMPTZ NULL,
  PRIMARY KEY (id)
);

CREATE TABLE sys_scheduled_job (
  id BIGSERIAL,
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
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  deleted_at TIMESTAMPTZ NULL,
  PRIMARY KEY (id)
);

CREATE TABLE sys_job_log (
  id BIGSERIAL,
  job_id BIGINT NOT NULL,
  trigger_type VARCHAR(32) NOT NULL,
  started_at TIMESTAMPTZ NOT NULL,
  finished_at TIMESTAMPTZ NULL,
  success SMALLINT NULL,
  attempt INT NOT NULL DEFAULT 1,
  output TEXT NULL,
  error_message TEXT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
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
CREATE UNIQUE INDEX uk_department_code ON sys_department (code) WHERE deleted_at IS NULL;

CREATE UNIQUE INDEX uk_position_code ON sys_position (code) WHERE deleted_at IS NULL;

CREATE UNIQUE INDEX uk_user_username ON sys_user (username) WHERE deleted_at IS NULL;

CREATE UNIQUE INDEX uk_role_code ON sys_role (code) WHERE deleted_at IS NULL;

CREATE UNIQUE INDEX uk_menu_code ON sys_menu (code) WHERE deleted_at IS NULL;

CREATE UNIQUE INDEX uk_dict_type_code ON sys_dict_type (code) WHERE deleted_at IS NULL;

CREATE UNIQUE INDEX uk_dict_item_value ON sys_dict_item (dict_type_id, value) WHERE deleted_at IS NULL;

CREATE UNIQUE INDEX uk_scheduled_job_code ON sys_scheduled_job (code) WHERE deleted_at IS NULL;

CREATE INDEX idx_department_parent ON sys_department (parent_id);

CREATE INDEX idx_department_deleted ON sys_department (deleted_at);

CREATE INDEX idx_position_deleted ON sys_position (deleted_at);

CREATE INDEX idx_user_department ON sys_user (department_id);

CREATE INDEX idx_user_mobile ON sys_user (mobile);

CREATE INDEX idx_user_email ON sys_user (email);

CREATE INDEX idx_user_deleted ON sys_user (deleted_at);

CREATE INDEX idx_user_position_position ON sys_user_position (position_id);

CREATE INDEX idx_role_deleted ON sys_role (deleted_at);

CREATE INDEX idx_user_role_role ON sys_user_role (role_id);

CREATE INDEX idx_role_department_department ON sys_role_department (department_id);

CREATE INDEX idx_menu_parent ON sys_menu (parent_id);

CREATE INDEX idx_menu_deleted ON sys_menu (deleted_at);

CREATE INDEX idx_role_menu_menu ON sys_role_menu (menu_id);

CREATE INDEX idx_dict_type_deleted ON sys_dict_type (deleted_at);

CREATE INDEX idx_dict_item_deleted ON sys_dict_item (deleted_at);

CREATE INDEX idx_config_group ON sys_config (config_group);

CREATE INDEX idx_auth_provider_type ON sys_auth_provider (provider_type);

CREATE INDEX idx_auth_provider_deleted ON sys_auth_provider (deleted_at);

CREATE INDEX idx_storage_default ON sys_storage_config (is_default, status);

CREATE INDEX idx_storage_deleted ON sys_storage_config (deleted_at);

CREATE INDEX idx_file_uploader ON sys_file (uploader_id);

CREATE INDEX idx_file_status ON sys_file (status);

CREATE INDEX idx_file_deleted ON sys_file (deleted_at);

CREATE INDEX idx_file_reference_business ON sys_file_reference (business_type, business_id);

CREATE INDEX idx_notice_status_publish ON sys_notice (status, publish_at);

CREATE INDEX idx_notice_deleted ON sys_notice (deleted_at);

CREATE INDEX idx_notice_read_user ON sys_notice_read (user_id, read_at);

CREATE INDEX idx_refresh_token_user ON sys_refresh_token (user_id, revoked_at, expires_at);

CREATE INDEX idx_refresh_token_family ON sys_refresh_token (family_id);

CREATE INDEX idx_api_token_user ON sys_api_token (user_id, status);

CREATE INDEX idx_login_log_user ON sys_login_log (user_id, created_at);

CREATE INDEX idx_login_log_username ON sys_login_log (username, created_at);

CREATE INDEX idx_login_log_success ON sys_login_log (success, created_at);

CREATE INDEX idx_operation_log_user ON sys_operation_log (user_id, created_at);

CREATE INDEX idx_operation_log_module ON sys_operation_log (module, created_at);

CREATE INDEX idx_operation_log_target ON sys_operation_log (target_type, target_id);

CREATE INDEX idx_operation_log_request ON sys_operation_log (request_id);

CREATE INDEX idx_error_log_created ON sys_error_log (created_at);

CREATE INDEX idx_error_log_resolved ON sys_error_log (resolved, created_at);

CREATE INDEX idx_error_log_request ON sys_error_log (request_id);

CREATE INDEX idx_config_log_key ON sys_config_log (config_type, config_key, created_at);

CREATE INDEX idx_webhook_deleted ON sys_webhook (deleted_at);

CREATE INDEX idx_scheduled_job_deleted ON sys_scheduled_job (deleted_at);

CREATE INDEX idx_job_log_job ON sys_job_log (job_id, started_at);

CREATE INDEX idx_job_log_success ON sys_job_log (success, started_at);

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
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
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
CREATE TABLE sys_translation (
  id BIGSERIAL,
  entity_key VARCHAR(191) NOT NULL,
  lang VARCHAR(16) NOT NULL,
  value TEXT NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  PRIMARY KEY (id),
  CONSTRAINT uk_translation_entity_lang UNIQUE (entity_key, lang)
);

CREATE INDEX idx_translation_lang ON sys_translation (lang);

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
    WHERE deleted_at IS NULL;

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
