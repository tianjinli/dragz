-- name: GetNoticeDetail :one
SELECT n.id, n.title, n.notice_type, n.priority, n.content,
  n.target_type, COALESCE(n.target_ids, '[]'::jsonb) AS target_ids, n.status, n.is_pinned,
  n.publish_at, n.published_by, n.created_by, COALESCE(u.display_name, '') AS creator_name,
  EXISTS(SELECT 1 FROM sys_notice_read nr WHERE nr.notice_id = n.id AND nr.user_id = $1) AS is_read,
  (SELECT COUNT(*) FROM sys_notice_read nr WHERE nr.notice_id = n.id) AS read_count,
  n.created_at, n.updated_at
FROM sys_notice n
LEFT JOIN sys_user u ON u.id = n.created_by
WHERE n.id = $2 AND n.deleted_at IS NULL;

-- name: CreateNotice :one
INSERT INTO sys_notice
  (title, notice_type, priority, content, target_type, target_ids, status,
   is_pinned, publish_at, created_by)
VALUES ($1, $2, $3, $4, $5, $6, 'draft', $7, $8, $9)
RETURNING id;

-- name: UpdateNoticeDraft :execrows
UPDATE sys_notice SET title = $1, notice_type = $2,
  priority = $3, content = $4, target_type = $5, target_ids = $6, is_pinned = $7, publish_at = $8
WHERE id = $9 AND deleted_at IS NULL AND status = 'draft';

-- name: PublishNotice :execrows
UPDATE sys_notice SET status = 'published',
  publish_at = COALESCE(publish_at, NOW()), published_by = $1, revoked_at = NULL
WHERE id = $2 AND deleted_at IS NULL AND status = 'draft';

-- name: RevokeNotice :execrows
UPDATE sys_notice SET status = 'revoked', revoked_at = NOW()
WHERE id = $1 AND deleted_at IS NULL AND status = 'published';

-- name: SoftDeleteNotice :execrows
UPDATE sys_notice SET deleted_at = NOW()
WHERE id = $1 AND deleted_at IS NULL AND status IN ('draft', 'revoked');

-- name: MarkNoticeRead :exec
INSERT INTO sys_notice_read (notice_id, user_id)
VALUES ($1, $2)
ON CONFLICT (notice_id, user_id) DO NOTHING;

-- name: NoticeAllowedForUser :one
SELECT EXISTS(
  SELECT 1 FROM sys_notice n WHERE n.id = sqlc.arg(notice_id) AND n.deleted_at IS NULL
    AND n.status = 'published' AND n.publish_at <= NOW()
    AND (n.target_type = 'all'
      OR (n.target_type = 'user' AND COALESCE(n.target_ids, '[]'::jsonb) @> jsonb_build_array(sqlc.arg(user_id)::bigint))
      OR (n.target_type = 'department' AND EXISTS(
        SELECT 1 FROM sys_user u WHERE u.id = sqlc.arg(user_id) AND COALESCE(n.target_ids, '[]'::jsonb) @> jsonb_build_array(u.department_id)))
      OR (n.target_type = 'role' AND EXISTS(
        SELECT 1 FROM sys_user_role ur WHERE ur.user_id = sqlc.arg(user_id) AND COALESCE(n.target_ids, '[]'::jsonb) @> jsonb_build_array(ur.role_id))))
);

-- name: CountNoticesFiltered :one
SELECT COUNT(*) FROM sys_notice n
WHERE n.deleted_at IS NULL
  AND (sqlc.narg('keyword_pattern')::text IS NULL OR n.title LIKE sqlc.narg('keyword_pattern'))
  AND (sqlc.narg('status')::text IS NULL OR n.status = sqlc.narg('status'));

-- name: ListNoticesFiltered :many
SELECT n.id, n.title, n.notice_type, n.priority,
  n.target_type, COALESCE(n.target_ids, '[]'::jsonb) AS target_ids, n.status, n.is_pinned,
  n.publish_at, n.published_by, n.created_by, COALESCE(u.display_name, '') AS creator_name,
  (SELECT COUNT(*) FROM sys_notice_read nr WHERE nr.notice_id = n.id) AS read_count,
  n.created_at, n.updated_at
FROM sys_notice n
LEFT JOIN sys_user u ON u.id = n.created_by
WHERE n.deleted_at IS NULL
  AND (sqlc.narg('keyword_pattern')::text IS NULL OR n.title LIKE sqlc.narg('keyword_pattern'))
  AND (sqlc.narg('status')::text IS NULL OR n.status = sqlc.narg('status'))
ORDER BY n.is_pinned DESC, n.created_at DESC
LIMIT $1 OFFSET $2;

-- name: CountMyNotices :one
SELECT COUNT(*) FROM sys_notice n
WHERE n.deleted_at IS NULL AND n.status = 'published' AND n.publish_at <= NOW()
  AND (n.target_type = 'all'
    OR (n.target_type = 'user' AND COALESCE(n.target_ids, '[]'::jsonb) @> jsonb_build_array(sqlc.arg(user_id)::bigint))
    OR (n.target_type = 'department' AND EXISTS(
      SELECT 1 FROM sys_user u WHERE u.id = sqlc.arg(user_id) AND COALESCE(n.target_ids, '[]'::jsonb) @> jsonb_build_array(u.department_id)))
    OR (n.target_type = 'role' AND EXISTS(
      SELECT 1 FROM sys_user_role ur WHERE ur.user_id = sqlc.arg(user_id) AND COALESCE(n.target_ids, '[]'::jsonb) @> jsonb_build_array(ur.role_id))))
  AND (NOT sqlc.arg('unread_only')::boolean OR NOT EXISTS (
    SELECT 1 FROM sys_notice_read nr WHERE nr.notice_id = n.id AND nr.user_id = sqlc.arg(user_id)));

-- name: ListMyNotices :many
SELECT n.id, n.title, n.notice_type, n.priority,
  n.target_type, COALESCE(n.target_ids, '[]'::jsonb) AS target_ids, n.status, n.is_pinned,
  n.publish_at, n.published_by, n.created_by, COALESCE(u.display_name, '') AS creator_name,
  EXISTS(SELECT 1 FROM sys_notice_read nr WHERE nr.notice_id = n.id AND nr.user_id = sqlc.arg(user_id)) AS is_read,
  n.created_at, n.updated_at
FROM sys_notice n
LEFT JOIN sys_user u ON u.id = n.created_by
WHERE n.deleted_at IS NULL AND n.status = 'published' AND n.publish_at <= NOW()
  AND (n.target_type = 'all'
    OR (n.target_type = 'user' AND COALESCE(n.target_ids, '[]'::jsonb) @> jsonb_build_array(sqlc.arg(user_id)::bigint))
    OR (n.target_type = 'department' AND EXISTS(
      SELECT 1 FROM sys_user u2 WHERE u2.id = sqlc.arg(user_id) AND COALESCE(n.target_ids, '[]'::jsonb) @> jsonb_build_array(u2.department_id)))
    OR (n.target_type = 'role' AND EXISTS(
      SELECT 1 FROM sys_user_role ur WHERE ur.user_id = sqlc.arg(user_id) AND COALESCE(n.target_ids, '[]'::jsonb) @> jsonb_build_array(ur.role_id))))
  AND (NOT sqlc.arg('unread_only')::boolean OR NOT EXISTS (
    SELECT 1 FROM sys_notice_read nr WHERE nr.notice_id = n.id AND nr.user_id = sqlc.arg(user_id)))
ORDER BY n.is_pinned DESC, n.publish_at DESC
LIMIT $1 OFFSET $2;
