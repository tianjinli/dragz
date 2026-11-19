-- Track whether a published notice has already been pushed to its audience.
-- Scheduled notices keep status=published with a future publish_at; delivery
-- happens when publish_at becomes due and notified_at is still null.
BEGIN;

ALTER TABLE sys_notice
  ADD COLUMN IF NOT EXISTS notified_at TIMESTAMPTZ NULL;

-- Already-visible published notices must not be re-pushed by the due dispatcher.
UPDATE sys_notice
SET notified_at = COALESCE(publish_at, updated_at, created_at)
WHERE deleted_at IS NULL
  AND status = 'published'
  AND notified_at IS NULL
  AND (publish_at IS NULL OR publish_at <= NOW());

CREATE INDEX IF NOT EXISTS idx_notice_due_notify
  ON sys_notice (publish_at)
  WHERE deleted_at IS NULL AND status = 'published' AND notified_at IS NULL;

INSERT INTO sys_scheduled_job
  (name, code, handler, cron_expression, payload, timeout_seconds, max_retries, status, next_run_at)
SELECT
  '推送到期通知公告', 'notice.notify_due', 'notice.notify_due',
  '* * * * *', '{}'::jsonb, 60, 1, 1, NOW() + INTERVAL '1 minute'
WHERE NOT EXISTS (
  SELECT 1 FROM sys_scheduled_job
  WHERE code = 'notice.notify_due' AND deleted_at IS NULL
);

COMMIT;
