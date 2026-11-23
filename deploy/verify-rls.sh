#!/usr/bin/env bash
# Real PostgreSQL acceptance for tenant isolation.
#
# Row-level security cannot be verified against a superuser: PostgreSQL exempts
# superusers and BYPASSRLS roles even when a table says FORCE ROW LEVEL SECURITY.
# This script therefore builds a disposable stack that mirrors production —
# a bootstrap superuser, a schema-owning migration role, a BYPASSRLS platform
# role, and a plain application role — and runs the two isolation tests against
# the application role.
#
#   bash deploy/verify-rls.sh
#
# Requires Docker. Everything runs in a throwaway container and is removed on
# exit, including on failure.
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
CONTAINER="dragz-rls-verify-$$"
PORT="${DRAGZ_VERIFY_PORT:-55432}"
POSTGRES_IMAGE="${DRAGZ_VERIFY_IMAGE:-postgres:16-alpine}"

cleanup() {
  docker rm -f "$CONTAINER" >/dev/null 2>&1 || true
}
trap cleanup EXIT

echo "==> starting disposable postgres ($POSTGRES_IMAGE)"
docker run -d --name "$CONTAINER" \
  -e POSTGRES_PASSWORD=verify \
  -e POSTGRES_USER=dragz_bootstrap \
  -e POSTGRES_DB=dragz_verify \
  -p "127.0.0.1:${PORT}:5432" \
  "$POSTGRES_IMAGE" >/dev/null

echo "==> waiting for readiness"
for _ in $(seq 1 60); do
  if docker exec "$CONTAINER" pg_isready -U dragz_bootstrap -d dragz_verify >/dev/null 2>&1; then
    break
  fi
  sleep 1
done
docker exec "$CONTAINER" pg_isready -U dragz_bootstrap -d dragz_verify >/dev/null

psql_in() {
  docker exec -i "$CONTAINER" psql -v ON_ERROR_STOP=1 -U dragz_bootstrap -d dragz_verify "$@"
}

echo "==> creating the three runtime roles"
psql_in -q -c "
CREATE ROLE dragz_app LOGIN PASSWORD 'dragz_app';
CREATE ROLE dragz_migrate LOGIN PASSWORD 'dragz_migrate';
CREATE ROLE dragz_platform LOGIN PASSWORD 'dragz_platform';
ALTER ROLE dragz_app NOSUPERUSER NOBYPASSRLS NOCREATEDB NOCREATEROLE;
ALTER ROLE dragz_migrate NOSUPERUSER NOBYPASSRLS NOCREATEDB NOCREATEROLE;
ALTER ROLE dragz_platform NOSUPERUSER BYPASSRLS NOCREATEDB NOCREATEROLE;
GRANT USAGE ON SCHEMA public TO dragz_app, dragz_migrate, dragz_platform;
-- The migration role owns the schema it evolves.
GRANT CREATE ON SCHEMA public TO dragz_migrate;
"

echo "==> applying migrations as dragz_migrate"
# Applied in lexical order, exactly like the runtime migrator. Each file is
# written to be re-runnable, so the loop needs no bookkeeping table.
for file in "$ROOT"/backend/migrations/*.sql; do
  echo "    $(basename "$file")"
  docker exec -i "$CONTAINER" psql -v ON_ERROR_STOP=1 -q \
    -U dragz_migrate -d dragz_verify <"$file"
done

echo "==> granting runtime privileges"
psql_in -q -c "
GRANT SELECT, INSERT, UPDATE, DELETE ON ALL TABLES IN SCHEMA public TO dragz_app, dragz_platform;
GRANT USAGE, SELECT ON ALL SEQUENCES IN SCHEMA public TO dragz_app, dragz_platform;
-- The predicate test builds its own probe schema. CREATE on the database is
-- what a schema needs; it grants no access to any existing table.
GRANT CREATE ON DATABASE dragz_verify TO dragz_app;
"

APP_DSN="postgres://dragz_app:dragz_app@127.0.0.1:${PORT}/dragz_verify?sslmode=disable"
PLATFORM_DSN="postgres://dragz_platform:dragz_platform@127.0.0.1:${PORT}/dragz_verify?sslmode=disable"

echo "==> running isolation tests as dragz_app"
DRAGZ_VERIFY_DSN="$APP_DSN" \
DRAGZ_VERIFY_PLATFORM_DSN="$PLATFORM_DSN" \
  go -C "$ROOT/backend" test ./internal/store \
    -run 'TestTenantRLS' -v -count=1

echo "==> done: tenant isolation holds on a real PostgreSQL"
