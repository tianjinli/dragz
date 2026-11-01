#!/usr/bin/env bash
# One-click: follow logs for the local stack.
#
#   bash deploy/logs.sh              # all services
#   bash deploy/logs.sh admin-api    # one service
#   bash deploy/logs.sh -n 500       # last 500 lines then follow
source "$(dirname "${BASH_SOURCE[0]}")/lib.sh"

require_docker

TAIL=200
SERVICE=""
while [ "$#" -gt 0 ]; do
  case "$1" in
    -n) TAIL="${2:?-n needs a value}"; shift 2 ;;
    -h|--help) sed -n '2,8p' "${BASH_SOURCE[0]}" | sed 's/^# \{0,1\}//'; exit 0 ;;
    *) SERVICE="$1"; shift ;;
  esac
done

log "following logs (tail $TAIL${SERVICE:+, service=$SERVICE})"
if [ -n "$SERVICE" ]; then
  run_compose "${LOCAL_FILES[@]}" -- logs -f --tail="$TAIL" "$SERVICE"
else
  run_compose "${LOCAL_FILES[@]}" -- logs -f --tail="$TAIL"
fi
