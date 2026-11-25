-- 021: a dictionary type keeps at most one default item among LIVE rows.
--
-- Two halves, both idempotent and safe to re-run:
--   1. Dedupe: a database written by older builds may hold several
--      is_default=1 rows in one (tenant, type); the unique index below would
--      refuse to build over them. The survivor is the first row in sort order
--      (id breaks ties), which is what the UI has always rendered as the
--      default. New installs from 001 have no violations, so this is a no-op
--      there.
--   2. The partial unique index itself. It is the concurrency guard for the
--      application's two-step save (clear the type's other defaults first,
--      then set the new one): two concurrent saves cannot both win, and the
--      loser surfaces as a duplicate-key 409.

BEGIN;

SELECT set_config('app.rls_bypass', 'on', true);

UPDATE sys_dict_item SET is_default = 0, updated_at = NOW()
WHERE is_default = 1 AND deleted_at IS NULL
  AND id <> (
    SELECT keep.id FROM sys_dict_item keep
    WHERE keep.tenant_id = sys_dict_item.tenant_id
      AND keep.dict_type_id = sys_dict_item.dict_type_id
      AND keep.is_default = 1 AND keep.deleted_at IS NULL
    ORDER BY keep.sort, keep.id
    LIMIT 1
  );

CREATE UNIQUE INDEX IF NOT EXISTS uk_dict_item_default
  ON sys_dict_item (tenant_id, dict_type_id)
  WHERE is_default = 1 AND deleted_at IS NULL;

COMMIT;
