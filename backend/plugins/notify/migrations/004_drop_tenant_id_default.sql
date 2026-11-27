-- tenant_id must have no column default, mirroring core migrations/028.
-- The app_fill_tenant_id trigger installed by 002 only fills NULL/0, so a
-- DEFAULT 1 would silently route inserts for other tenants into tenant 1.
-- An applied migration's checksum is verified, so 001_init.sql is left intact.
DO $$
DECLARE
  target TEXT;
BEGIN
  FOREACH target IN ARRAY ARRAY['notify_channel', 'notify_template', 'notify_log']
  LOOP
    EXECUTE format('ALTER TABLE %I ALTER COLUMN tenant_id DROP DEFAULT', target);
  END LOOP;
END $$;
