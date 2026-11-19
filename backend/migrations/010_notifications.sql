-- In-app notification inbox and per-user channel preferences.
-- Fresh databases already have these tables from 001_system_init.sql.
-- This script is the upgrade path for databases created before that change.
BEGIN;

CREATE TABLE IF NOT EXISTS sys_notification (
  id BIGSERIAL,
  user_id BIGINT NOT NULL,
  channel VARCHAR(32) NOT NULL,
  title VARCHAR(255) NOT NULL,
  content TEXT NOT NULL DEFAULT '',
  biz_type VARCHAR(64) NOT NULL DEFAULT '',
  biz_id VARCHAR(64) NOT NULL DEFAULT '',
  read_at TIMESTAMPTZ NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  PRIMARY KEY (id)
);

CREATE INDEX IF NOT EXISTS idx_notification_user_created ON sys_notification (user_id, created_at DESC);

CREATE TABLE IF NOT EXISTS sys_user_notify_pref (
  user_id BIGINT NOT NULL,
  channel VARCHAR(32) NOT NULL,
  enabled SMALLINT NOT NULL DEFAULT 1,
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  PRIMARY KEY (user_id, channel)
);

COMMIT;
