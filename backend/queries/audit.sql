-- name: CreateAPIToken :one
INSERT INTO sys_api_token
  (user_id, name, token_prefix, token_hash, scopes, ip_allowlist, expires_at)
VALUES ($1, $2, $3, $4, $5, $6, $7)
RETURNING id;

-- name: RevokeAPIToken :execrows
UPDATE sys_api_token SET status = 0, revoked_at = COALESCE(revoked_at, NOW())
WHERE id = $1;

-- name: RevokeAPITokenOwned :execrows
UPDATE sys_api_token SET status = 0, revoked_at = COALESCE(revoked_at, NOW())
WHERE id = $1 AND user_id = $2;

-- name: AuthenticateAPIToken :one
SELECT t.id, t.user_id, u.username, t.scopes,
  COALESCE(t.ip_allowlist, '[]'::jsonb) AS ip_allowlist, t.expires_at
FROM sys_api_token t
INNER JOIN sys_user u ON u.id = t.user_id
WHERE t.token_hash = $1 AND t.status = 1 AND t.revoked_at IS NULL
  AND u.status = 1 AND u.deleted_at IS NULL;

-- name: TouchAPITokenUsage :exec
UPDATE sys_api_token SET last_used_at = NOW(), last_used_ip = $1 WHERE id = $2;

-- name: InsertOperationLog :exec
INSERT INTO sys_operation_log
  (user_id, username, module, action, permission, method, path, target_type,
   target_id, request_id, ip, user_agent, success, status_code, duration_ms,
   error_code, error_message)
VALUES (NULLIF(sqlc.arg(user_id)::bigint, 0), sqlc.arg(username), sqlc.arg(module), sqlc.arg(action),
  sqlc.arg(permission), sqlc.arg(method), sqlc.arg(path), sqlc.arg(target_type),
  sqlc.arg(target_id), sqlc.arg(request_id), sqlc.arg(ip), sqlc.arg(user_agent),
  sqlc.arg(success), sqlc.arg(status_code), sqlc.arg(duration_ms),
  sqlc.arg(error_code), sqlc.arg(error_message));

-- name: InsertErrorLog :exec
INSERT INTO sys_error_log
  (request_id, user_id, module, method, path, error_code, message, ip)
VALUES (sqlc.arg(request_id), NULLIF(sqlc.arg(user_id)::bigint, 0), sqlc.arg(module), sqlc.arg(method),
  sqlc.arg(path), sqlc.arg(error_code), sqlc.arg(message), sqlc.arg(ip));

-- name: ResolveErrorLog :execrows
UPDATE sys_error_log SET resolved = 1, resolved_by = $1, resolved_at = NOW()
WHERE id = $2 AND resolved = 0;

-- name: GetDashboardSummary :one
SELECT
  (SELECT COUNT(*) FROM sys_user WHERE deleted_at IS NULL) AS users,
  (SELECT COUNT(*) FROM sys_user WHERE deleted_at IS NULL AND status = 1) AS enabled_users,
  (SELECT COUNT(*) FROM sys_refresh_token WHERE revoked_at IS NULL AND expires_at > NOW()) AS online_sessions,
  (SELECT COUNT(*) FROM sys_department WHERE deleted_at IS NULL) AS departments,
  (SELECT COUNT(*) FROM sys_role WHERE deleted_at IS NULL) AS roles,
  (SELECT COUNT(*) FROM sys_file WHERE deleted_at IS NULL AND status = 'ready') AS files,
  (SELECT COALESCE(SUM(size), 0)::bigint FROM sys_file WHERE deleted_at IS NULL AND status = 'ready') AS storage_bytes,
  (SELECT COUNT(*) FROM sys_notice n WHERE n.deleted_at IS NULL AND n.status = 'published'
    AND (n.publish_at IS NULL OR n.publish_at <= NOW())
    AND NOT EXISTS (SELECT 1 FROM sys_notice_read nr WHERE nr.notice_id = n.id AND nr.user_id = $1)) AS unread_notices,
  (SELECT COUNT(*) FROM sys_login_log WHERE success = 1 AND created_at >= CURRENT_DATE) AS today_logins,
  (SELECT COUNT(*) FROM sys_login_log WHERE success = 0 AND created_at >= CURRENT_DATE) AS today_failures,
  (SELECT COUNT(*) FROM sys_error_log WHERE resolved = 0) AS unresolved_errors;

