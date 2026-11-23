-- 020: rename the platform invoices menu to "账单管理".
--
-- The menu name is stored in sys_menu.name and shown verbatim in the sidebar;
-- it is not translated at runtime. Renaming it here keeps existing installs in
-- sync with the seed (which new installs pick up). Also refresh the English
-- translation row used by the translation-management screen. Both statements
-- are idempotent and safe to re-run.

BEGIN;

SELECT set_config('app.rls_bypass', 'on', true);

UPDATE sys_menu
SET name = '账单管理', updated_at = NOW()
WHERE code = 'platform_invoices' AND deleted_at IS NULL AND name <> '账单管理';

UPDATE sys_translation
SET value = 'Invoice Management', updated_at = NOW()
WHERE entity_key = 'menu.platform_invoices' AND lang = 'en' AND value <> 'Invoice Management';

COMMIT;
