-- Add tenant_id so sys_example_item follows the core tables.
-- 001_init.sql is immutable: its checksum is verified on every enable.
--
-- Enable row level security the same way the core tables do, once this
-- column exists and app_current_tenant_id / app_rls_bypass are installed:
--   ALTER TABLE sys_example_item ENABLE ROW LEVEL SECURITY;
--   ALTER TABLE sys_example_item FORCE ROW LEVEL SECURITY;
--   DROP POLICY IF EXISTS tenant_isolation ON sys_example_item;
--   CREATE POLICY tenant_isolation ON sys_example_item
--     USING (app_rls_bypass() OR tenant_id = app_current_tenant_id())
--     WITH CHECK (app_rls_bypass() OR tenant_id = app_current_tenant_id());

ALTER TABLE sys_example_item
  ADD COLUMN IF NOT EXISTS tenant_id BIGINT NOT NULL DEFAULT 1;
