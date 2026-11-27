-- name: GetTenantMembership :one
SELECT t.id, t.code, t.name,
  EXISTS (SELECT 1 FROM sys_platform_operator po WHERE po.user_id = m.user_id) AS is_platform_operator
FROM sys_tenant_member m
INNER JOIN sys_tenant t ON t.id = m.tenant_id
WHERE m.user_id = sqlc.arg(user_id)
  AND m.tenant_id = sqlc.arg(tenant_id)
  AND m.status = 1
  AND t.status = 1
  AND t.deleted_at IS NULL;

-- name: IsTenantMember :one
-- sys_user is a global account table, so a user id alone never authorises a
-- read or a write. Every tenant-scoped user operation checks this first.
SELECT EXISTS (
  SELECT 1 FROM sys_tenant_member m
  WHERE m.user_id = sqlc.arg(user_id)
    AND m.tenant_id = sqlc.arg(tenant_id)
    AND m.status = 1
) AS is_member;

-- name: CountTenantMembers :one
SELECT COUNT(*) FROM sys_tenant_member
WHERE tenant_id = sqlc.arg(tenant_id) AND status = 1;

-- name: ListTenantMembers :many
SELECT m.user_id, m.department_id, m.is_tenant_admin, m.is_platform_operator,
       m.status, m.created_at, u.username, u.display_name, u.email, u.mobile
FROM sys_tenant_member m
INNER JOIN sys_user u ON u.id = m.user_id AND u.deleted_at IS NULL
WHERE m.tenant_id = sqlc.arg(tenant_id)
ORDER BY m.created_at, m.user_id;

-- name: GetTenantMember :one
SELECT m.user_id, m.department_id, m.is_tenant_admin, m.is_platform_operator,
       m.status, m.created_at, u.username, u.display_name, u.email, u.mobile
FROM sys_tenant_member m
INNER JOIN sys_user u ON u.id = m.user_id AND u.deleted_at IS NULL
WHERE m.tenant_id = sqlc.arg(tenant_id) AND m.user_id = sqlc.arg(user_id);

-- name: UpsertTenantMember :exec
INSERT INTO sys_tenant_member (tenant_id, user_id, department_id, is_tenant_admin, is_platform_operator, status,
  created_by, updated_by)
VALUES (sqlc.arg(tenant_id), sqlc.arg(user_id), sqlc.narg(department_id), sqlc.arg(is_tenant_admin),
  sqlc.arg(is_platform_operator), sqlc.arg(status), sqlc.arg(actor), sqlc.arg(actor))
ON CONFLICT (tenant_id, user_id) DO UPDATE
SET department_id = EXCLUDED.department_id,
    is_tenant_admin = EXCLUDED.is_tenant_admin,
    is_platform_operator = EXCLUDED.is_platform_operator,
    status = EXCLUDED.status,
    updated_at = NOW(),
    -- created_by keeps the first writer: an upsert is an edit, not a creation.
    updated_by = EXCLUDED.updated_by;

-- name: SetTenantMemberStatus :execrows
UPDATE sys_tenant_member SET status = sqlc.arg(status), updated_at = NOW(),
  updated_by = sqlc.arg(actor)
WHERE tenant_id = sqlc.arg(tenant_id) AND user_id = sqlc.arg(user_id);

-- name: UpdateTenantMemberDepartment :exec
UPDATE sys_tenant_member SET department_id = sqlc.narg(department_id), updated_at = NOW(),
  updated_by = sqlc.arg(actor)
WHERE tenant_id = sqlc.arg(tenant_id) AND user_id = sqlc.arg(user_id);

-- name: CountMembershipsForUser :one
-- How many tenants still claim this global account. Deleting the account itself
-- is only safe once this reaches zero.
SELECT COUNT(*) FROM sys_tenant_member
WHERE user_id = sqlc.arg(user_id) AND status = 1;

-- name: ReleaseQuota :exec
-- Gives quota back, never below zero and never above the limit.
UPDATE sys_tenant_quota
SET used_value = GREATEST(used_value - sqlc.arg(amount)::bigint, 0), updated_at = NOW()
WHERE tenant_id = sqlc.arg(tenant_id) AND quota_key = sqlc.arg(quota_key);

-- name: DeleteTenantMember :execrows
DELETE FROM sys_tenant_member
WHERE tenant_id = sqlc.arg(tenant_id) AND user_id = sqlc.arg(user_id);

-- name: CountTenantAdmins :one
SELECT COUNT(*) FROM sys_tenant_member
WHERE tenant_id = sqlc.arg(tenant_id) AND status = 1 AND is_tenant_admin = TRUE;

-- name: ListTenantsForUser :many
SELECT t.id, t.code, t.name,
  EXISTS (SELECT 1 FROM sys_platform_operator po WHERE po.user_id = m.user_id) AS is_platform_operator
FROM sys_tenant_member m
INNER JOIN sys_tenant t ON t.id = m.tenant_id
WHERE m.user_id = $1
  AND m.status = 1
  AND t.status = 1
  AND t.deleted_at IS NULL
ORDER BY t.id;

-- name: FirstTenantForUser :one
SELECT t.id, t.code, t.name,
  EXISTS (SELECT 1 FROM sys_platform_operator po WHERE po.user_id = m.user_id) AS is_platform_operator
FROM sys_tenant_member m
INNER JOIN sys_tenant t ON t.id = m.tenant_id
WHERE m.user_id = $1
  AND m.status = 1
  AND t.status = 1
  AND t.deleted_at IS NULL
ORDER BY t.id
LIMIT 1;

