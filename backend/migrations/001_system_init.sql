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
  permission VARCHAR(128) NOT NULL DEFAULT '',
  sort INT NOT NULL DEFAULT 0,
  status SMALLINT NOT NULL DEFAULT 1,
  visible SMALLINT NOT NULL DEFAULT 1,
  keep_alive SMALLINT NOT NULL DEFAULT 0,
  affix_tab SMALLINT NOT NULL DEFAULT 0,
  external_url VARCHAR(500) NOT NULL DEFAULT '',
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

-- Business keys are unique among LIVE rows only: a soft-deleted row keeps its
-- code for the audit trail, and the code becomes free again for a new record.
-- See schema/tables.sql for the full explanation; store.bootstrapStatements
-- applies the same DDL to databases created before this change.
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

INSERT INTO sys_department (id, parent_id, ancestors, code, name, sort, status)
VALUES (1, 0, '0', 'root', '总部', 0, 1);

INSERT INTO sys_role (id, code, name, data_scope, sort, status, is_system, remark)
VALUES (1, 'super_admin', '超级管理员', 'all', 0, 1, 1, '系统内置角色，不允许删除');

-- Development bootstrap administrator. The password is Dragz!7xZ and must be
-- changed at first login. The bcrypt hash below (cost 10) is the only source
-- of truth for that password: Bootstrap.AdminPassword in etc/*.yaml is used
-- only when this row does not exist, which never happens on a fresh database.
INSERT INTO sys_user
  (department_id, username, password_hash, display_name, status, is_super_admin, must_change_password)
VALUES
  (1, 'admin', '$2a$10$GsG.n57BtMaOq9w2dqZPqOzZGbROiaqmwpX0MOLQPot9RKwMvQKZ.', 'Administrator', 1, 1, 1);

INSERT INTO sys_user_role (user_id, role_id)
SELECT id, 1 FROM sys_user WHERE username = 'admin';

INSERT INTO sys_config (config_group, config_key, name, value_type, config_value, default_value, description)
VALUES
  ('basic', 'system.name', 'config.label.system_name', 'string', 'Dragz Admin', 'Dragz Admin', 'config.desc.system_name'),
  ('security', 'security.captcha.enabled', 'config.label.security_captcha_enabled', 'boolean', 'false', 'false', 'config.desc.security_captcha_enabled'),
  ('security', 'security.login.maxAttempts', 'config.label.security_login_maxAttempts', 'number', '5', '5', 'config.desc.security_login_maxAttempts'),
  ('security', 'security.login.lockMinutes', 'config.label.security_login_lockMinutes', 'number', '30', '30', 'config.desc.security_login_lockMinutes'),
  ('security', 'security.password.minLength', 'config.label.security_password_minLength', 'number', '8', '8', 'config.desc.security_password_minLength'),
  ('feature', 'feature.apiToken.enabled', 'config.label.feature_apiToken_enabled', 'boolean', 'true', 'true', ''),
  ('feature', 'feature.notice.enabled', 'config.label.feature_notice_enabled', 'boolean', 'true', 'true', ''),
  ('feature', 'feature.fileUpload.enabled', 'config.label.feature_fileUpload_enabled', 'boolean', 'true', 'true', ''),
  ('feature', 'feature.scheduler.enabled', 'config.label.feature_scheduler_enabled', 'boolean', 'true', 'true', '');

INSERT INTO sys_storage_config
  (id, name, storage_type, base_path, private_access, max_file_size, is_default, status)
VALUES
  (1, '本地存储', 'local', './data/uploads', 1, 104857600, 1, 1);

INSERT INTO sys_scheduled_job
  (name, code, handler, cron_expression, payload, timeout_seconds, max_retries, status, next_run_at)
VALUES
  ('清理过期登录会话', 'maintenance.refresh_tokens', 'maintenance.cleanup_refresh_tokens',
   '15 3 * * *', '{"retentionDays": 7}'::jsonb, 120, 1, 1, NOW() + INTERVAL '1 day'),
  ('清理历史审计日志', 'maintenance.audit_logs', 'maintenance.cleanup_audit_logs',
   '30 3 * * 0', '{"retentionDays": 180}'::jsonb, 300, 1, 1, NOW() + INTERVAL '1 day'),
  ('清理上传失败记录', 'maintenance.failed_uploads', 'maintenance.cleanup_failed_uploads',
   '45 3 * * *', '{"retentionDays": 7}'::jsonb, 120, 1, 1, NOW() + INTERVAL '1 day'),
  ('清理存储测试文件', 'maintenance.storage_test_files', 'maintenance.cleanup_storage_test_files',
   '*/15 * * * *', '{"retentionHours": 1}'::jsonb, 300, 3, 1, NOW() + INTERVAL '1 hour'),
  ('清理孤儿文件', 'maintenance.orphan_files', 'maintenance.cleanup_orphan_files',
   '0 4 * * *', '{"graceDays": 7, "recycleDays": 30, "maxFiles": 1000}'::jsonb, 300, 1, 1, NOW() + INTERVAL '1 day');

