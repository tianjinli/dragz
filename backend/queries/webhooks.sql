-- name: ListWebhooks :many
SELECT id, name, url, secret_encrypted, events,
  COALESCE(headers_encrypted, '') AS headers_encrypted, allow_private_network, timeout_seconds, max_retries,
  status, last_success_at, last_failure_at, created_at, updated_at
FROM sys_webhook
WHERE deleted_at IS NULL
ORDER BY id DESC;

-- name: GetWebhookByID :one
SELECT id, name, url, secret_encrypted, events,
  COALESCE(headers_encrypted, '') AS headers_encrypted, allow_private_network, timeout_seconds, max_retries,
  status, last_success_at, last_failure_at, created_at, updated_at
FROM sys_webhook
WHERE id = $1 AND deleted_at IS NULL;

-- name: CreateWebhook :one
INSERT INTO sys_webhook
  (name, url, secret_encrypted, events, headers_encrypted, allow_private_network,
   timeout_seconds, max_retries, status)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
RETURNING id;

-- name: UpdateWebhookStatus :execrows
UPDATE sys_webhook SET status = $1
WHERE id = $2 AND deleted_at IS NULL;

-- name: UpdateWebhook :execrows
UPDATE sys_webhook SET name = $1, url = $2, events = $3, allow_private_network = $4,
  timeout_seconds = $5, max_retries = $6, status = $7
WHERE id = $8 AND deleted_at IS NULL;

-- name: UpdateWebhookWithSecret :execrows
UPDATE sys_webhook SET name = $1, url = $2, events = $3, allow_private_network = $4,
  timeout_seconds = $5, max_retries = $6, status = $7, secret_encrypted = $8
WHERE id = $9 AND deleted_at IS NULL;

-- name: UpdateWebhookWithHeaders :execrows
UPDATE sys_webhook SET name = $1, url = $2, events = $3, allow_private_network = $4,
  timeout_seconds = $5, max_retries = $6, status = $7, headers_encrypted = $8
WHERE id = $9 AND deleted_at IS NULL;

-- name: UpdateWebhookWithSecretAndHeaders :execrows
UPDATE sys_webhook SET name = $1, url = $2, events = $3, allow_private_network = $4,
  timeout_seconds = $5, max_retries = $6, status = $7, secret_encrypted = $8, headers_encrypted = $9
WHERE id = $10 AND deleted_at IS NULL;

-- name: SoftDeleteWebhook :execrows
UPDATE sys_webhook SET deleted_at = NOW(), status = 0
WHERE id = $1 AND deleted_at IS NULL;

-- name: MarkWebhookSuccess :exec
UPDATE sys_webhook SET last_success_at = NOW() WHERE id = $1;

-- name: MarkWebhookFailure :exec
UPDATE sys_webhook SET last_failure_at = NOW() WHERE id = $1;
