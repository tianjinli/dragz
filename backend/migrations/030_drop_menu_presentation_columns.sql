-- Drop menu columns that no longer carry meaning.
--
-- is_sensitive was derived from the HTTP method at sync time but never read:
-- enforcement is by permission code, and the admin UI never showed it.
-- external_url had no writer at all (the UI offers no external menu type and
-- plugin declarations do not carry the field), and the frontend never consumed
-- the meta.link it produced.
-- visible / keep_alive / affix_tab were only ever written by seed data and the
-- plugin declaration, never editable in the UI, and UpdateMenu rewrote them
-- from empty form values; removing them also removes the hide-in-menu and
-- per-menu tab pinning behaviours.
--
-- Repeated at the top of bootstrapStatements because a fresh database only
-- runs 001_system_init.sql at image build time.
BEGIN;

SELECT set_config('app.rls_bypass', 'on', true);

ALTER TABLE sys_menu
  DROP COLUMN IF EXISTS is_sensitive,
  DROP COLUMN IF EXISTS external_url,
  DROP COLUMN IF EXISTS affix_tab,
  DROP COLUMN IF EXISTS keep_alive,
  DROP COLUMN IF EXISTS visible;

COMMIT;
