-- Unify free-text "note" columns under the name remark.
--
-- sys_position already used remark; these three tables used description for the
-- same concept, which forced code to remember two names. Rename in place so no
-- data is lost and no backfill is needed.
--
-- The bootstrap DDL in internal/store/plugin.go may already have added an empty
-- remark column before this migration runs (it runs on every start, while
-- migrations only run against a fresh volume). When both columns exist the old
-- value wins, then description is dropped. Re-running is a no-op.

DO $$
DECLARE
  target RECORD;
BEGIN
  FOR target IN
    SELECT *
    FROM (VALUES
      ('sys_department', 'description'),
      ('sys_config', 'description'),
      ('sys_plan', 'description')
    ) AS t(table_name, old_column)
  LOOP
    IF EXISTS (
      SELECT 1 FROM information_schema.columns
      WHERE table_schema = current_schema()
        AND table_name::text = target.table_name
        AND column_name = target.old_column
    ) THEN
      IF EXISTS (
        SELECT 1 FROM information_schema.columns
        WHERE table_schema = current_schema()
          AND table_name::text = target.table_name
          AND column_name = 'remark'
      ) THEN
        EXECUTE format(
          'UPDATE %I SET remark = %I WHERE remark = %L AND %I <> %L',
          target.table_name, target.old_column, '', target.old_column, ''
        );
        EXECUTE format('ALTER TABLE %I DROP COLUMN %I', target.table_name, target.old_column);
      ELSE
        EXECUTE format('ALTER TABLE %I RENAME COLUMN %I TO remark', target.table_name, target.old_column);
      END IF;
    END IF;
  END LOOP;
END $$;
