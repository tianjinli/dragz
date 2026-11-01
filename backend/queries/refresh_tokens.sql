-- name: InsertRefreshToken :exec
INSERT INTO sys_refresh_token
  (user_id, token_hash, family_id, device_id, device_name, ip, user_agent, expires_at)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8);

-- name: InsertExternalRefreshToken :exec
INSERT INTO sys_refresh_token
  (user_id, token_hash, family_id, login_method, ip, user_agent, expires_at)
VALUES ($1, $2, $3, $4, $5, $6, $7);

-- name: FindRefreshSession :one
SELECT rt.id, rt.user_id, u.username, rt.token_hash, rt.family_id,
  rt.expires_at, rt.revoked_at
FROM sys_refresh_token rt
INNER JOIN sys_user u ON u.id = rt.user_id
WHERE rt.token_hash = $1 AND u.deleted_at IS NULL
LIMIT 1;

-- name: MarkRefreshTokenRotated :execrows
UPDATE sys_refresh_token SET revoked_at = NOW(),
  revoke_reason = 'rotated', last_used_at = NOW()
WHERE id = $1 AND revoked_at IS NULL;

-- name: RevokeRefreshTokenByHash :exec
UPDATE sys_refresh_token SET revoked_at = COALESCE(revoked_at, NOW()),
  revoke_reason = CASE WHEN revoked_at IS NULL THEN $1 ELSE revoke_reason END
WHERE token_hash = $2;

-- name: RevokeRefreshFamily :exec
UPDATE sys_refresh_token SET revoked_at = COALESCE(revoked_at, NOW()),
  revoke_reason = CASE WHEN revoked_at IS NULL THEN $1 ELSE revoke_reason END
WHERE family_id = $2;

-- name: RevokeUserRefreshTokens :exec
UPDATE sys_refresh_token SET revoked_at = COALESCE(revoked_at, NOW()),
  revoke_reason = CASE WHEN revoked_at IS NULL THEN $1 ELSE revoke_reason END
WHERE user_id = $2 AND revoked_at IS NULL;

-- name: RevokeAllUserRefreshTokens :exec
UPDATE sys_refresh_token SET revoked_at = COALESCE(revoked_at, NOW()),
  revoke_reason = CASE WHEN revoked_at IS NULL THEN $1 ELSE revoke_reason END
WHERE user_id = $2;

-- name: RevokeSession :execrows
UPDATE sys_refresh_token SET revoked_at = NOW(), revoke_reason = $1
WHERE id = $2 AND revoked_at IS NULL;
