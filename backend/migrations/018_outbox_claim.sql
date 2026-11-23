-- Concurrent outbox delivery.
--
-- The relay used to list pending rows and then deliver them. Two workers doing
-- that at the same time read the same rows and both delivered them, so a
-- webhook received duplicates and the second status update was a silent no-op.
-- The fix is a claim: one UPDATE moves a batch to "processing" with
-- FOR UPDATE SKIP LOCKED, so a competing worker skips those rows instead of
-- waiting for them or repeating them.
--
-- status 3 = processing. The two existing values keep their meaning:
--   0 pending, 1 sent, 2 dead.
BEGIN;

SELECT set_config('app.rls_bypass', 'on', true);

ALTER TABLE sys_outbox ADD COLUMN IF NOT EXISTS claimed_at TIMESTAMPTZ NULL;
ALTER TABLE sys_outbox ADD COLUMN IF NOT EXISTS claimed_by VARCHAR(64) NOT NULL DEFAULT '';

COMMENT ON COLUMN sys_outbox.claimed_at IS
  'When the current worker took the row. ReclaimStaleOutbox returns rows older than the lease.';
COMMENT ON COLUMN sys_outbox.claimed_by IS
  'Worker identity that holds the claim. Empty when the row is not claimed.';

-- The claim query filters on status and orders by id; the existing
-- idx_outbox_pending covers (status, id). The stale-lease sweep filters on
-- claimed_at, which needs its own partial index so it does not scan the table.
CREATE INDEX IF NOT EXISTS idx_outbox_claimed ON sys_outbox (claimed_at)
  WHERE status = 3;

COMMIT;
