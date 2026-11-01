#!/usr/bin/env bash
# One-click: show local stack status plus a health probe per service.
#
#   bash deploy/ps.sh
source "$(dirname "${BASH_SOURCE[0]}")/lib.sh"

require_docker

run_compose "${LOCAL_FILES[@]}" -- ps

echo
log "health probes"
API_PORT="$(svc_host_port admin-api 5280 5280)"
WEB_PORT="$(svc_host_port admin-web 80 5279)"
probe() {
  if http_ok "$2"; then
    printf '    %-12s %sok%s   %s\n' "$1" "$C_GRN" "$C_RST" "$2"
  else
    printf '    %-12s %sdown%s %s\n' "$1" "$C_RED" "$C_RST" "$2"
  fi
}
probe admin-api "http://127.0.0.1:$API_PORT/api/health"
probe admin-web "http://127.0.0.1:$WEB_PORT/healthz"