INSERT INTO sys_menu
  (id, parent_id, type, name, code, route_name, route_path, component, icon, sort, status, visible, keep_alive, affix_tab, redirect)
VALUES
  (1, 0, 'page', '工作台', 'dashboard', 'Analytics', '/analytics', '/dashboard/analytics/index', 'lucide:layout-dashboard', 0, 1, 1, 1, 1, ''),
  (10, 0, 'directory', '组织权限', 'organization', 'Organization', '/organization', '', 'lucide:shield-check', 10, 1, 1, 0, 0, '/organization/users'),
  (11, 10, 'page', '用户管理', 'system_user', 'SystemUser', '/organization/users', '/system/user/index', 'lucide:users', 11, 1, 1, 1, 0, ''),
  (12, 10, 'page', '部门管理', 'system_department', 'SystemDepartment', '/organization/departments', '/system/department/index', 'lucide:network', 12, 1, 1, 1, 0, ''),
  (13, 10, 'page', '岗位管理', 'system_position', 'SystemPosition', '/organization/positions', '/system/position/index', 'lucide:briefcase-business', 13, 1, 1, 1, 0, ''),
  (14, 10, 'page', '角色管理', 'system_role', 'SystemRole', '/organization/roles', '/system/role/index', 'lucide:user-round-cog', 14, 1, 1, 1, 0, ''),
  (15, 10, 'page', '菜单管理', 'system_menu', 'SystemMenu', '/organization/menus', '/system/menu/index', 'lucide:list-tree', 15, 1, 1, 1, 0, ''),
  (20, 0, 'directory', '系统设置', 'settings', 'Settings', '/settings', '', 'lucide:settings', 20, 1, 1, 0, 0, '/settings/dictionaries'),
  (21, 20, 'page', '字典管理', 'system_dict', 'SystemDict', '/settings/dictionaries', '/system/dict/index', 'lucide:book-key', 21, 1, 1, 1, 0, ''),
  (22, 20, 'page', '参数配置', 'system_config', 'SystemConfig', '/settings/config', '/system/config/index', 'lucide:sliders-horizontal', 22, 1, 1, 1, 0, ''),
  (23, 20, 'page', '存储配置', 'system_storage', 'SystemStorage', '/settings/storage', '/system/storage/index', 'lucide:hard-drive-upload', 23, 1, 1, 1, 0, ''),
  (24, 20, 'page', '翻译管理', 'system_translation', 'SystemTranslation', '/settings/translations', '/system/translation/index', 'lucide:languages', 24, 1, 1, 1, 0, ''),
  (30, 0, 'directory', '消息中心', 'messages', 'Messages', '/messages', '', 'lucide:bell', 30, 1, 1, 0, 0, '/messages/notices'),
  (31, 30, 'page', '通知公告', 'system_notice', 'SystemNotice', '/messages/notices', '/system/notice/index', 'lucide:megaphone', 31, 1, 1, 1, 0, ''),
  (32, 30, 'page', '我的消息', 'my_messages', 'MyMessages', '/messages/mine', '/system/message/index', 'lucide:mail', 32, 1, 1, 1, 0, ''),
  (40, 0, 'directory', '文件中心', 'files', 'Files', '/files', '', 'lucide:folder-open', 40, 1, 1, 0, 0, '/files/list'),
  (41, 40, 'page', '文件管理', 'system_file', 'SystemFile', '/files/list', '/system/file/index', 'lucide:files', 41, 1, 1, 1, 0, ''),
  (42, 40, 'page', '回收站', 'system_file_recycle', 'SystemFileRecycle', '/files/recycle', '/system/file/recycle', 'lucide:trash-2', 42, 1, 1, 1, 0, ''),
  (50, 0, 'directory', '安全审计', 'security', 'Security', '/security', '', 'lucide:shield', 50, 1, 1, 0, 0, '/security/online-users'),
  (51, 50, 'page', '在线用户', 'system_online', 'SystemOnline', '/security/online-users', '/system/security/online-user', 'lucide:monitor-smartphone', 51, 1, 1, 1, 0, ''),
  (52, 50, 'page', '令牌管理', 'system_token', 'SystemToken', '/security/tokens', '/system/security/token', 'lucide:key-round', 52, 1, 1, 1, 0, ''),
  (53, 50, 'page', '登录日志', 'system_login_log', 'SystemLoginLog', '/security/login-logs', '/system/log/login', 'lucide:log-in', 53, 1, 1, 1, 0, ''),
  (54, 50, 'page', '操作日志', 'system_operation_log', 'SystemOperationLog', '/security/operation-logs', '/system/log/operation', 'lucide:history', 54, 1, 1, 1, 0, ''),
  (55, 50, 'page', '异常日志', 'system_error_log', 'SystemErrorLog', '/security/error-logs', '/system/log/error', 'lucide:triangle-alert', 55, 1, 1, 1, 0, ''),
  (60, 0, 'directory', '运维工具', 'tools', 'Tools', '/tools', '', 'lucide:wrench', 60, 1, 1, 0, 0, '/tools/jobs'),
  (61, 60, 'page', '定时任务', 'system_job', 'SystemJob', '/tools/jobs', '/system/job/index', 'lucide:clock-3', 61, 1, 1, 1, 0, ''),
  (62, 60, 'page', '网络钩子', 'system_webhook', 'SystemWebhook', '/tools/webhooks', '/system/webhook/index', 'lucide:webhook', 62, 1, 1, 1, 0, ''),
  (63, 60, 'page', '缓存管理', 'system_cache', 'SystemCache', '/tools/cache', '/system/cache/index', 'lucide:database-zap', 63, 1, 1, 1, 0, '');

