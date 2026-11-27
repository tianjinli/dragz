-- sys_example_item carries created_at, so the uniform audit rule
-- (created_at -> created_by) applies here too. The demo rows are seeded, so 0
-- ("no user context") is the expected value.

ALTER TABLE sys_example_item ADD COLUMN IF NOT EXISTS created_by BIGINT NOT NULL DEFAULT 0;
