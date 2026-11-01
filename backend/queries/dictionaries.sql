-- name: CreateDictionaryType :one
INSERT INTO sys_dict_type (code, name, display_type, status, remark)
VALUES ($1, $2, $3, $4, $5)
RETURNING id;

-- name: UpdateDictionaryType :execrows
UPDATE sys_dict_type SET code = $1, name = $2, display_type = $3, status = $4, remark = $5
WHERE id = $6 AND deleted_at IS NULL;

-- name: GetDictionaryTypeDeleteChecks :one
SELECT d.is_system,
  (SELECT COUNT(*) FROM sys_dict_item WHERE dict_type_id = d.id AND deleted_at IS NULL) AS item_count
FROM sys_dict_type d
WHERE d.id = $1 AND d.deleted_at IS NULL;

-- name: SoftDeleteDictionaryType :execrows
UPDATE sys_dict_type SET deleted_at = NOW(), status = 0
WHERE id = $1 AND deleted_at IS NULL;

-- name: ListDictionaryItems :many
SELECT id, dict_type_id, label, value, color, css_class, sort, status,
  is_default, remark, created_at
FROM sys_dict_item
WHERE dict_type_id = $1 AND deleted_at IS NULL
ORDER BY sort, id;

-- name: ListEnabledDictionaryItems :many
SELECT id, dict_type_id, label, value, color, css_class, sort, status,
  is_default, remark, created_at
FROM sys_dict_item
WHERE dict_type_id = $1 AND deleted_at IS NULL AND status = 1
ORDER BY sort, id;

-- name: GetDictionaryTypeIDByCode :one
SELECT id FROM sys_dict_type
WHERE code = $1 AND status = 1 AND deleted_at IS NULL;

-- name: CreateDictionaryItem :one
INSERT INTO sys_dict_item
  (dict_type_id, label, value, color, css_class, sort, status, is_default, remark)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
RETURNING id;

-- name: UpdateDictionaryItem :execrows
UPDATE sys_dict_item SET label = $1, value = $2, color = $3,
  css_class = $4, sort = $5, status = $6, is_default = $7, remark = $8
WHERE id = $9 AND deleted_at IS NULL;

-- name: SoftDeleteDictionaryItem :execrows
UPDATE sys_dict_item SET deleted_at = NOW(), status = 0
WHERE id = $1 AND deleted_at IS NULL;

-- name: ListDictionaryTypesFiltered :many
SELECT d.id, d.code, d.name, d.display_type, d.status, d.is_system,
  d.remark,
  (SELECT COUNT(*) FROM sys_dict_item i WHERE i.dict_type_id = d.id AND i.deleted_at IS NULL) AS item_count,
  d.created_at
FROM sys_dict_type d
WHERE d.deleted_at IS NULL
  AND (sqlc.narg('keyword_pattern')::text IS NULL OR (d.name LIKE sqlc.narg('keyword_pattern') OR d.code LIKE sqlc.narg('keyword_pattern')))
  AND (sqlc.narg('status')::smallint IS NULL OR d.status = sqlc.narg('status'))
ORDER BY d.id DESC;
