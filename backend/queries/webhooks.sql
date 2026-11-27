-- name: ListWebhooks :many
SELECT id, name, url, secret_encrypted, events,
  COALESCE(headers_encrypted, '') AS headers_encrypted, allow_private_network, timeout_seconds, max_retries,
  status, last_success_at, last_failure_at, created_at, updated_at
FROM sys_webhook
WHERE deleted_at IS NULL
ORDER BY id DESC;

-- name: ListEnabledWebhooks :many
-- Delivery only. Secrets stay encrypted; tenant RLS applies when the session is set.
SELECT id, name, url, secret_encrypted, events,
  COALESCE(headers_encrypted, '') AS headers_encrypted, allow_private_network, timeout_seconds, max_retries,
  status, last_success_at, last_failure_at, created_at, updated_at
FROM sys_webhook
WHERE deleted_at IS NULL AND status = 1
ORDER BY id;

-- name: GetWebhookByID :one
SELECT id, name, url, secret_encrypted, events,
  COALESCE(headers_encrypted, '') AS headers_encrypted, allow_private_network, timeout_seconds, max_retries,
  status, last_success_at, last_failure_at, created_at, updated_at
FROM sys_webhook
WHERE id = $1 AND deleted_at IS NULL;

-- name: CreateWebhook :one
INSERT INTO sys_webhook
  (name, url, secret_encrypted, events, headers_encrypted, allow_private_network,
   timeout_seconds, max_retries, status, created_by, updated_by)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $10)
RETURNING id;

-- name: UpdateWebhookStatus :execrows
UPDATE sys_webhook SET status = $1, updated_by = $3
WHERE id = $2 AND deleted_at IS NULL;

-- name: UpdateWebhook :execrows
UPDATE sys_webhook SET name = $1, url = $2, events = $3, allow_private_network = $4,
  timeout_seconds = $5, max_retries = $6, status = $7, updated_by = $8
WHERE id = $9 AND deleted_at IS NULL;

-- name: UpdateWebhookWithSecret :execrows
UPDATE sys_webhook SET name = $1, url = $2, events = $3, allow_private_network = $4,
  timeout_seconds = $5, max_retries = $6, status = $7, secret_encrypted = $8, updated_by = $9
WHERE id = $10 AND deleted_at IS NULL;

-- name: UpdateWebhookWithHeaders :execrows
UPDATE sys_webhook SET name = $1, url = $2, events = $3, allow_private_network = $4,
  timeout_seconds = $5, max_retries = $6, status = $7, headers_encrypted = $8, updated_by = $9
WHERE id = $10 AND deleted_at IS NULL;

-- name: UpdateWebhookWithSecretAndHeaders :execrows
UPDATE sys_webhook SET name = $1, url = $2, events = $3, allow_private_network = $4,
  timeout_seconds = $5, max_retries = $6, status = $7, secret_encrypted = $8, headers_encrypted = $9,
  updated_by = $10
WHERE id = $11 AND deleted_at IS NULL;

-- name: SoftDeleteWebhook :execrows
UPDATE sys_webhook SET deleted_at = NOW(), status = 0, updated_by = $2
WHERE id = $1 AND deleted_at IS NULL;

-- name: MarkWebhookSuccess :exec
-- Delivery outcome, not an edit: the last human editor must survive it.
UPDATE sys_webhook SET last_success_at = NOW() WHERE id = $1;

-- name: MarkWebhookFailure :exec
UPDATE sys_webhook SET last_failure_at = NOW() WHERE id = $1;
