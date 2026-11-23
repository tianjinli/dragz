-- Host → tenant. slug is the label in {slug}.{base domain}.
-- sys_tenant_domain stores custom hosts. It has no row-level security:
-- lookup runs before a tenant is selected and must stay readable.
-- Idempotent: safe to run again on a database that already has these objects.
-- FORCE ROW LEVEL SECURITY applies to the table owner, so this script turns
-- app.rls_bypass on for the session. Bootstrap does the same on every startup.
BEGIN;

SELECT set_config('app.rls_bypass', 'on', true);

ALTER TABLE sys_tenant ADD COLUMN IF NOT EXISTS slug VARCHAR(64) NOT NULL DEFAULT '';

-- id 1 is the built-in tenant. Other empty slugs come from lower(code) with
-- everything outside [a-z0-9-] removed. Collisions and labels that are too
-- short become t{id}, which is unique.
DO $$
DECLARE
  rec record;
  candidate text;
BEGIN
  UPDATE sys_tenant
  SET slug = 'default', updated_at = NOW()
  WHERE id = 1 AND slug = '';

  FOR rec IN
    SELECT id, code FROM sys_tenant
    WHERE id <> 1 AND slug = '' AND deleted_at IS NULL
    ORDER BY id
  LOOP
    candidate := left(regexp_replace(lower(rec.code), '[^a-z0-9-]', '', 'g'), 63);
    IF candidate !~ '^[a-z][a-z0-9-]{1,62}$'
       OR EXISTS (
         SELECT 1 FROM sys_tenant
         WHERE deleted_at IS NULL AND slug = candidate AND id <> rec.id
       )
    THEN
      candidate := 't' || rec.id::text;
    END IF;
    IF EXISTS (
      SELECT 1 FROM sys_tenant
      WHERE deleted_at IS NULL AND slug = candidate AND id <> rec.id
    ) THEN
      candidate := 't' || rec.id::text || 'x';
    END IF;
    UPDATE sys_tenant
    SET slug = candidate, updated_at = NOW()
    WHERE id = rec.id AND slug = '';
  END LOOP;
END
$$;

CREATE UNIQUE INDEX IF NOT EXISTS uk_tenant_slug
  ON sys_tenant (slug) WHERE deleted_at IS NULL AND slug <> '';

CREATE TABLE IF NOT EXISTS sys_tenant_domain (
  id BIGSERIAL PRIMARY KEY,
  tenant_id BIGINT NOT NULL,
  host VARCHAR(255) NOT NULL,
  verified_at TIMESTAMPTZ NULL,
  is_primary SMALLINT NOT NULL DEFAULT 0,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX IF NOT EXISTS uk_tenant_domain_host ON sys_tenant_domain (host);
CREATE INDEX IF NOT EXISTS idx_tenant_domain_tenant ON sys_tenant_domain (tenant_id);

COMMIT;
