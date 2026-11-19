-- Revoke window for published notices, in minutes from publish_at.
-- 0 means revoking is never time-limited.
BEGIN;

INSERT INTO sys_config (config_group, config_key, name, value_type, config_value, default_value, description)
VALUES
  ('notice', 'notice.revokeWindowMinutes', 'config.label.notice_revokeWindowMinutes', 'number', '5', '5', 'config.desc.notice_revokeWindowMinutes')
ON CONFLICT (config_key) DO NOTHING;

COMMIT;