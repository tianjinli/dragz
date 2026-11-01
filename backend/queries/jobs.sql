-- name: ListJobs :many
SELECT id, name, code, handler, cron_expression,
  COALESCE(payload, '{}'::jsonb) AS payload, timeout_seconds, max_retries, status, last_run_at,
  next_run_at, created_at, updated_at
FROM sys_scheduled_job
WHERE deleted_at IS NULL
ORDER BY id DESC;

-- name: GetJobByID :one
SELECT id, name, code, handler, cron_expression,
  COALESCE(payload, '{}'::jsonb) AS payload, timeout_seconds, max_retries, status, last_run_at,
  next_run_at, created_at, updated_at
FROM sys_scheduled_job
WHERE id = $1 AND deleted_at IS NULL;

-- name: CreateJob :one
INSERT INTO sys_scheduled_job
  (name, code, handler, cron_expression, payload, timeout_seconds, max_retries, status, next_run_at)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
RETURNING id;

-- name: UpdateJob :execrows
UPDATE sys_scheduled_job SET name = $1, code = $2,
  handler = $3, cron_expression = $4, payload = $5, timeout_seconds = $6, max_retries = $7,
  status = $8, next_run_at = $9
WHERE id = $10 AND deleted_at IS NULL;

-- name: SoftDeleteJob :execrows
UPDATE sys_scheduled_job SET deleted_at = NOW(), status = 0, next_run_at = NULL
WHERE id = $1 AND deleted_at IS NULL;

-- name: ListEnabledJobs :many
SELECT id, name, code, handler, cron_expression,
  COALESCE(payload, '{}'::jsonb) AS payload, timeout_seconds, max_retries, status, last_run_at,
  next_run_at, created_at, updated_at
FROM sys_scheduled_job
WHERE deleted_at IS NULL AND status = 1
ORDER BY id;

-- name: UpdateJobNextRunAt :exec
UPDATE sys_scheduled_job SET next_run_at = $1
WHERE id = $2 AND deleted_at IS NULL;

-- name: TouchJobLastRunAt :exec
UPDATE sys_scheduled_job SET last_run_at = NOW() WHERE id = $1;

-- name: LatestJobLogSince :one
SELECT id, job_id, trigger_type, started_at,
  finished_at, success, attempt, COALESCE(output, '') AS output,
  COALESCE(error_message, '') AS error_message, created_at
FROM sys_job_log
WHERE job_id = $1 AND trigger_type = $2 AND started_at >= $3
ORDER BY id DESC
LIMIT 1;

-- name: StartJobLog :one
INSERT INTO sys_job_log (job_id, trigger_type, started_at, attempt)
VALUES ($1, $2, NOW(), $3)
RETURNING id;

-- name: FinishJobLog :exec
UPDATE sys_job_log SET finished_at = NOW(), success = $1,
  output = $2, error_message = $3
WHERE id = $4 AND job_id = $5;

-- name: CountJobLogs :one
SELECT COUNT(*) FROM sys_job_log WHERE job_id = $1;

-- name: ListJobLogs :many
SELECT id, job_id, trigger_type, started_at,
  finished_at, success, attempt, COALESCE(output, '') AS output,
  COALESCE(error_message, '') AS error_message, created_at
FROM sys_job_log
WHERE job_id = $1
ORDER BY id DESC
LIMIT $2 OFFSET $3;