-- name: GetOpenTenant :one
-- Current tenant row for the open API. Row-level security already limits sys_tenant to the caller.
SELECT id, code, name, status
FROM sys_tenant
WHERE id = $1 AND deleted_at IS NULL;

-- name: GetPlanEntitlementForTenant :one
-- Active, trial, or overdue (grace) subscription → plan catalog value.
-- Cancelled rows are excluded; EntitlementEnabled denies those.
SELECT pe.entitlement_value
FROM sys_tenant_subscription s
INNER JOIN sys_plan_entitlement pe ON pe.plan_id = s.plan_id
WHERE s.tenant_id = sqlc.arg(tenant_id)
  AND s.status IN ('active', 'trial', 'overdue')
  AND pe.entitlement_key = sqlc.arg(feature_key)
ORDER BY s.id DESC
LIMIT 1;

-- name: GetTenantSubscriptionStatus :one
-- Latest subscription status. sql.ErrNoRows means the tenant has none.
SELECT status
FROM sys_tenant_subscription
WHERE tenant_id = $1
ORDER BY id DESC
LIMIT 1;

-- name: HasActiveImpersonation :one
SELECT EXISTS (
  SELECT 1 FROM sys_tenant_impersonation
  WHERE tenant_id = sqlc.arg(tenant_id)
    AND actor_user_id = sqlc.arg(actor_user_id)
    AND ended_at IS NULL
    AND expires_at > NOW()
)::bool;

-- name: GetTenantQuota :one
SELECT used_value, limit_value
FROM sys_tenant_quota
WHERE tenant_id = sqlc.arg(tenant_id) AND quota_key = sqlc.arg(quota_key);

-- name: TryConsumeQuota :one
UPDATE sys_tenant_quota
SET used_value = used_value + sqlc.arg(amount),
    updated_at = NOW()
WHERE tenant_id = sqlc.arg(tenant_id)
  AND quota_key = sqlc.arg(quota_key)
  AND used_value + sqlc.arg(amount) <= limit_value
RETURNING used_value, limit_value;

-- name: GetPlanIDByCode :one
SELECT id FROM sys_plan WHERE code = sqlc.arg(code) LIMIT 1;

-- name: ListPlanQuotaEntitlements :many
-- quota.<name> entitlements become sys_tenant_quota rows. The value stays
-- text here; the caller parses it and skips anything that is not an integer.
SELECT entitlement_key, entitlement_value
FROM sys_plan_entitlement
WHERE plan_id = sqlc.arg(plan_id)
  AND entitlement_key LIKE 'quota.%'
ORDER BY entitlement_key;

-- name: EnsureTenantQuota :exec
INSERT INTO sys_tenant_quota (tenant_id, quota_key, limit_value, used_value)
VALUES (sqlc.arg(tenant_id), sqlc.arg(quota_key), sqlc.arg(limit_value), 0)
ON CONFLICT (tenant_id, quota_key) DO NOTHING;

-- name: InsertOutboxEvent :one
INSERT INTO sys_outbox (tenant_id, event_type, payload)
VALUES (sqlc.arg(tenant_id), sqlc.arg(event_type), sqlc.arg(payload)::jsonb)
RETURNING id;

-- name: ClaimPendingOutbox :many
-- Claiming, not listing. Two workers running the relay must never deliver the
-- same event twice, and a plain SELECT has no way to express that: both read
-- the same rows, both deliver, and the second MarkOutboxSent becomes a no-op
-- the webhook has already seen. FOR UPDATE SKIP LOCKED moves the row to
-- status 3 inside the same statement, so a concurrent worker skips it instead
-- of blocking on it.
--
-- status 3 = processing. A worker that dies mid-delivery leaves its row there;
-- ReclaimStaleOutbox puts it back after the lease expires.
UPDATE sys_outbox
SET status = 3, claimed_at = NOW(), claimed_by = sqlc.arg(worker_id)
WHERE id IN (
  SELECT id FROM sys_outbox
  WHERE status = 0 AND available_at <= NOW()
  ORDER BY id
  LIMIT sqlc.arg(batch_size)
  FOR UPDATE SKIP LOCKED
)
RETURNING id, tenant_id, event_type, payload, created_at;

-- name: ReclaimStaleOutbox :execrows
-- Returns rows whose worker never reported back. The lease is generous: a
-- delivery holds its claim across webhook retries.
UPDATE sys_outbox
SET status = 0, claimed_at = NULL, claimed_by = ''
WHERE status = 3
  AND claimed_at < NOW() - make_interval(secs => sqlc.arg(lease_seconds)::int);

-- name: MarkOutboxSent :execrows
-- status 3 is accepted because the sender is the worker that claimed the row.
-- A row already marked sent (status 1) is left alone, which is what makes a
-- duplicate call harmless.
UPDATE sys_outbox
SET status = 1, sent_at = NOW(), claimed_at = NULL, claimed_by = ''
WHERE id = sqlc.arg(id) AND status IN (0, 3);

-- name: MarkOutboxFailed :exec
-- attempts is the pre-update value. The next try waits min(attempts+1, 30)
-- minutes. The eighth failure (attempts+1 >= 8) dead-letters the row (status 2).
-- The claim is released in both cases so the row is immediately eligible once
-- its backoff elapses.
UPDATE sys_outbox SET
  attempts = attempts + 1,
  last_error = sqlc.arg(last_error),
  available_at = NOW() + make_interval(mins => LEAST(attempts + 1, 30)),
  status = CASE WHEN attempts + 1 >= 8 THEN 2 ELSE 0 END,
  claimed_at = NULL,
  claimed_by = ''
WHERE id = sqlc.arg(id) AND status IN (0, 3);
