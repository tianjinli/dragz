#!/usr/bin/env bash
# One-click: build the SPA and bring up the full local stack
# (postgres + redis + admin-api + jobs-runner + admin-web).
#
#   bash deploy/up.sh                 # build frontend, then start everything
#   bash deploy/up.sh --no-web        # skip the frontend build (reuse dist/)
#   bash deploy/up.sh --no-build      # reuse existing docker images
#   bash deploy/up.sh --api-only      # start backend + infra, no nginx/web
#
# Idempotent: safe to re-run, only changed layers rebuild.
source "$(dirname "${BASH_SOURCE[0]}")/lib.sh"

WITH_WEB=1
DOCKER_BUILD=1
API_ONLY=0

while [ "$#" -gt 0 ]; do
  case "$1" in
    --no-web)   WITH_WEB=0 ;;
    --no-build) DOCKER_BUILD=0 ;;
    --api-only) API_ONLY=1; WITH_WEB=0 ;;
    -h|--help)  sed -n '2,12p' "${BASH_SOURCE[0]}" | sed 's/^# \{0,1\}//'; exit 0 ;;
    *)          die "unknown option: $1" ;;
  esac
  shift
done

require_docker

# --- frontend ---------------------------------------------------------------
if [ "$WITH_WEB" = "1" ]; then
  build_web
elif [ ! -f "$WEB_DIR/dist/index.html" ]; then
  warn "frontend/admin/dist missing and --no-web given — the web image will fail to build"
fi

# --- compose ----------------------------------------------------------------
# NOTE: no --remove-orphans. The compose project name is derived from the
# directory ("deploy"), which other projects on this machine also use, so
# orphan cleanup would delete THEIR containers.
UP_ARGS=(up -d)
[ "$DOCKER_BUILD" = "1" ] && UP_ARGS+=(--build)

COMPOSE_UP_ARGS=("${UP_ARGS[@]}")
if [ "$API_ONLY" = "1" ]; then
  log "starting infra + backend (no web)"
  COMPOSE_UP_ARGS+=(postgres redis admin-api jobs-runner)
else
  log "starting full local stack"
fi

if ! run_compose "${LOCAL_FILES[@]}" -- "${COMPOSE_UP_ARGS[@]}"; then
  echo
  warn "compose failed to start the stack"
  note "most common cause: host port already in use (5432 / 6379 / 5280 / 5279)."
  note "remap them with an override file, then retry:"
  note "  DRAGZ_EXTRA_COMPOSE=\$PWD/deploy/ports.override.yml bash deploy/up.sh"
  exit 1
fi

# --- wait for readiness -----------------------------------------------------
API_PORT="$(svc_host_port admin-api 5280 5280)"
WEB_PORT="$(svc_host_port admin-web 80 5279)"

FAILED=0
wait_http "http://127.0.0.1:$API_PORT/api/health" 180 "admin-api  http://127.0.0.1:$API_PORT/api/health" || FAILED=1
# --api-only is the only mode that leaves admin-web out of the stack.
if [ "$API_ONLY" != "1" ]; then
  wait_http "http://127.0.0.1:$WEB_PORT/healthz" 120 "admin-web  http://127.0.0.1:$WEB_PORT" || FAILED=1
fi

echo
if [ "$FAILED" = "1" ]; then
  warn "one or more services did not become healthy — showing recent logs"
  run_compose "${LOCAL_FILES[@]}" -- ps
  echo
  note "inspect with: bash deploy/logs.sh"
  exit 1
fi

log "stack is up"
if [ "$API_ONLY" != "1" ]; then
  ok "admin web  -> http://127.0.0.1:$WEB_PORT"
fi
ok "admin api  -> http://127.0.0.1:$API_PORT/api/health"
note "login: admin / Dragz!7xZ  (password seeded by 001_system_init.sql, change on first login)"
note "logs: bash deploy/logs.sh   stop: bash deploy/down.sh"
