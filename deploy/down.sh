#!/usr/bin/env bash
# One-click: stop the local stack.
#
#   bash deploy/down.sh             # stop and remove containers (data volumes kept)
#   bash deploy/down.sh --volumes   # ALSO delete postgres/redis/upload data (destructive)
source "$(dirname "${BASH_SOURCE[0]}")/lib.sh"

require_docker

# No --remove-orphans: the project name ("deploy") is shared with other stacks
# on this machine, so orphan cleanup could remove their containers.
ARGS=(down)
if [ "${1:-}" = "--volumes" ] || [ "${1:-}" = "-v" ]; then
  confirm_yes "This deletes the postgres, redis and upload volumes — all local data is lost."
  ARGS+=(--volumes)
elif [ "$#" -gt 0 ] && [ "$1" != "-h" ] && [ "$1" != "--help" ]; then
  die "unknown option: $1"
fi

log "stopping local stack"
run_compose "${LOCAL_FILES[@]}" -- "${ARGS[@]}"
ok "stopped"

if [ "${1:-}" != "--volumes" ] && [ "${1:-}" != "-v" ]; then
  note "data volumes were kept; use 'bash deploy/down.sh --volumes' to wipe them"
fi
