-- 外部通知通道：通道配置、消息模板、发送日志。
--
-- 通道凭据（SMTP 密码、短信 AccessKey）用 app.Context.Secrets 加密后存
-- config_json，读时解密；数据库里不出现明文密钥。
--
-- tenant_id 由 app_fill_tenant_id 触发器从请求会话填入；三张表全部启用
-- FORCE ROW LEVEL SECURITY，与核心业务表同构。

CREATE TABLE IF NOT EXISTS notify_channel (
  id BIGSERIAL PRIMARY KEY,
  tenant_id BIGINT NOT NULL DEFAULT 1,
  channel_type VARCHAR(24) NOT NULL DEFAULT '',
  name VARCHAR(128) NOT NULL DEFAULT '',
  enabled SMALLINT NOT NULL DEFAULT 1,
  config_json TEXT NOT NULL DEFAULT '',
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  deleted_at TIMESTAMPTZ NULL
);

CREATE UNIQUE INDEX IF NOT EXISTS uk_notify_channel_name
  ON notify_channel (tenant_id, channel_type, name) WHERE deleted_at IS NULL;

CREATE TABLE IF NOT EXISTS notify_template (
  id BIGSERIAL PRIMARY KEY,
  tenant_id BIGINT NOT NULL DEFAULT 1,
  channel_type VARCHAR(24) NOT NULL DEFAULT '',
  code VARCHAR(64) NOT NULL DEFAULT '',
  name VARCHAR(128) NOT NULL DEFAULT '',
  subject VARCHAR(255) NOT NULL DEFAULT '',
  content TEXT NOT NULL DEFAULT '',
  status SMALLINT NOT NULL DEFAULT 1,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  deleted_at TIMESTAMPTZ NULL
);

CREATE UNIQUE INDEX IF NOT EXISTS uk_notify_template_code
  ON notify_template (tenant_id, channel_type, code) WHERE deleted_at IS NULL;

CREATE TABLE IF NOT EXISTS notify_log (
  id BIGSERIAL PRIMARY KEY,
  tenant_id BIGINT NOT NULL DEFAULT 1,
  channel_type VARCHAR(24) NOT NULL DEFAULT '',
  channel_name VARCHAR(128) NOT NULL DEFAULT '',
  template_code VARCHAR(64) NOT NULL DEFAULT '',
  recipient VARCHAR(255) NOT NULL DEFAULT '',
  subject VARCHAR(255) NOT NULL DEFAULT '',
  content TEXT NOT NULL DEFAULT '',
  status SMALLINT NOT NULL DEFAULT 0,
  error_msg TEXT NOT NULL DEFAULT '',
  sent_at TIMESTAMPTZ NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_notify_log_created ON notify_log (tenant_id, created_at DESC);

-- 推进序列：显式默认 1 不占用序列，若库里已有行需对齐。
SELECT setval(pg_get_serial_sequence('notify_channel', 'id'),
  GREATEST((SELECT COALESCE(MAX(id), 0) FROM notify_channel), 1));
SELECT setval(pg_get_serial_sequence('notify_template', 'id'),
  GREATEST((SELECT COALESCE(MAX(id), 0) FROM notify_template), 1));
SELECT setval(pg_get_serial_sequence('notify_log', 'id'),
  GREATEST((SELECT COALESCE(MAX(id), 0) FROM notify_log), 1));
