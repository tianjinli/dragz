-- name: GetUserByID :one
SELECT id, department_id, username, password_hash, display_name, email, mobile, status,
  is_super_admin, must_change_password, failed_login_count, locked_until, token_version
FROM sys_user
WHERE id = $1 AND deleted_at IS NULL
LIMIT 1;

-- name: GetUserByUsername :one
SELECT id, department_id, username, password_hash, display_name, email, mobile, status,
  is_super_admin, must_change_password, failed_login_count, locked_until, token_version
FROM sys_user
WHERE username = $1 AND deleted_at IS NULL
LIMIT 1;

-- name: CountActiveUsers :one
SELECT COUNT(*) FROM sys_user WHERE deleted_at IS NULL;

-- name: CreateBootstrapAdmin :one
INSERT INTO sys_user
  (department_id, username, password_hash, display_name, status, is_super_admin, must_change_password)
VALUES (1, $1, $2, 'Administrator', 1, 1, 1)
RETURNING id;

-- name: InsertUserRole :exec
INSERT INTO sys_user_role (user_id, role_id) VALUES ($1, $2);

-- name: UpdateSelfProfile :execrows
UPDATE sys_user
SET display_name = $1, email = $2, mobile = $3, updated_at = NOW()
WHERE id = $4 AND deleted_at IS NULL;

-- name: ChangeSelfPassword :execrows
UPDATE sys_user
SET password_hash = $1, must_change_password = 0, password_changed_at = NOW(),
  failed_login_count = 0, locked_until = NULL, token_version = token_version + 1, updated_at = NOW()
WHERE id = $2 AND deleted_at IS NULL;

-- name: IncrementFailedLogin :exec
UPDATE sys_user SET failed_login_count = failed_login_count + 1
WHERE username = $1 AND deleted_at IS NULL;

-- name: IncrementFailedLoginWithLock :exec
UPDATE sys_user SET failed_login_count = failed_login_count + 1, locked_until = $1
WHERE username = $2 AND deleted_at IS NULL;

-- name: RecordLoginSuccessUser :exec
UPDATE sys_user SET failed_login_count = 0, locked_until = NULL,
  last_login_at = NOW(), last_login_ip = $1 WHERE id = $2;

-- name: InsertLoginLogFailure :exec
INSERT INTO sys_login_log (username, login_method, success, failure_reason, ip, user_agent)
VALUES ($1, 'password', 0, $2, $3, $4);

-- name: InsertLoginLogSuccess :exec
INSERT INTO sys_login_log (user_id, username, login_method, success, ip, user_agent)
VALUES ($1, $2, 'password', 1, $3, $4);

-- name: InsertLoginLogOAuth :exec
INSERT INTO sys_login_log (user_id, username, login_method, success, ip, user_agent)
VALUES ($1, $2, $3, 1, $4, $5);

-- name: GetUserDetail :one
SELECT u.id, u.department_id, COALESCE(d.name, '') AS department_name,
  COALESCE(d.code, '') AS department_code, u.username,
  u.display_name, u.email, u.mobile, u.gender, u.status, u.is_super_admin,
  u.must_change_password, u.last_login_at, u.last_login_ip, u.created_at, u.remark
FROM sys_user u
LEFT JOIN sys_department d ON d.id = u.department_id AND d.deleted_at IS NULL
WHERE u.id = $1 AND u.deleted_at IS NULL;

-- name: ListUserRoleIDs :many
SELECT role_id FROM sys_user_role WHERE user_id = $1 ORDER BY role_id;

-- name: ListUserPositionIDs :many
SELECT position_id FROM sys_user_position WHERE user_id = $1 ORDER BY position_id;

-- name: CreateUser :one
INSERT INTO sys_user
  (department_id, username, password_hash, display_name, email, mobile, gender, status,
   must_change_password, remark)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
RETURNING id;

-- name: UpdateUser :execrows
UPDATE sys_user SET department_id = $1, display_name = $2,
  email = $3, mobile = $4, gender = $5, status = $6, must_change_password = $7, remark = $8
WHERE id = $9 AND deleted_at IS NULL AND is_super_admin = 0;

-- name: UpdateUserStatus :execrows
UPDATE sys_user SET status = $1, updated_at = NOW()
WHERE id = $2 AND deleted_at IS NULL AND is_super_admin = 0;

-- name: SoftDeleteUser :execrows
UPDATE sys_user SET deleted_at = NOW(), status = 0
WHERE id = $1 AND deleted_at IS NULL AND is_super_admin = 0;

-- name: ResetUserPassword :execrows
UPDATE sys_user SET password_hash = $1, must_change_password = 1,
  password_changed_at = NOW(), failed_login_count = 0, locked_until = NULL,
  token_version = token_version + 1
WHERE id = $2 AND deleted_at IS NULL AND is_super_admin = 0;

-- name: BumpUserTokenVersion :exec
UPDATE sys_user SET token_version = token_version + 1, updated_at = NOW()
WHERE id = $1 AND deleted_at IS NULL;

-- name: DeleteUserRoles :exec
DELETE FROM sys_user_role WHERE user_id = $1;

-- name: DeleteUserPositions :exec
DELETE FROM sys_user_position WHERE user_id = $1;

-- name: InsertUserPosition :exec
INSERT INTO sys_user_position (user_id, position_id) VALUES ($1, $2);

-- name: GetUserDepartmentID :one
SELECT department_id FROM sys_user WHERE id = $1 AND deleted_at IS NULL;

