-- name: CreatePendingFile :one
INSERT INTO sys_file
  (storage_id, original_name, object_key, extension, mime_type, size, access_type, status, uploader_id)
VALUES ($1, $2, $3, $4, $5, $6, $7, 'pending', $8)
RETURNING id;

-- name: CompleteFile :execrows
UPDATE sys_file SET status = 'ready', sha256 = $1, uploaded_at = NOW()
WHERE id = $2 AND status = 'pending';

-- name: FailFile :exec
UPDATE sys_file SET status = 'failed', purge_error = $1
WHERE id = $2 AND status = 'pending';

-- name: GetFileByID :one
SELECT f.id, f.storage_id, s.name AS storage_name, f.original_name,
  f.object_key, f.extension, f.mime_type, f.size, f.sha256, f.access_type, f.status,
  f.uploader_id, COALESCE(u.display_name, '') AS uploader_name, f.uploaded_at, f.recycle_at, f.purge_after,
  (SELECT COUNT(*) FROM sys_file_reference fr WHERE fr.file_id = f.id) AS reference_count,
  f.created_at
FROM sys_file f
INNER JOIN sys_storage_config s ON s.id = f.storage_id
LEFT JOIN sys_user u ON u.id = f.uploader_id
WHERE f.id = $1 AND f.deleted_at IS NULL;

-- name: RecycleFile :execrows
UPDATE sys_file SET status = 'recycled',
  recycle_at = NOW(), purge_after = NOW() + (sqlc.arg(days)::int * INTERVAL '1 day')
WHERE id = sqlc.arg(id) AND status = 'ready' AND deleted_at IS NULL;

-- name: RestoreFile :execrows
UPDATE sys_file SET status = 'ready', recycle_at = NULL, purge_after = NULL
WHERE id = $1 AND status = 'recycled' AND deleted_at IS NULL;

-- name: DeleteFileReferences :exec
DELETE FROM sys_file_reference WHERE file_id = $1;

-- name: PurgeFileRecord :execrows
UPDATE sys_file SET deleted_at = NOW(), status = 'purged'
WHERE id = $1 AND status IN ('recycled', 'failed') AND deleted_at IS NULL;

-- name: AddFileReference :one
INSERT INTO sys_file_reference (file_id, business_type, business_id, field_name, created_by)
SELECT sqlc.arg(file_id), sqlc.arg(business_type), sqlc.arg(business_id), sqlc.arg(field_name), sqlc.arg(created_by)
FROM sys_file f
WHERE f.id = sqlc.arg(file_id) AND f.status = 'ready' AND f.deleted_at IS NULL
RETURNING id;

-- name: DeleteFileReference :execrows
DELETE FROM sys_file_reference WHERE id = $1;

-- name: ListFileReferences :many
SELECT id, file_id, business_type, business_id, field_name, created_by, created_at
FROM sys_file_reference
WHERE file_id = $1
ORDER BY id;

-- name: CountFilesFiltered :one
SELECT COUNT(*)
FROM sys_file f
INNER JOIN sys_storage_config s ON s.id = f.storage_id
LEFT JOIN sys_user u ON u.id = f.uploader_id
WHERE f.deleted_at IS NULL
  AND (sqlc.narg('keyword_pattern')::text IS NULL OR (
    f.original_name LIKE sqlc.narg('keyword_pattern')
    OR f.sha256 LIKE sqlc.narg('keyword_pattern')
    OR u.username LIKE sqlc.narg('keyword_pattern')))
  AND (sqlc.narg('mime_pattern')::text IS NULL OR f.mime_type LIKE sqlc.narg('mime_pattern'))
  AND (sqlc.narg('status')::text IS NULL OR f.status = sqlc.narg('status'))
  AND (sqlc.narg('exclude_status')::text IS NULL OR f.status <> sqlc.narg('exclude_status'))
  AND (sqlc.narg('uploader_id')::bigint IS NULL OR f.uploader_id = sqlc.narg('uploader_id'));

-- name: ListFilesFiltered :many
SELECT f.id, f.storage_id, s.name AS storage_name, f.original_name,
  f.object_key, f.extension, f.mime_type, f.size, f.sha256, f.access_type, f.status,
  f.uploader_id, COALESCE(u.display_name, '') AS uploader_name, f.uploaded_at, f.recycle_at, f.purge_after,
  (SELECT COUNT(*) FROM sys_file_reference fr WHERE fr.file_id = f.id) AS reference_count,
  f.created_at
FROM sys_file f
INNER JOIN sys_storage_config s ON s.id = f.storage_id
LEFT JOIN sys_user u ON u.id = f.uploader_id
WHERE f.deleted_at IS NULL
  AND (sqlc.narg('keyword_pattern')::text IS NULL OR (
    f.original_name LIKE sqlc.narg('keyword_pattern')
    OR f.sha256 LIKE sqlc.narg('keyword_pattern')
    OR u.username LIKE sqlc.narg('keyword_pattern')))
  AND (sqlc.narg('mime_pattern')::text IS NULL OR f.mime_type LIKE sqlc.narg('mime_pattern'))
  AND (sqlc.narg('status')::text IS NULL OR f.status = sqlc.narg('status'))
  AND (sqlc.narg('exclude_status')::text IS NULL OR f.status <> sqlc.narg('exclude_status'))
  AND (sqlc.narg('uploader_id')::bigint IS NULL OR f.uploader_id = sqlc.narg('uploader_id'))
ORDER BY f.id DESC
LIMIT $1 OFFSET $2;
