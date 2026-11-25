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
--
-- REASSIGN OWNED is deliberately not used: when the cluster was bootstrapped
-- with a non-postgres superuser, that role also owns pg_catalog objects, and
-- REASSIGN refuses to move them ("required by the database system"). Moving
-- only the public schema is what the migration role actually needs.
DO $$
DECLARE
  r record;
BEGIN
  FOR r IN
    SELECT c.relname, c.relkind
    FROM pg_class c
    JOIN pg_namespace n ON n.oid = c.relnamespace
    WHERE n.nspname = 'public'
      AND c.relkind IN ('r', 'p', 'v', 'm')
      AND c.relowner <> (SELECT oid FROM pg_roles WHERE rolname = 'dragz_migrate')
  LOOP
    CASE r.relkind
      WHEN 'r', 'p' THEN EXECUTE format('ALTER TABLE public.%I OWNER TO dragz_migrate', r.relname);
      WHEN 'v' THEN EXECUTE format('ALTER VIEW public.%I OWNER TO dragz_migrate', r.relname);
      WHEN 'm' THEN EXECUTE format('ALTER MATERIALIZED VIEW public.%I OWNER TO dragz_migrate', r.relname);
    END CASE;
  END LOOP;
  -- Sequences are skipped on purpose: a sequence owned by a table column
  -- cannot be reassigned on its own, and ALTER TABLE above already moves it.

  FOR r IN
    SELECT p.oid::regprocedure AS sig
    FROM pg_proc p
    JOIN pg_namespace n ON n.oid = p.pronamespace
    WHERE n.nspname = 'public'
      AND p.proowner <> (SELECT oid FROM pg_roles WHERE rolname = 'dragz_migrate')
  LOOP
    EXECUTE format('ALTER FUNCTION %s OWNER TO dragz_migrate', r.sig);
  END LOOP;
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
