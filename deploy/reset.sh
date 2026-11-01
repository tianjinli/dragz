#!/usr/bin/env bash
# One-click: wipe the local database and re-bootstrap it from the migrations.
#
#   bash deploy/reset.sh
#
# Destructive: deletes the postgres volume, so the init scripts in
# backend/migrations/ run again on the next `up.sh`. Asks for confirmation.
source "$(dirname "${BASH_SOURCE[0]}")/lib.sh"

require_docker

confirm_yes "This deletes the local postgres/redis/upload volumes and re-runs the bootstrap migrations."

log "stopping stack and deleting volumes"
run_compose "${LOCAL_FILES[@]}" -- down --volumes

log "restarting from a clean database"
exec bash "$DEPLOY_DIR/up.sh" "$@"
