-- tenant_id must have no column default.
--
-- app_fill_tenant_id() only replaces NULL or 0 with the tenant from the
-- session. A column default of 1 short-circuits that trigger: an INSERT that
-- omits tenant_id lands in tenant 1 even when the request belongs to another
-- tenant. Dropping the default makes every write state its tenant explicitly.
--
-- The same conversion is repeated at the top of bootstrapStatements because
-- a fresh database only runs 001_system_init.sql at image build time.
BEGIN;

DO $$
DECLARE
  target TEXT;
BEGIN
  FOREACH target IN ARRAY ARRAY[
    'sys_department', 'sys_position', 'sys_user_position', 'sys_role',
    'sys_user_role', 'sys_role_department', 'sys_role_menu', 'sys_dict_type',
    'sys_dict_item', 'sys_file', 'sys_file_reference', 'sys_notice',
    'sys_notice_read', 'sys_notification', 'sys_user_notify_pref', 'sys_api_token',
    'sys_operation_log', 'sys_error_log', 'sys_webhook', 'sys_scheduled_job',
    'sys_job_log', 'sys_outbox'
  ]
  LOOP
    IF EXISTS (
      SELECT 1 FROM information_schema.columns
      WHERE table_schema = current_schema()
        AND table_name = target AND column_name = 'tenant_id'
    ) THEN
      EXECUTE format('ALTER TABLE %I ALTER COLUMN tenant_id DROP DEFAULT', target);
    END IF;
  END LOOP;
END $$;

COMMIT;