-- name: ListLoginTrendLast7Days :many
SELECT TO_CHAR(created_at::date, 'YYYY-MM-DD') AS date, success, COUNT(*) AS count
FROM sys_login_log
WHERE created_at >= CURRENT_DATE - INTERVAL '6 days'
GROUP BY created_at::date, success
ORDER BY created_at::date;

-- name: CountSessionsFiltered :one
SELECT COUNT(*)
FROM sys_refresh_token rt
INNER JOIN sys_user u ON u.id = rt.user_id
WHERE rt.revoked_at IS NULL AND rt.expires_at > NOW() AND u.deleted_at IS NULL
  AND (sqlc.narg('keyword_pattern')::text IS NULL OR (
    u.username LIKE sqlc.narg('keyword_pattern')
    OR u.display_name LIKE sqlc.narg('keyword_pattern')
    OR rt.ip LIKE sqlc.narg('keyword_pattern')
    OR rt.device_name LIKE sqlc.narg('keyword_pattern')));

-- name: ListSessionsFiltered :many
SELECT rt.id, rt.user_id, u.username, u.display_name,
  rt.device_id, rt.device_name, rt.login_method, rt.ip, rt.user_agent, rt.expires_at,
  rt.last_used_at, rt.created_at
FROM sys_refresh_token rt
INNER JOIN sys_user u ON u.id = rt.user_id
WHERE rt.revoked_at IS NULL AND rt.expires_at > NOW() AND u.deleted_at IS NULL
  AND (sqlc.narg('keyword_pattern')::text IS NULL OR (
    u.username LIKE sqlc.narg('keyword_pattern')
    OR u.display_name LIKE sqlc.narg('keyword_pattern')
    OR rt.ip LIKE sqlc.narg('keyword_pattern')
    OR rt.device_name LIKE sqlc.narg('keyword_pattern')))
ORDER BY COALESCE(rt.last_used_at, rt.created_at) DESC
LIMIT $1 OFFSET $2;

-- name: CountAPITokensFiltered :one
SELECT COUNT(*)
FROM sys_api_token t
INNER JOIN sys_user u ON u.id = t.user_id
WHERE (sqlc.narg('keyword_pattern')::text IS NULL OR (
    t.name LIKE sqlc.narg('keyword_pattern')
    OR t.token_prefix LIKE sqlc.narg('keyword_pattern')
    OR u.username LIKE sqlc.narg('keyword_pattern')))
  AND (sqlc.narg('user_id')::bigint IS NULL OR t.user_id = sqlc.narg('user_id'))
  AND (sqlc.narg('status')::smallint IS NULL OR t.status = sqlc.narg('status'));

-- name: ListAPITokensFiltered :many
SELECT t.id, t.user_id, u.username, t.name, t.token_prefix, t.scopes,
  COALESCE(t.ip_allowlist, '[]'::jsonb) AS ip_allowlist, t.status, t.expires_at, t.last_used_at,
  t.last_used_ip, t.created_at, t.revoked_at
FROM sys_api_token t
INNER JOIN sys_user u ON u.id = t.user_id
WHERE (sqlc.narg('keyword_pattern')::text IS NULL OR (
    t.name LIKE sqlc.narg('keyword_pattern')
    OR t.token_prefix LIKE sqlc.narg('keyword_pattern')
    OR u.username LIKE sqlc.narg('keyword_pattern')))
  AND (sqlc.narg('user_id')::bigint IS NULL OR t.user_id = sqlc.narg('user_id'))
  AND (sqlc.narg('status')::smallint IS NULL OR t.status = sqlc.narg('status'))
