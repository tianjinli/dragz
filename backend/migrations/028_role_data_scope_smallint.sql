-- sys_role.data_scope changes from a VARCHAR label ('all', 'self', ...) to a
-- SMALLINT enum, so one value has one representation instead of a string each
-- caller has to spell.
--
--   0 ALL                全部数据
--   1 SELF               仅本人数据
--   2 DEPT               本部门数据
--   3 DEPT_AND_CHILDREN  本部门及以下数据
--   4 CUSTOM             自定义部门数据
--
-- Idempotent: a column already typed smallint is left alone and only has its
-- default normalised. Re-runnable, like every other migration file.

BEGIN;

SELECT set_config('app.rls_bypass', 'on', true);

DO $$
DECLARE
  current_type TEXT;
BEGIN
  SELECT data_type INTO current_type
  FROM information_schema.columns
  WHERE table_schema = current_schema()
    AND table_name = 'sys_role'
    AND column_name = 'data_scope';

  IF current_type = 'character varying' THEN
    EXECUTE 'ALTER TABLE sys_role ALTER COLUMN data_scope DROP DEFAULT';
    EXECUTE 'ALTER TABLE sys_role ALTER COLUMN data_scope TYPE SMALLINT USING CASE data_scope '
         || 'WHEN ''all'' THEN 0 '
         || 'WHEN ''self'' THEN 1 '
         || 'WHEN ''department'' THEN 2 '
         || 'WHEN ''department_and_children'' THEN 3 '
         || 'WHEN ''custom'' THEN 4 '
         || 'ELSE 1 END';
    EXECUTE 'ALTER TABLE sys_role ALTER COLUMN data_scope SET DEFAULT 1';
  ELSIF current_type IS NOT NULL THEN
    EXECUTE 'ALTER TABLE sys_role ALTER COLUMN data_scope SET DEFAULT 1';
  END IF;
END $$;

COMMIT;