-- name: GetUserScopeBasics :one
SELECT department_id, is_super_admin FROM sys_user
WHERE id = $1 AND status = 1 AND deleted_at IS NULL;

-- name: FindUserIDByEmail :one
SELECT id FROM sys_user WHERE email = $1 AND status = 1 AND deleted_at IS NULL LIMIT 1;

-- name: FindUserIDByMobile :one
SELECT id FROM sys_user WHERE mobile = $1 AND status = 1 AND deleted_at IS NULL LIMIT 1;

-- name: CreateOAuthUser :one
INSERT INTO sys_user
  (department_id, username, password_hash, display_name, email, mobile, status,
   must_change_password, password_changed_at)
VALUES (1, $1, $2, $3, $4, $5, 1, 0, NOW())
RETURNING id;

-- name: InsertUserRoleIfActive :exec
INSERT INTO sys_user_role (user_id, role_id)
SELECT $1, id FROM sys_role WHERE id = $2 AND status = 1 AND deleted_at IS NULL;

-- name: ClearLoginState :exec
UPDATE sys_user SET failed_login_count = 0, locked_until = NULL,
  last_login_at = NOW(), last_login_ip = $1 WHERE id = $2;

-- name: CountUsersFiltered :one
SELECT COUNT(*) FROM sys_user u
WHERE u.deleted_at IS NULL
  AND (sqlc.narg('keyword_pattern')::text IS NULL OR (
    u.username LIKE sqlc.narg('keyword_pattern')
    OR u.display_name LIKE sqlc.narg('keyword_pattern')
    OR u.mobile LIKE sqlc.narg('keyword_pattern')))
  AND (sqlc.narg('department_id')::bigint IS NULL OR u.department_id = sqlc.narg('department_id'))
  AND (sqlc.narg('status')::smallint IS NULL OR u.status = sqlc.narg('status'))
  AND (
    sqlc.arg('scope_all')::boolean
    OR (sqlc.narg('self_user_id')::bigint IS NOT NULL AND u.id = sqlc.narg('self_user_id'))
    OR (cardinality(COALESCE(sqlc.narg('department_ids')::bigint[], '{}'::bigint[])) > 0
        AND u.department_id = ANY(COALESCE(sqlc.narg('department_ids')::bigint[], '{}'::bigint[])))
  );

-- name: ListUsersFiltered :many
-- The department, role and position names are preset data, so they travel with
-- the codes their translation records are keyed by ("dept.<code>",
-- "role.<code>", "post.<code>"): the handler rebuilds the display strings in
-- the request language and leaves the aggregated names above as the source
-- fallback. The user's own columns (username, display name, email, mobile) are
-- never translated — they are facts the administrator typed, not preset text.
SELECT u.id, u.department_id, COALESCE(d.name, '') AS department_name,
  COALESCE(d.code, '') AS department_code, u.username, u.display_name,
  u.email, u.mobile, u.gender, u.status, u.is_super_admin, u.must_change_password,
  COALESCE((SELECT string_agg(r.name, ', ' ORDER BY r.sort)
    FROM sys_user_role ur INNER JOIN sys_role r ON r.id = ur.role_id
    WHERE ur.user_id = u.id AND r.deleted_at IS NULL), '')::text AS role_names,
  COALESCE((SELECT string_agg(p.name, ', ' ORDER BY p.sort)
    FROM sys_user_position up INNER JOIN sys_position p ON p.id = up.position_id
    WHERE up.user_id = u.id AND p.deleted_at IS NULL), '')::text AS position_names,
  COALESCE((SELECT json_agg(json_build_object('code', r.code, 'name', r.name) ORDER BY r.sort)::text
    FROM sys_user_role ur INNER JOIN sys_role r ON r.id = ur.role_id
    WHERE ur.user_id = u.id AND r.deleted_at IS NULL), '[]')::text AS roles,
  COALESCE((SELECT json_agg(json_build_object('code', p.code, 'name', p.name) ORDER BY p.sort)::text
    FROM sys_user_position up INNER JOIN sys_position p ON p.id = up.position_id
    WHERE up.user_id = u.id AND p.deleted_at IS NULL), '[]')::text AS positions,
  u.last_login_at, u.last_login_ip, u.created_at
FROM sys_user u
LEFT JOIN sys_department d ON d.id = u.department_id AND d.deleted_at IS NULL
WHERE u.deleted_at IS NULL
  AND (sqlc.narg('keyword_pattern')::text IS NULL OR (
    u.username LIKE sqlc.narg('keyword_pattern')
    OR u.display_name LIKE sqlc.narg('keyword_pattern')
    OR u.mobile LIKE sqlc.narg('keyword_pattern')))
  AND (sqlc.narg('department_id')::bigint IS NULL OR u.department_id = sqlc.narg('department_id'))
  AND (sqlc.narg('status')::smallint IS NULL OR u.status = sqlc.narg('status'))
  AND (
    sqlc.arg('scope_all')::boolean
    OR (sqlc.narg('self_user_id')::bigint IS NOT NULL AND u.id = sqlc.narg('self_user_id'))
    OR (cardinality(COALESCE(sqlc.narg('department_ids')::bigint[], '{}'::bigint[])) > 0
        AND u.department_id = ANY(COALESCE(sqlc.narg('department_ids')::bigint[], '{}'::bigint[])))
  )
ORDER BY u.id DESC
LIMIT $1 OFFSET $2;
