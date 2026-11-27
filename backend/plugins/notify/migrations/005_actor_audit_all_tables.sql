-- The audit rule is now uniform: created_at pairs with created_by and
-- updated_at pairs with updated_by (see the host's
-- migrations/030_actor_audit_all_tables.sql). notify_channel and
-- notify_template gained their columns in 003; notify_log still needs them.
--
-- notify_log rows are written by the sender (system or a scheduled job), so
-- both columns stay 0. They exist so every timestamped table has the same
-- shape and a future manual re-send can record its actor without a migration.

ALTER TABLE notify_log ADD COLUMN IF NOT EXISTS created_by BIGINT NOT NULL DEFAULT 0;
ALTER TABLE notify_log ADD COLUMN IF NOT EXISTS updated_by BIGINT NOT NULL DEFAULT 0;
