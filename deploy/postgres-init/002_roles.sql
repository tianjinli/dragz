-- Three database identities, one job each.
--
-- The stack used to connect as POSTGRES_USER, which the postgres image creates
-- as a superuser. A superuser is exempt from row-level security even with FORCE
-- ROW LEVEL SECURITY, so tenant isolation was configured but never enforced.
-- These roles are what make 013_multitenant.sql's policies effective:
--
--   dragz_app       application runtime. No DDL, no BYPASSRLS.
--                   Every request runs here, so every tenant table is filtered.
--   dragz_migrate   schema owner. Runs migrations; owns the tables so it can
--                   evolve them. Still no BYPASSRLS.
--   dragz_platform  cross-tenant reads for the platform console, the outbox
--                   relay and the token lookup that runs before a tenant is
--                   known. The only role with BYPASSRLS.
--
-- Production must replace the passwords below. They are development defaults
-- because /docker-entrypoint-initdb.d scripts run without access to the
-- container environment.
--
-- Idempotent: safe to run against an existing database with
--   psql "$DSN" -f deploy/postgres-init/002_roles.sql

\set ON_ERROR_STOP on

DO $$
BEGIN
  IF NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'dragz_app') THEN
    CREATE ROLE dragz_app LOGIN PASSWORD 'dragz_app';
  END IF;
  IF NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'dragz_migrate') THEN
    CREATE ROLE dragz_migrate LOGIN PASSWORD 'dragz_migrate';
  END IF;
  IF NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'dragz_platform') THEN
    CREATE ROLE dragz_platform LOGIN PASSWORD 'dragz_platform';
  END IF;
END
$$;

-- Re-assert the attributes on every run: an upgraded cluster may have been
-- hand-edited, and a stray BYPASSRLS on dragz_app would reopen the hole.
ALTER ROLE dragz_app NOSUPERUSER NOBYPASSRLS NOCREATEDB NOCREATEROLE;
ALTER ROLE dragz_migrate NOSUPERUSER NOBYPASSRLS NOCREATEDB NOCREATEROLE;
ALTER ROLE dragz_platform NOSUPERUSER BYPASSRLS NOCREATEDB NOCREATEROLE;

-- Hand the schema to the migration role. Bootstrap tables are owned by the
-- initdb superuser; without this, migrations would have to run as a superuser.
DO $$
DECLARE
  bootstrap_role TEXT := current_user;
BEGIN
  IF bootstrap_role <> 'dragz_migrate' THEN
    EXECUTE format('REASSIGN OWNED BY %I TO dragz_migrate', bootstrap_role);
  END IF;
END
$$;

ALTER SCHEMA public OWNER TO dragz_migrate;
GRANT CREATE, USAGE ON SCHEMA public TO dragz_migrate;

-- The application role may read and write rows, never change the schema.
GRANT USAGE ON SCHEMA public TO dragz_app, dragz_platform;
GRANT SELECT, INSERT, UPDATE, DELETE ON ALL TABLES IN SCHEMA public TO dragz_app, dragz_platform;
GRANT USAGE, SELECT ON ALL SEQUENCES IN SCHEMA public TO dragz_app, dragz_platform;

-- Migrations run as dragz_migrate, so tables they create must be reachable by
-- the runtime roles without a second grant.
ALTER DEFAULT PRIVILEGES FOR ROLE dragz_migrate IN SCHEMA public
  GRANT SELECT, INSERT, UPDATE, DELETE ON TABLES TO dragz_app, dragz_platform;
ALTER DEFAULT PRIVILEGES FOR ROLE dragz_migrate IN SCHEMA public
  GRANT USAGE, SELECT ON SEQUENCES TO dragz_app, dragz_platform;
