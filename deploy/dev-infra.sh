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
note "postgres: postgres://dragz_admin:dragz_admin@127.0.0.1:5432/dragz_admin"
note "redis:    127.0.0.1:6379"
note "stop:     bash deploy/dev-infra.sh --stop"
