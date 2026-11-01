-- name: ListPublicAuthProviders :many
SELECT id, provider_type, name, icon, sort
FROM sys_auth_provider
WHERE status = 1
  AND deleted_at IS NULL
  AND NULLIF(TRIM(client_id), '') IS NOT NULL
  AND NULLIF(TRIM(client_secret_encrypted), '') IS NOT NULL
  AND NULLIF(TRIM(authorization_url), '') IS NOT NULL
  AND NULLIF(TRIM(token_url), '') IS NOT NULL
ORDER BY sort, id;

-- name: GetActiveAuthProvider :one
SELECT id, provider_type, name, icon, client_id,
  client_secret_encrypted, authorization_url, token_url, user_info_url, issuer_url,
  callback_url, scopes, auto_register, default_role_id, binding_rule, status, sort, updated_at
FROM sys_auth_provider
WHERE id = $1 AND status = 1 AND deleted_at IS NULL;

-- name: GetUserIdentityUserID :one
SELECT user_id FROM sys_user_identity
WHERE provider_id = $1 AND external_user_id = $2;

-- name: InsertUserIdentity :exec
INSERT INTO sys_user_identity
  (user_id, provider_id, external_user_id, external_username, external_email, profile_json, last_login_at)
VALUES ($1, $2, $3, $4, $5, $6, NOW());

-- name: UpdateUserIdentity :exec
UPDATE sys_user_identity SET external_username = $1,
  external_email = $2, profile_json = $3, last_login_at = NOW()
WHERE provider_id = $4 AND external_user_id = $5;
