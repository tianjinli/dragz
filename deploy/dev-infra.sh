#!/usr/bin/env bash
# One-click: start only the infrastructure (postgres + redis) so the Go
# services can run on the host with `go run` and hot-reload.
#
#   bash deploy/dev-infra.sh          # start postgres + redis
#   bash deploy/dev-infra.sh --stop   # stop them
source "$(dirname "${BASH_SOURCE[0]}")/lib.sh"

require_docker

if [ "${1:-}" = "--stop" ]; then
  log "stopping infrastructure"
  run_compose "${INFRA_FILES[@]}" -- down
  ok "infrastructure stopped"
  exit 0
fi

log "starting postgres + redis"
run_compose "${INFRA_FILES[@]}" -- up -d

printf '    waiting for postgres '
for _ in $(seq 1 60); do
  if run_compose "${INFRA_FILES[@]}" -- exec -T postgres pg_isready -U dragz_admin -d dragz_admin >/dev/null 2>&1; then
    printf ' %sready%s\n' "$C_GRN" "$C_RST"
    break
  fi
  printf '.'
  sleep 2
done

echo
log "infrastructure is up — run the services on the host:"
note "cd backend && go run ./cmd/admin-api -f etc/admin-api.yaml"
note "cd backend && go run ./cmd/jobs-runner -f etc/jobs-runner.yaml"
echo
# The API connects as dragz_app (no BYPASSRLS) so row-level security is
# enforced; dragz_admin is the initdb superuser and is only used to
# administer the database. See deploy/postgres-init/002_roles.sql.
note "app:      postgres://dragz_app:dragz_app@127.0.0.1:5432/dragz_admin"
note "migrate:  postgres://dragz_migrate:dragz_migrate@127.0.0.1:5432/dragz_admin"
note "platform: postgres://dragz_platform:dragz_platform@127.0.0.1:5432/dragz_admin"
note "redis:    127.0.0.1:6379"
note "stop:     bash deploy/dev-infra.sh --stop"
echo
# A volume created before 002_roles.sql existed has only dragz_admin, and a
# superuser silently disables every RLS policy. initdb scripts never re-run, so
# the roles have to be created once by hand.
note "existing volume without the three roles? run:"
note "  docker compose -f deploy/docker-compose.infra.yml exec -T postgres \\"
note "    psql -U dragz_admin -d dragz_admin < deploy/postgres-init/002_roles.sql"
