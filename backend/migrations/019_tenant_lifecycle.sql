-- 019: self-service tenant lifecycle.
--
-- Four things land here, all of them needed for a tenant to exist without a
-- platform engineer:
--
--   1. sys_platform_operator — platform staff are no longer modelled as a
--      membership flag. A platform operator is a person, and that person does
--      not have to belong to any tenant.
--   2. sys_tenant_invite — an invitation is a bearer token with a tenant, a
--      role set and an expiry. Accepting it creates a membership, never a
--      second global account for the same human.
--   3. Plan versions and a subscription snapshot — a plan edit must not
--      retroactively change what an existing subscriber paid for.
--   4. sys_payment_event — provider webhooks are stored before they are
--      applied, keyed by (provider, event_id), so a retried delivery is
--      answered idempotently instead of billing twice.
--
-- Every statement is re-runnable: the deploy path applies these files with a
-- plain psql loop and no bookkeeping table.

-- ---------------------------------------------------------------------------
-- 1. Platform operators
-- ---------------------------------------------------------------------------
-- No tenant_id and no RLS: the table is a global staff directory, like
-- sys_tenant_domain. It holds user ids only, and every write goes through a
-- platform route.
CREATE TABLE IF NOT EXISTS sys_platform_operator (
  user_id BIGINT PRIMARY KEY,
  granted_by BIGINT NOT NULL DEFAULT 0,
  note VARCHAR(255) NOT NULL DEFAULT '',
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Backfill: the membership flag was the previous source of truth. Super
-- administrators kept implicit access before that; record them too so this
-- table becomes the single answer to "is this person platform staff?".
INSERT INTO sys_platform_operator (user_id, note)
SELECT m.user_id, 'migrated from sys_tenant_member.is_platform_operator'
FROM sys_tenant_member m
WHERE m.is_platform_operator
ON CONFLICT (user_id) DO NOTHING;

INSERT INTO sys_platform_operator (user_id, note)
SELECT u.id, 'migrated from sys_user.is_super_admin'
FROM sys_user u
WHERE u.is_super_admin <> 0 AND u.deleted_at IS NULL
ON CONFLICT (user_id) DO NOTHING;

-- ---------------------------------------------------------------------------
-- 2. Invitations
-- ---------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS sys_tenant_invite (
  id BIGSERIAL PRIMARY KEY,
  tenant_id BIGINT NOT NULL,
  -- Empty means "any account may accept". A non-empty address is checked
  -- against the signed-in account so a forwarded token is not enough.
  email VARCHAR(255) NOT NULL DEFAULT '',
  -- Role assignments applied when the invite is accepted. Kept as an array
  -- because invitation roles are a set, not an entity of their own.
  role_ids BIGINT[] NOT NULL DEFAULT '{}'::bigint[],
  department_id BIGINT NULL,
  is_tenant_admin SMALLINT NOT NULL DEFAULT 0,
  -- SHA-256 of the token. The token itself is shown once, at creation.
  token_hash CHAR(64) NOT NULL,
  -- pending | accepted | revoked | expired
  status VARCHAR(16) NOT NULL DEFAULT 'pending',
  invited_by BIGINT NOT NULL,
  accepted_by BIGINT NULL,
  expires_at TIMESTAMPTZ NOT NULL,
  accepted_at TIMESTAMPTZ NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX IF NOT EXISTS uk_tenant_invite_token ON sys_tenant_invite (token_hash);
CREATE INDEX IF NOT EXISTS idx_tenant_invite_tenant ON sys_tenant_invite (tenant_id, id DESC);
-- One live invite per address per tenant. Re-inviting the same address
-- replaces the old link instead of leaving two valid ones.
CREATE UNIQUE INDEX IF NOT EXISTS uk_tenant_invite_pending_email
  ON sys_tenant_invite (tenant_id, lower(email))
  WHERE status = 'pending' AND email <> '';

-- Invitations are tenant rows: a member of tenant A must not see tenant B's
-- outstanding invites, and RLS is what enforces that rather than a WHERE
-- clause somebody can forget.
DROP TRIGGER IF EXISTS trg_fill_tenant_id ON sys_tenant_invite;
CREATE TRIGGER trg_fill_tenant_id
  BEFORE INSERT ON sys_tenant_invite
  FOR EACH ROW EXECUTE FUNCTION app_fill_tenant_id();

ALTER TABLE sys_tenant_invite ENABLE ROW LEVEL SECURITY;
ALTER TABLE sys_tenant_invite FORCE ROW LEVEL SECURITY;
DROP POLICY IF EXISTS tenant_isolation ON sys_tenant_invite;
CREATE POLICY tenant_isolation ON sys_tenant_invite
  USING (app_rls_bypass() OR tenant_id = app_current_tenant_id())
  WITH CHECK (app_rls_bypass() OR tenant_id = app_current_tenant_id());

-- A department named by an invite must belong to the inviting tenant.
ALTER TABLE sys_tenant_invite DROP CONSTRAINT IF EXISTS fk_tenant_invite_department;
ALTER TABLE sys_tenant_invite
  ADD CONSTRAINT fk_tenant_invite_department
  FOREIGN KEY (tenant_id, department_id) REFERENCES sys_department (tenant_id, id)
  DEFERRABLE INITIALLY DEFERRED;

-- ---------------------------------------------------------------------------
-- 3. Plan versions and subscription snapshots
-- ---------------------------------------------------------------------------
ALTER TABLE sys_plan ADD COLUMN IF NOT EXISTS version INT NOT NULL DEFAULT 1;

CREATE TABLE IF NOT EXISTS sys_plan_version (
  id BIGSERIAL PRIMARY KEY,
  plan_id BIGINT NOT NULL,
  version INT NOT NULL,
  -- The whole plan as it stood at this version: code, name, entitlements and
  -- quotas. A subscriber is pinned to the snapshot, not to the live catalog.
  snapshot JSONB NOT NULL DEFAULT '{}'::jsonb,
  created_by BIGINT NOT NULL DEFAULT 0,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX IF NOT EXISTS uk_plan_version ON sys_plan_version (plan_id, version);
CREATE INDEX IF NOT EXISTS idx_plan_version_plan ON sys_plan_version (plan_id, version DESC);

ALTER TABLE sys_tenant_subscription ADD COLUMN IF NOT EXISTS plan_version_id BIGINT NULL;
ALTER TABLE sys_tenant_subscription ADD COLUMN IF NOT EXISTS plan_snapshot JSONB NOT NULL DEFAULT '{}'::jsonb;

-- Backfill one version per existing plan, then pin every existing
-- subscription to it. jsonb_object_agg cannot aggregate an empty set, hence
-- the COALESCE around the subquery.
INSERT INTO sys_plan_version (plan_id, version, snapshot)
SELECT p.id,
       p.version,
       jsonb_build_object(
         'planId', p.id,
         'code', p.code,
         'name', p.name,
         'version', p.version,
         'entitlements', COALESCE((
           SELECT jsonb_object_agg(e.entitlement_key, e.entitlement_value)
           FROM sys_plan_entitlement e
           WHERE e.plan_id = p.id
         ), '{}'::jsonb)
       )
FROM sys_plan p
WHERE NOT EXISTS (
  SELECT 1 FROM sys_plan_version v WHERE v.plan_id = p.id AND v.version = p.version
);

UPDATE sys_tenant_subscription s
SET plan_version_id = v.id,
    plan_snapshot = v.snapshot
FROM sys_plan_version v
WHERE v.plan_id = s.plan_id
  AND s.plan_snapshot = '{}'::jsonb
  AND v.version = (SELECT MAX(v2.version) FROM sys_plan_version v2 WHERE v2.plan_id = s.plan_id);

-- Entitlement lookups now read the snapshot first and only fall back to the
-- live catalog for rows written before this migration. COALESCE keeps the
-- old behaviour for those, and a removed entitlement is still visible through
-- the live plan if the snapshot does not mention it.
-- (The query lives in queries/tenants.sql; this note exists so the intent is
-- discoverable from the schema side too.)

-- ---------------------------------------------------------------------------
-- 4. Payment webhook idempotency
-- ---------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS sys_payment_event (
  id BIGSERIAL PRIMARY KEY,
  provider VARCHAR(64) NOT NULL,
  -- The provider's own event id. The unique index below is the idempotency
  -- key: a retried delivery inserts nothing and is answered from the stored
  -- row instead of being applied twice.
  event_id VARCHAR(128) NOT NULL,
  event_type VARCHAR(128) NOT NULL DEFAULT '',
  -- 0 when the event names a tenant nobody can resolve. Such a row is kept
  -- for forensics and never applied.
  tenant_id BIGINT NOT NULL DEFAULT 0,
  payload JSONB NOT NULL DEFAULT '{}'::jsonb,
  -- received | processing | processed | ignored | failed
  status VARCHAR(16) NOT NULL DEFAULT 'received',
  attempts INT NOT NULL DEFAULT 0,
  last_error VARCHAR(1000) NOT NULL DEFAULT '',
  received_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  processed_at TIMESTAMPTZ NULL
);

CREATE UNIQUE INDEX IF NOT EXISTS uk_payment_event ON sys_payment_event (provider, event_id);
CREATE INDEX IF NOT EXISTS idx_payment_event_pending ON sys_payment_event (status, id);

-- Deliberately no app_fill_tenant_id trigger: an event may arrive before its
-- tenant is resolvable, and the column has to stay 0 in that case. The writer
-- always supplies tenant_id (it runs on the platform identity), and RLS still
-- keeps one tenant from reading another's events.
ALTER TABLE sys_payment_event ENABLE ROW LEVEL SECURITY;
ALTER TABLE sys_payment_event FORCE ROW LEVEL SECURITY;
DROP POLICY IF EXISTS tenant_isolation ON sys_payment_event;
CREATE POLICY tenant_isolation ON sys_payment_event
  USING (app_rls_bypass() OR tenant_id = app_current_tenant_id())
  WITH CHECK (app_rls_bypass() OR tenant_id = app_current_tenant_id());

-- ---------------------------------------------------------------------------
-- 5. Usage metering
-- ---------------------------------------------------------------------------
-- sys_usage_counter already exists (tenant_id, quota_key, period_key). This
-- index serves the "current period" read on the billing page without scanning
-- every period a long-lived tenant ever wrote.
CREATE INDEX IF NOT EXISTS idx_usage_counter_tenant
  ON sys_usage_counter (tenant_id, quota_key, period_key);

-- The seat quota is the one limit that shipped before usage metering, so a
-- tenant created by a plan has a used_value that may disagree with the member
-- count. Re-align it once; later changes are maintained transactionally.
UPDATE sys_tenant_quota q
SET used_value = m.total, updated_at = NOW()
FROM (
  SELECT tenant_id, COUNT(*) AS total
  FROM sys_tenant_member
  WHERE status = 1
  GROUP BY tenant_id
) m
WHERE q.quota_key = 'seats'
  AND q.tenant_id = m.tenant_id
  AND q.used_value <> m.total;
