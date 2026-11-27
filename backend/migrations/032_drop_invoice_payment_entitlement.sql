-- Drop unused billing skeleton and never-written tenant entitlement override.
-- Menus that pointed at invoice screens are soft-deleted; historical
-- migrations that created these objects are left as-is for already-applied DBs.

DROP TABLE IF EXISTS sys_payment_event CASCADE;
DROP TABLE IF EXISTS sys_invoice CASCADE;
DROP TABLE IF EXISTS sys_tenant_entitlement CASCADE;

UPDATE sys_menu
SET deleted_at = NOW(), updated_at = NOW()
WHERE code = 'platform_invoices' AND deleted_at IS NULL;

DELETE FROM sys_translation
WHERE entity_key = 'menu.platform_invoices';
