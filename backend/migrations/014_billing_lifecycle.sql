-- Subscription lifecycle columns, invoice skeleton, and tenant purge markers.
-- Idempotent: safe to run again on a database that already has these objects.
-- FORCE ROW LEVEL SECURITY applies to the table owner, so this script turns
-- app.rls_bypass on for the session. Bootstrap does the same on every startup.
BEGIN;

SELECT set_config('app.rls_bypass', 'on', true);

ALTER TABLE sys_tenant_subscription ADD COLUMN IF NOT EXISTS trial_ends_at TIMESTAMPTZ NULL;
ALTER TABLE sys_tenant_subscription ADD COLUMN IF NOT EXISTS grace_ends_at TIMESTAMPTZ NULL;

ALTER TABLE sys_tenant ADD COLUMN IF NOT EXISTS purge_after TIMESTAMPTZ NULL;
ALTER TABLE sys_tenant ADD COLUMN IF NOT EXISTS purge_status VARCHAR(32) NOT NULL DEFAULT '';

CREATE TABLE IF NOT EXISTS sys_invoice (
  id BIGSERIAL,
  tenant_id BIGINT NOT NULL,
  subscription_id BIGINT NULL,
  kind VARCHAR(32) NOT NULL,
  status VARCHAR(32) NOT NULL DEFAULT 'draft',
  amount_cents BIGINT NOT NULL DEFAULT 0,
  currency CHAR(3) NOT NULL DEFAULT 'CNY',
  period_start TIMESTAMPTZ NULL,
  period_end TIMESTAMPTZ NULL,
  memo VARCHAR(500) NOT NULL DEFAULT '',
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  PRIMARY KEY (id)
);

CREATE INDEX IF NOT EXISTS idx_invoice_tenant_id ON sys_invoice (tenant_id, id DESC);

DROP TRIGGER IF EXISTS trg_fill_tenant_id ON sys_invoice;
CREATE TRIGGER trg_fill_tenant_id
  BEFORE INSERT ON sys_invoice
  FOR EACH ROW EXECUTE FUNCTION app_fill_tenant_id();

ALTER TABLE sys_invoice ENABLE ROW LEVEL SECURITY;
ALTER TABLE sys_invoice FORCE ROW LEVEL SECURITY;
DROP POLICY IF EXISTS tenant_isolation ON sys_invoice;
CREATE POLICY tenant_isolation ON sys_invoice
  USING (app_rls_bypass() OR tenant_id = app_current_tenant_id())
  WITH CHECK (app_rls_bypass() OR tenant_id = app_current_tenant_id());

COMMIT;
