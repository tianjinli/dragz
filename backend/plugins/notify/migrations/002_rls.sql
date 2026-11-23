-- Row level security for the three notify tables. Mirrors the core tables:
-- tenant rows are visible only inside their own tenant session, and the
-- fill trigger stamps tenant_id from app.tenant_id on insert.
--
-- Requires the host multitenant schema (app_rls_bypass,
-- app_current_tenant_id, app_fill_tenant_id); installed by the core migration.

DO $$
DECLARE t TEXT;
BEGIN
  FOREACH t IN ARRAY ARRAY['notify_channel', 'notify_template', 'notify_log'] LOOP
    EXECUTE format('ALTER TABLE %I ENABLE ROW LEVEL SECURITY', t);
    EXECUTE format('ALTER TABLE %I FORCE ROW LEVEL SECURITY', t);
    EXECUTE format('DROP POLICY IF EXISTS tenant_isolation ON %I', t);
    EXECUTE format(
      'CREATE POLICY tenant_isolation ON %I USING (app_rls_bypass() OR tenant_id = app_current_tenant_id()) WITH CHECK (app_rls_bypass() OR tenant_id = app_current_tenant_id())',
      t);
    EXECUTE format('DROP TRIGGER IF EXISTS trg_fill_tenant_id ON %I', t);
    EXECUTE format(
      'CREATE TRIGGER trg_fill_tenant_id BEFORE INSERT ON %I FOR EACH ROW EXECUTE FUNCTION app_fill_tenant_id()',
      t);
  END LOOP;
END
$$;
