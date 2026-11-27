-- Department free-text note column. Display-only; no business logic branches
-- on it. Migrations/026 renames it to remark on existing databases, matching
-- the name used by sys_position and the rest of the codebase.
--
-- Existing departments keep the default and need no backfill.

ALTER TABLE sys_department ADD COLUMN IF NOT EXISTS description VARCHAR(500) NOT NULL DEFAULT '';
