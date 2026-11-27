-- Actor audit columns for the notify configuration tables.
--
-- Same convention as the host's 023/024 migrations: BIGINT NOT NULL
-- DEFAULT 0, no foreign key, no trigger. The plugin writes them from the
-- request context on create/edit/delete.
--
-- notify_log is left alone: a delivery row is written by the sender (system,
-- or a scheduled job), not by a person, so there is no meaningful author.

ALTER TABLE notify_channel ADD COLUMN IF NOT EXISTS created_by BIGINT NOT NULL DEFAULT 0;
ALTER TABLE notify_channel ADD COLUMN IF NOT EXISTS updated_by BIGINT NOT NULL DEFAULT 0;

ALTER TABLE notify_template ADD COLUMN IF NOT EXISTS created_by BIGINT NOT NULL DEFAULT 0;
ALTER TABLE notify_template ADD COLUMN IF NOT EXISTS updated_by BIGINT NOT NULL DEFAULT 0;
