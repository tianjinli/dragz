-- Refresh tokens remember the tenant they were issued in.
--
-- 013_multitenant.sql dropped sys_refresh_token.tenant_id on the assumption that
-- the tenant could be re-derived from the user's memberships. It cannot: a user
-- who belongs to two tenants switches to the second one, and the next refresh
-- silently puts them back in the first (whichever the fallback picks). Every
-- request after a refresh then runs in the wrong tenant — which is visible as
-- data that "disappears" after the access token expires.
--
-- The column is nullable on purpose: tokens issued before this migration have
-- no recorded tenant, so the refresh path falls back to the old behaviour for
-- them and uses the stored tenant for everything issued afterwards.
BEGIN;

SELECT set_config('app.rls_bypass', 'on', true);

ALTER TABLE sys_refresh_token ADD COLUMN IF NOT EXISTS tenant_id BIGINT NULL;

COMMENT ON COLUMN sys_refresh_token.tenant_id IS
  'Tenant the session was opened in. NULL only for tokens issued before 017.';

-- Session listing and revocation are per user, not per tenant, so no index on
-- tenant_id alone; the tenant is read together with the token hash.
CREATE INDEX IF NOT EXISTS idx_refresh_token_tenant ON sys_refresh_token (tenant_id)
  WHERE tenant_id IS NOT NULL;

COMMIT;
