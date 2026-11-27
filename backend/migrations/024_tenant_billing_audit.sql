-- Actor audit columns for the tenant, billing catalogue and membership tables.
--
-- Same convention as 023_actor_audit_columns.sql:
--   * BIGINT NOT NULL DEFAULT 0, where 0 means "no user context";
--   * no foreign key (0 is a sentinel, users are soft-deleted, platform
--     operators act across tenants);
--   * no trigger, because system writes must not overwrite the last editor.
--
-- Who writes 0 here:
--   sys_tenant            signup / provisionTenant / purge and subscription
--                         transitions performed by the billing job
--   sys_tenant_domain     nothing: every write comes from a console action
--   sys_tenant_member     signup, invite acceptance and bootstrap seeding
--   sys_plan              the free plan seeded at bootstrap
--   sys_plan_entitlement  the free plan's seeded entitlements
--
-- No index: both columns are display-only until a screen filters by them.

ALTER TABLE sys_tenant ADD COLUMN IF NOT EXISTS created_by BIGINT NOT NULL DEFAULT 0;
ALTER TABLE sys_tenant ADD COLUMN IF NOT EXISTS updated_by BIGINT NOT NULL DEFAULT 0;

ALTER TABLE sys_tenant_domain ADD COLUMN IF NOT EXISTS created_by BIGINT NOT NULL DEFAULT 0;
ALTER TABLE sys_tenant_domain ADD COLUMN IF NOT EXISTS updated_by BIGINT NOT NULL DEFAULT 0;

ALTER TABLE sys_tenant_member ADD COLUMN IF NOT EXISTS created_by BIGINT NOT NULL DEFAULT 0;
ALTER TABLE sys_tenant_member ADD COLUMN IF NOT EXISTS updated_by BIGINT NOT NULL DEFAULT 0;

ALTER TABLE sys_plan ADD COLUMN IF NOT EXISTS created_by BIGINT NOT NULL DEFAULT 0;
ALTER TABLE sys_plan ADD COLUMN IF NOT EXISTS updated_by BIGINT NOT NULL DEFAULT 0;

ALTER TABLE sys_plan_entitlement ADD COLUMN IF NOT EXISTS created_by BIGINT NOT NULL DEFAULT 0;
ALTER TABLE sys_plan_entitlement ADD COLUMN IF NOT EXISTS updated_by BIGINT NOT NULL DEFAULT 0;

-- A plan edit rewrites its entitlement rows, so the editor is also the author
-- of the replacement set. Existing rows predate the column and stay 0.
