-- Enable row level security on sys_example_item.
-- 001_init.sql and 002_tenant.sql are immutable: their checksums are verified
-- on every enable. 002 added tenant_id; this file applies the core policy.
--
-- Plugin enable requires the host multitenant schema. app_rls_bypass,
-- app_current_tenant_id, and app_fill_tenant_id are installed by the core
-- multitenant migration and are not created here.

ALTER TABLE sys_example_item ENABLE ROW LEVEL SECURITY;
ALTER TABLE sys_example_item FORCE ROW LEVEL SECURITY;
DROP POLICY IF EXISTS tenant_isolation ON sys_example_item;
CREATE POLICY tenant_isolation ON sys_example_item
  USING (app_rls_bypass() OR tenant_id = app_current_tenant_id())
  WITH CHECK (app_rls_bypass() OR tenant_id = app_current_tenant_id());
DROP TRIGGER IF EXISTS trg_fill_tenant_id ON sys_example_item;
CREATE TRIGGER trg_fill_tenant_id BEFORE INSERT ON sys_example_item
  FOR EACH ROW EXECUTE FUNCTION app_fill_tenant_id();