ORDER BY t.created_at DESC
LIMIT $1 OFFSET $2;

-- name: CountLoginLogsFiltered :one
SELECT COUNT(*) FROM sys_login_log
WHERE (sqlc.narg('keyword_pattern')::text IS NULL OR (
    username LIKE sqlc.narg('keyword_pattern')
    OR ip LIKE sqlc.narg('keyword_pattern')
    OR failure_reason LIKE sqlc.narg('keyword_pattern')))
  AND (sqlc.narg('success')::smallint IS NULL OR success = sqlc.narg('success'));

-- name: ListLoginLogsFiltered :many
SELECT id, user_id, username, login_method, success,
  failure_reason, ip, location, user_agent, is_suspicious, created_at
FROM sys_login_log
WHERE (sqlc.narg('keyword_pattern')::text IS NULL OR (
    username LIKE sqlc.narg('keyword_pattern')
    OR ip LIKE sqlc.narg('keyword_pattern')
    OR failure_reason LIKE sqlc.narg('keyword_pattern')))
  AND (sqlc.narg('success')::smallint IS NULL OR success = sqlc.narg('success'))
ORDER BY id DESC
LIMIT $1 OFFSET $2;

-- name: CountOperationLogsFiltered :one
SELECT COUNT(*) FROM sys_operation_log
WHERE (sqlc.narg('keyword_pattern')::text IS NULL OR (
    username LIKE sqlc.narg('keyword_pattern')
    OR path LIKE sqlc.narg('keyword_pattern')
    OR target_name LIKE sqlc.narg('keyword_pattern')
    OR request_id LIKE sqlc.narg('keyword_pattern')))
  AND (sqlc.narg('module')::text IS NULL OR module = sqlc.narg('module'))
  AND (sqlc.narg('success')::smallint IS NULL OR success = sqlc.narg('success'));

-- name: ListOperationLogsFiltered :many
SELECT id, user_id, username, module, action, permission,
  method, path, target_type, target_id, request_id, ip, success, status_code, duration_ms,
  COALESCE(before_json, '{}'::jsonb) AS before_json,
  COALESCE(after_json, '{}'::jsonb) AS after_json,
  error_code, error_message, created_at
FROM sys_operation_log
WHERE (sqlc.narg('keyword_pattern')::text IS NULL OR (
    username LIKE sqlc.narg('keyword_pattern')
    OR path LIKE sqlc.narg('keyword_pattern')
    OR target_name LIKE sqlc.narg('keyword_pattern')
    OR request_id LIKE sqlc.narg('keyword_pattern')))
  AND (sqlc.narg('module')::text IS NULL OR module = sqlc.narg('module'))
  AND (sqlc.narg('success')::smallint IS NULL OR success = sqlc.narg('success'))
ORDER BY id DESC
LIMIT $1 OFFSET $2;

-- name: CountErrorLogsFiltered :one
SELECT COUNT(*) FROM sys_error_log
WHERE (sqlc.narg('keyword_pattern')::text IS NULL OR (
    message LIKE sqlc.narg('keyword_pattern')
    OR path LIKE sqlc.narg('keyword_pattern')
    OR request_id LIKE sqlc.narg('keyword_pattern')))
  AND (sqlc.narg('resolved')::smallint IS NULL OR resolved = sqlc.narg('resolved'));

-- name: ListErrorLogsFiltered :many
SELECT id, request_id, user_id, module, method, path,
  error_code, message, ip, resolved, resolved_by, resolved_at, created_at
FROM sys_error_log
WHERE (sqlc.narg('keyword_pattern')::text IS NULL OR (
    message LIKE sqlc.narg('keyword_pattern')
    OR path LIKE sqlc.narg('keyword_pattern')
    OR request_id LIKE sqlc.narg('keyword_pattern')))
  AND (sqlc.narg('resolved')::smallint IS NULL OR resolved = sqlc.narg('resolved'))
ORDER BY id DESC
LIMIT $1 OFFSET $2;