-- Grant every menu to the built-in super_admin role. Permission buttons are
-- created and granted by the startup sync (see store.SyncPermissionMenus).
INSERT INTO sys_role_menu (role_id, menu_id)
SELECT 1, id FROM sys_menu WHERE deleted_at IS NULL;

-- English translations for seeded preset rows. Chinese (the default language)
-- lives in the owning tables; button translations are written by the startup
-- sync via menuperm.NameEn, so they are not seeded here.
INSERT INTO sys_translation (entity_key, lang, value) VALUES
  ('menu.dashboard', 'en', 'Workbench'),
  ('menu.organization', 'en', 'Organization'),
  ('menu.system_user', 'en', 'Users'),
  ('menu.system_department', 'en', 'Departments'),
  ('menu.system_position', 'en', 'Positions'),
  ('menu.system_role', 'en', 'Roles'),
  ('menu.system_menu', 'en', 'Menus'),
  ('menu.settings', 'en', 'Settings'),
  ('menu.system_dict', 'en', 'Dictionaries'),
  ('menu.system_config', 'en', 'Config Parameters'),
  ('menu.system_storage', 'en', 'Storage'),
  ('menu.system_plugin', 'en', 'Plugin Center'),
  ('menu.messages', 'en', 'Messages'),
  ('menu.system_notice', 'en', 'Announcements'),
  ('menu.my_messages', 'en', 'My Messages'),
  ('menu.files', 'en', 'Files'),
  ('menu.system_file', 'en', 'File Management'),
  ('menu.system_file_recycle', 'en', 'Recycle Bin'),
  ('menu.security', 'en', 'Security'),
  ('menu.system_online', 'en', 'Online Users'),
  ('menu.system_token', 'en', 'API Tokens'),
  ('menu.system_login_log', 'en', 'Login Logs'),
  ('menu.system_operation_log', 'en', 'Operation Logs'),
  ('menu.system_error_log', 'en', 'Error Logs'),
  ('menu.tools', 'en', 'Operations Tools'),
  ('menu.system_job', 'en', 'Scheduled Jobs'),
  ('menu.system_webhook', 'en', 'Webhooks'),
  ('menu.system_cache', 'en', 'Cache'),
  ('role.super_admin', 'en', 'Super Administrator'),
  ('role.super_admin.remark', 'en', 'Built-in role and cannot be deleted'),
  ('dept.root', 'en', 'Headquarters'),
  ('job.maintenance.refresh_tokens', 'en', 'Clean up expired login sessions'),
  ('job.maintenance.audit_logs', 'en', 'Clean up historical audit logs'),
  ('job.maintenance.failed_uploads', 'en', 'Clean up failed upload records'),
  ('job.maintenance.storage_test_files', 'en', 'Clean up storage test files'),
  ('job.maintenance.orphan_files', 'en', 'Clean up orphan files'),
  ('storage.local', 'en', 'Local Storage'),
  ('menu.system_translation', 'en', 'Translations');
