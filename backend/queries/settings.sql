-- name: GetConfigValue :one
SELECT config_value FROM sys_config WHERE config_key = $1;

-- name: GetFeatureConfigValue :one
SELECT config_value FROM sys_config
WHERE config_key = $1 AND config_group = 'feature';

-- name: ListAllConfigs :many
SELECT id, config_group, config_key, name, value_type, config_value,
  COALESCE(default_value, '') AS default_value, is_secret, is_readonly, effect_mode, description, updated_at
FROM sys_config
ORDER BY config_group, id;

-- name: ListConfigsByGroup :many
SELECT id, config_group, config_key, name, value_type, config_value,
  COALESCE(default_value, '') AS default_value, is_secret, is_readonly, effect_mode, description, updated_at
FROM sys_config
WHERE config_group = $1
ORDER BY config_group, id;

-- name: GetConfigByKey :one
SELECT id, config_group, config_key, name, value_type,
  config_value, COALESCE(default_value, '') AS default_value, is_secret, is_readonly, effect_mode,
  description, updated_at
FROM sys_config WHERE config_key = $1;

-- name: GetConfigByKeyForUpdate :one
SELECT id, config_group, config_key, name, value_type,
  config_value, COALESCE(default_value, '') AS default_value, is_secret, is_readonly, effect_mode,
  description, updated_at
FROM sys_config WHERE config_key = $1
FOR UPDATE;

-- name: UpdateConfigValue :exec
UPDATE sys_config SET config_value = $1 WHERE config_key = $2;

-- name: InsertConfigLog :exec
INSERT INTO sys_config_log (config_type, config_key, operator_id, before_json, after_json)
VALUES ('config', $1, $2, $3, $4);

-- name: ListAuthProviders :many
SELECT id, provider_type, name, icon, client_id,
  client_secret_encrypted, authorization_url, token_url, user_info_url, issuer_url,
  callback_url, scopes, auto_register, default_role_id, binding_rule, status, sort, updated_at
FROM sys_auth_provider
WHERE deleted_at IS NULL
ORDER BY sort, id;

-- name: GetAuthProviderSecret :one
SELECT client_secret_encrypted FROM sys_auth_provider
WHERE id = $1 AND deleted_at IS NULL;

-- name: CreateAuthProvider :one
INSERT INTO sys_auth_provider
  (provider_type, name, icon, client_id, client_secret_encrypted, authorization_url,
   token_url, user_info_url, issuer_url, callback_url, scopes, auto_register,
   default_role_id, binding_rule, status, sort)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16)
RETURNING id;

-- name: UpdateAuthProviderStatus :execrows
UPDATE sys_auth_provider SET status = $1
WHERE id = $2 AND deleted_at IS NULL;

-- name: UpdateAuthProvider :execrows
UPDATE sys_auth_provider SET provider_type = $1, name = $2, icon = $3, client_id = $4,
  authorization_url = $5, token_url = $6, user_info_url = $7, issuer_url = $8, callback_url = $9,
  scopes = $10, auto_register = $11, default_role_id = $12, binding_rule = $13, status = $14, sort = $15
WHERE id = $16 AND deleted_at IS NULL;

-- name: UpdateAuthProviderWithSecret :execrows
UPDATE sys_auth_provider SET provider_type = $1, name = $2, icon = $3, client_id = $4,
  authorization_url = $5, token_url = $6, user_info_url = $7, issuer_url = $8, callback_url = $9,
  scopes = $10, auto_register = $11, default_role_id = $12, binding_rule = $13, status = $14, sort = $15,
  client_secret_encrypted = $16
WHERE id = $17 AND deleted_at IS NULL;

-- name: CountUserIdentitiesByProvider :one
SELECT COUNT(*) FROM sys_user_identity WHERE provider_id = $1;

-- name: SoftDeleteAuthProvider :execrows
UPDATE sys_auth_provider SET deleted_at = NOW(), status = 0
WHERE id = $1 AND deleted_at IS NULL;

-- name: ListStorageConfigs :many
SELECT id, name, storage_type, endpoint, bucket, region,
  COALESCE(access_key_encrypted, '') AS access_key_encrypted,
  COALESCE(secret_key_encrypted, '') AS secret_key_encrypted, base_path,
  public_base_url, use_https, private_access, max_file_size,
  COALESCE(allowed_extensions, '') AS allowed_extensions,
  is_default, status, updated_at
