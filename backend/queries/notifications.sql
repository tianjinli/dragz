-- name: CreateNotification :one
INSERT INTO sys_notification (user_id, channel, title, content, biz_type, biz_id)
VALUES ($1, $2, $3, $4, $5, $6)
RETURNING id, user_id, channel, title, content, biz_type, biz_id, read_at, created_at;

-- name: ListMyNotifications :many
SELECT id, user_id, channel, title, content, biz_type, biz_id, read_at, created_at
FROM sys_notification
WHERE user_id = sqlc.arg(user_id)
  AND (NOT sqlc.arg(unread_only)::boolean OR read_at IS NULL)
  AND (sqlc.narg(channel)::text IS NULL OR channel = sqlc.narg(channel))
ORDER BY created_at DESC
LIMIT sqlc.arg(row_limit) OFFSET sqlc.arg(row_offset);

-- name: CountMyNotifications :one
SELECT COUNT(*) FROM sys_notification
WHERE user_id = sqlc.arg(user_id)
  AND (NOT sqlc.arg(unread_only)::boolean OR read_at IS NULL)
  AND (sqlc.narg(channel)::text IS NULL OR channel = sqlc.narg(channel));

-- name: MarkNotificationRead :execrows
UPDATE sys_notification
SET read_at = NOW()
WHERE id = sqlc.arg(id) AND user_id = sqlc.arg(user_id) AND read_at IS NULL;

-- name: MarkAllNotificationsRead :execrows
UPDATE sys_notification
SET read_at = NOW()
WHERE user_id = sqlc.arg(user_id) AND read_at IS NULL
  AND (sqlc.narg(channel)::text IS NULL OR channel = sqlc.narg(channel));

-- name: GetNotificationForUser :one
SELECT id, user_id, channel, title, content, biz_type, biz_id, read_at, created_at
FROM sys_notification
WHERE id = sqlc.arg(id) AND user_id = sqlc.arg(user_id);

-- name: IsNotifyChannelEnabled :one
-- Missing preference means the channel is on. enabled = 0 is the only off state.
SELECT COALESCE((
  SELECT (enabled = 1)
  FROM sys_user_notify_pref
  WHERE user_id = sqlc.arg(user_id) AND channel = sqlc.arg(channel)
), TRUE)::boolean AS enabled;

-- name: UpsertNotifyPref :exec
INSERT INTO sys_user_notify_pref (user_id, channel, enabled, updated_at)
VALUES (sqlc.arg(user_id), sqlc.arg(channel), sqlc.arg(enabled), NOW())
ON CONFLICT (user_id, channel) DO UPDATE
SET enabled = EXCLUDED.enabled, updated_at = NOW();

-- name: ListNotifyPrefsForUser :many
SELECT user_id, channel, enabled, updated_at
FROM sys_user_notify_pref
WHERE user_id = sqlc.arg(user_id)
ORDER BY channel;

-- name: CountUnreadNotifications :one
SELECT COUNT(*) FROM sys_notification
WHERE user_id = sqlc.arg(user_id) AND read_at IS NULL;

-- name: ListNoticeAudienceUserIDs :many
-- Inverse of NoticeAllowedForUser: the active users a published notice targets.
SELECT u.id
FROM sys_user u
WHERE u.deleted_at IS NULL AND u.status = 1
  AND EXISTS (
    SELECT 1 FROM sys_notice n
    WHERE n.id = sqlc.arg(notice_id)
      AND n.deleted_at IS NULL
      AND n.status = 'published'
      AND n.publish_at <= NOW()
      AND (
        n.target_type = 'all'
        OR (n.target_type = 'user' AND COALESCE(n.target_ids, '[]'::jsonb) @> jsonb_build_array(u.id))
        OR (n.target_type = 'department' AND u.department_id IS NOT NULL
            AND COALESCE(n.target_ids, '[]'::jsonb) @> jsonb_build_array(u.department_id))
        OR (n.target_type = 'role' AND EXISTS (
          SELECT 1 FROM sys_user_role ur
          WHERE ur.user_id = u.id
            AND COALESCE(n.target_ids, '[]'::jsonb) @> jsonb_build_array(ur.role_id)))
      )
  );
