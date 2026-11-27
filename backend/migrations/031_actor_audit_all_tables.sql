-- 031: every created_at has a created_by, every updated_at has an updated_by.
--
-- 023/024 covered only the tables a person maintains and deliberately excluded
-- logs, queues, relation tables and counters. The audit rule is now uniform:
-- the actor column is part of the timestamp pair, so any table carrying
-- created_at gets created_by and any table carrying updated_at gets updated_by.
-- Tables without a human writer still satisfy the rule: their rows hold 0
-- ("no user context"), which is exactly what the column contract means.
--
-- Column contract, unchanged from 023:
--   * BIGINT NOT NULL DEFAULT 0; 0 = bootstrap, seed, plugin migration,
--     scheduled job, payment callback, request-path counter bump;
--   * no foreign key (0 is a sentinel, users are soft-deleted, platform
--     operators act across tenants);
--   * no trigger: updated_at is stamped by the 022 trigger, but updated_by must
--     not be, otherwise system writes would overwrite the last human editor.
--
-- The loop is intentional: a hand-written list drifted once already. It also
-- covers plugin tables and tables introduced later, and re-running is a no-op.
--
-- The same block sits at the end of store.bootstrapStatements, which is what
-- actually executes on an existing database (there is no migration runner).

BEGIN;

SELECT set_config('app.rls_bypass', 'on', true);

DO $$
DECLARE
  target text;
BEGIN
  FOR target IN
    SELECT c.relname
    FROM pg_class c
    JOIN pg_namespace n ON n.oid = c.relnamespace
    JOIN pg_attribute a ON a.attrelid = c.oid
      AND a.attname = 'created_at' AND a.attnum > 0 AND NOT a.attisdropped
    WHERE n.nspname = current_schema() AND c.relkind = 'r'
      AND NOT EXISTS (
        SELECT 1 FROM pg_attribute b
        WHERE b.attrelid = c.oid
          AND b.attname = 'created_by' AND b.attnum > 0 AND NOT b.attisdropped
      )
  LOOP
    EXECUTE format('ALTER TABLE %I ADD COLUMN created_by BIGINT NOT NULL DEFAULT 0', target);
  END LOOP;

  FOR target IN
    SELECT c.relname
    FROM pg_class c
    JOIN pg_namespace n ON n.oid = c.relnamespace
    JOIN pg_attribute a ON a.attrelid = c.oid
      AND a.attname = 'updated_at' AND a.attnum > 0 AND NOT a.attisdropped
    WHERE n.nspname = current_schema() AND c.relkind = 'r'
      AND NOT EXISTS (
        SELECT 1 FROM pg_attribute b
        WHERE b.attrelid = c.oid
          AND b.attname = 'updated_by' AND b.attnum > 0 AND NOT b.attisdropped
      )
  LOOP
    EXECUTE format('ALTER TABLE %I ADD COLUMN updated_by BIGINT NOT NULL DEFAULT 0', target);
  END LOOP;
END $$;

COMMIT;