FROM sys_storage_config
WHERE deleted_at IS NULL
ORDER BY is_default DESC, id;

-- name: GetStorageSecrets :one
SELECT access_key_encrypted, secret_key_encrypted
FROM sys_storage_config WHERE id = $1 AND deleted_at IS NULL;

-- name: GetStorageConfigByID :one
SELECT id, name, storage_type, endpoint, bucket, region,
  COALESCE(access_key_encrypted, '') AS access_key_encrypted,
  COALESCE(secret_key_encrypted, '') AS secret_key_encrypted, base_path,
  public_base_url, use_https, private_access, max_file_size,
  COALESCE(allowed_extensions, '') AS allowed_extensions,
  is_default, status, updated_at
FROM sys_storage_config
WHERE id = $1 AND deleted_at IS NULL;

-- name: GetDefaultStorageConfig :one
SELECT id, name, storage_type, endpoint, bucket, region,
  COALESCE(access_key_encrypted, '') AS access_key_encrypted,
  COALESCE(secret_key_encrypted, '') AS secret_key_encrypted, base_path,
  public_base_url, use_https, private_access, max_file_size,
  COALESCE(allowed_extensions, '') AS allowed_extensions,
  is_default, status, updated_at
FROM sys_storage_config
WHERE is_default = 1 AND status = 1 AND deleted_at IS NULL
LIMIT 1;

-- name: ClearDefaultStorageConfigs :exec
UPDATE sys_storage_config SET is_default = 0 WHERE deleted_at IS NULL;

-- name: ClearOtherDefaultStorageConfigs :exec
UPDATE sys_storage_config SET is_default = 0 WHERE id <> $1 AND deleted_at IS NULL;

-- name: CreateStorageConfig :one
INSERT INTO sys_storage_config
  (name, storage_type, endpoint, bucket, region, access_key_encrypted, secret_key_encrypted,
   base_path, public_base_url, use_https, private_access, max_file_size,
   allowed_extensions, is_default, status)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15)
RETURNING id;

-- name: UpdateStorageConfig :execrows
UPDATE sys_storage_config SET name = $1, storage_type = $2, endpoint = $3, bucket = $4,
  region = $5, base_path = $6, public_base_url = $7, use_https = $8, private_access = $9,
  max_file_size = $10, allowed_extensions = $11, is_default = $12, status = $13
WHERE id = $14 AND deleted_at IS NULL;

-- name: UpdateStorageConfigWithAccessKey :execrows
UPDATE sys_storage_config SET name = $1, storage_type = $2, endpoint = $3, bucket = $4,
  region = $5, base_path = $6, public_base_url = $7, use_https = $8, private_access = $9,
  max_file_size = $10, allowed_extensions = $11, is_default = $12, status = $13,
  access_key_encrypted = $14
WHERE id = $15 AND deleted_at IS NULL;

-- name: UpdateStorageConfigWithSecretKey :execrows
UPDATE sys_storage_config SET name = $1, storage_type = $2, endpoint = $3, bucket = $4,
  region = $5, base_path = $6, public_base_url = $7, use_https = $8, private_access = $9,
  max_file_size = $10, allowed_extensions = $11, is_default = $12, status = $13,
  secret_key_encrypted = $14
WHERE id = $15 AND deleted_at IS NULL;

-- name: UpdateStorageConfigWithBothKeys :execrows
UPDATE sys_storage_config SET name = $1, storage_type = $2, endpoint = $3, bucket = $4,
  region = $5, base_path = $6, public_base_url = $7, use_https = $8, private_access = $9,
  max_file_size = $10, allowed_extensions = $11, is_default = $12, status = $13,
  access_key_encrypted = $14, secret_key_encrypted = $15
WHERE id = $16 AND deleted_at IS NULL;

-- name: GetStorageDeleteChecks :one
SELECT s.is_default,
  (SELECT COUNT(*) FROM sys_file WHERE storage_id = s.id AND deleted_at IS NULL) AS file_count
FROM sys_storage_config s
WHERE s.id = $1 AND s.deleted_at IS NULL;

-- name: SoftDeleteStorageConfig :execrows
UPDATE sys_storage_config SET deleted_at = NOW(), status = 0
WHERE id = $1 AND deleted_at IS NULL;
