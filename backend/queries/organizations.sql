-- name: CreateDepartment :one
INSERT INTO sys_department
  (parent_id, ancestors, code, name, leader_user_id, sort, status)
VALUES ($1, $2, $3, $4, $5, $6, $7)
RETURNING id;

-- name: GetDepartmentParentInfo :one
SELECT parent_id, ancestors FROM sys_department
WHERE id = $1 AND deleted_at IS NULL;

-- name: UpdateDepartment :execrows
UPDATE sys_department SET parent_id = $1, ancestors = $2, code = $3,
  name = $4, leader_user_id = $5, sort = $6, status = $7
WHERE id = $8 AND deleted_at IS NULL;

-- name: UpdateDepartmentStatus :execrows
UPDATE sys_department SET status = $1
WHERE id = $2 AND deleted_at IS NULL;

-- name: RewriteDepartmentAncestorsPrefix :exec
UPDATE sys_department
SET ancestors = CONCAT($1::text, SUBSTRING(ancestors, LENGTH($2::text) + 1))
WHERE deleted_at IS NULL AND (ancestors = $3 OR ancestors LIKE CONCAT($4::text, ',%'));

-- name: DisableDepartmentDescendants :exec
UPDATE sys_department SET status = 0
WHERE deleted_at IS NULL AND (ancestors = $1 OR ancestors LIKE CONCAT($2::text, ',%'));

-- name: CountDepartmentChildrenAndUsers :one
SELECT
  (SELECT COUNT(*) FROM sys_department WHERE parent_id = $1 AND deleted_at IS NULL) AS child_count,
  (SELECT COUNT(*) FROM sys_user WHERE department_id = $2 AND deleted_at IS NULL) AS user_count;

-- name: SoftDeleteDepartment :execrows
UPDATE sys_department SET deleted_at = NOW(), status = 0
WHERE id = $1 AND deleted_at IS NULL;

-- name: GetDepartmentAncestorsStatus :one
SELECT ancestors, status FROM sys_department
WHERE id = $1 AND deleted_at IS NULL;

-- name: ListDepartmentNames :many
SELECT id, name FROM sys_department WHERE deleted_at IS NULL;

-- name: ListDepartmentsFiltered :many
SELECT d.id, d.parent_id, d.ancestors, d.code, d.name,
  d.leader_user_id, COALESCE(u.display_name, '') AS leader_name, d.sort, d.status, d.created_at
FROM sys_department d
LEFT JOIN sys_user u ON u.id = d.leader_user_id AND u.deleted_at IS NULL
WHERE d.deleted_at IS NULL
  AND (sqlc.narg('keyword_pattern')::text IS NULL OR (d.name LIKE sqlc.narg('keyword_pattern') OR d.code LIKE sqlc.narg('keyword_pattern')))
  AND (sqlc.narg('status')::smallint IS NULL OR d.status = sqlc.narg('status'))
ORDER BY d.ancestors, d.sort, d.id;

-- name: CreatePosition :one
INSERT INTO sys_position (code, name, sort, status, remark)
VALUES ($1, $2, $3, $4, $5)
RETURNING id;

-- name: UpdatePosition :execrows
UPDATE sys_position SET code = $1, name = $2, sort = $3,
  status = $4, remark = $5
WHERE id = $6 AND deleted_at IS NULL;

-- name: UpdatePositionStatus :execrows
UPDATE sys_position SET status = $1
WHERE id = $2 AND deleted_at IS NULL;

-- name: CountPositionUsers :one
SELECT COUNT(*) FROM sys_user_position up
INNER JOIN sys_user u ON u.id = up.user_id
WHERE up.position_id = $1 AND u.deleted_at IS NULL;

-- name: SoftDeletePosition :execrows
UPDATE sys_position SET deleted_at = NOW(), status = 0
WHERE id = $1 AND deleted_at IS NULL;

-- name: CountPositionsFiltered :one
SELECT COUNT(*) FROM sys_position p
WHERE p.deleted_at IS NULL
  AND (sqlc.narg('keyword_pattern')::text IS NULL OR (p.name LIKE sqlc.narg('keyword_pattern') OR p.code LIKE sqlc.narg('keyword_pattern')))
  AND (sqlc.narg('status')::smallint IS NULL OR p.status = sqlc.narg('status'));

-- name: ListPositionsFiltered :many
SELECT p.id, p.code, p.name, p.sort, p.status, p.remark,
  (SELECT COUNT(*) FROM sys_user_position up INNER JOIN sys_user u ON u.id = up.user_id
   WHERE up.position_id = p.id AND u.deleted_at IS NULL) AS user_count,
  p.created_at
FROM sys_position p
WHERE p.deleted_at IS NULL
  AND (sqlc.narg('keyword_pattern')::text IS NULL OR (p.name LIKE sqlc.narg('keyword_pattern') OR p.code LIKE sqlc.narg('keyword_pattern')))
  AND (sqlc.narg('status')::smallint IS NULL OR p.status = sqlc.narg('status'))
ORDER BY p.sort, p.id
LIMIT $1 OFFSET $2;
