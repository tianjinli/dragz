-- Drop the DEFAULT 1 added by 002 so app_fill_tenant_id() stamps the tenant
-- from the request session. Earlier migration files are checksum-verified.
ALTER TABLE sys_example_item ALTER COLUMN tenant_id DROP DEFAULT;
