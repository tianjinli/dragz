-- Boolean flags should be BOOLEAN, not SMALLINT 0/1. Repeating a flag as a
-- number forced every caller to remember two representations and made SQL
-- predicates inconsistent (`= 1` vs `= TRUE`).
--
-- Idempotent: only columns still typed smallint are converted; defaults are
-- then normalised to FALSE. Already-boolean flags (sys_plugin.is_builtin,
-- sys_tenant_member.is_platform_operator) are untouched.
--
-- The partial unique index on sys_dict_item uses is_default in its WHERE
-- clause, so it has to be dropped before the type change and rebuilt with a
-- boolean predicate afterwards. Dedupe first, same rule as migrations/021.

BEGIN;

SELECT set_config('app.rls_bypass', 'on', true);

DROP INDEX IF EXISTS uk_dict_item_default;

DO $$
DECLARE
  target RECORD;
  current_type TEXT;
BEGIN
  FOR target IN
    SELECT *
    FROM (VALUES
      ('sys_user', 'is_super_admin'),
      ('sys_role', 'is_system'),
      ('sys_menu', 'is_sensitive'),
      ('sys_dict_type', 'is_system'),
      ('sys_dict_item', 'is_default'),
      ('sys_config', 'is_secret'),
      ('sys_config', 'is_readonly'),
      ('sys_storage_config', 'is_default'),
      ('sys_notice', 'is_pinned'),
      ('sys_login_log', 'is_suspicious'),
      ('sys_tenant_domain', 'is_primary'),
      ('sys_tenant_member', 'is_tenant_admin'),
      ('sys_tenant_invite', 'is_tenant_admin')
    ) AS t(table_name, column_name)
  LOOP
    SELECT data_type INTO current_type
    FROM information_schema.columns
    WHERE table_schema = current_schema()
      AND table_name::text = target.table_name
      AND column_name = target.column_name;

    -- 022 added ck_<table>_<column> CHECK (col IN (0,1)); it blocks the type
    -- change and is redundant once the column is BOOLEAN.
    EXECUTE format('ALTER TABLE %I DROP CONSTRAINT IF EXISTS %I',
                   target.table_name, 'ck_' || target.table_name || '_' || target.column_name);

    IF current_type = 'smallint' THEN
      -- The old DEFAULT 0 cannot be cast implicitly to boolean.
      EXECUTE format('ALTER TABLE %I ALTER COLUMN %I DROP DEFAULT',
                     target.table_name, target.column_name);
      EXECUTE format(
        'ALTER TABLE %I ALTER COLUMN %I TYPE BOOLEAN USING %I <> 0',
        target.table_name, target.column_name, target.column_name
      );
    END IF;

    IF current_type IS NOT NULL THEN
      EXECUTE format(
        'ALTER TABLE %I ALTER COLUMN %I SET DEFAULT FALSE',
        target.table_name, target.column_name
      );
    END IF;
  END LOOP;
END $$;

UPDATE sys_dict_item SET is_default = FALSE, updated_at = NOW()
WHERE is_default = TRUE AND deleted_at IS NULL
  AND id <> (
    SELECT keep.id FROM sys_dict_item keep
    WHERE keep.tenant_id = sys_dict_item.tenant_id
      AND keep.dict_type_id = sys_dict_item.dict_type_id
      AND keep.is_default = TRUE AND keep.deleted_at IS NULL
    ORDER BY keep.sort, keep.id
    LIMIT 1
  );

CREATE UNIQUE INDEX uk_dict_item_default
  ON sys_dict_item (tenant_id, dict_type_id)
  WHERE is_default = TRUE AND deleted_at IS NULL;

COMMIT;
