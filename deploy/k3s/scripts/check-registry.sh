#!/usr/bin/env bash
# Read-only diagnostics for the hub.cszqq.top registry host (134.175.157.238).
# It NEVER prints passwords or full hashes (usernames + hash scheme only).
#
# Usage on the registry host:
#   sudo bash check-registry.sh '<username>' '<password>'
#
# It reports:
#   1. which registry/nginx containers are actually running (and since when)
#   2. REGISTRY_AUTH* env + bind mounts of the registry container
#   3. host htpasswd: usernames present, hash scheme, CRLF check
#   4. whether the in-container file matches the host file
#   5. recent registry auth errors from docker logs
#   6. nginx upstream backend for hub.cszqq.top
#   7. login test DIRECT to registry backend vs VIA nginx (HTTP code only)
set -uo pipefail

USER_NAME="${1:?usage: sudo bash check-registry.sh <username> <password>}"
USER_PASS="${2:?usage: sudo bash check-registry.sh <username> <password>}"

say() { echo; echo "===== $* ====="; }

say "1. containers"
docker ps --format 'table {{.Names}}\t{{.Image}}\t{{.Status}}\t{{.Ports}}' 2>/dev/null | grep -iE '^(NAME|.*(regist|nginx))' || echo "(no registry/nginx container found)"

REG_C="$(docker ps --format '{{.Names}}' 2>/dev/null | grep -i regist | head -1 || true)"
if [ -z "$REG_C" ]; then echo "No running registry container. Stop here."; exit 1; fi
echo "registry container: $REG_C"

say "2. REGISTRY_AUTH env + mounts ($REG_C)"
docker inspect "$REG_C" --format '{{range .Config.Env}}{{println .}}{{end}}' | grep -iE '^REGISTRY_' || echo "(no REGISTRY_ env)"
docker inspect "$REG_C" --format '{{range .Mounts}}{{println .Source " -> " .Destination}}{{end}}'

HTPASSWD_DST="$(docker inspect "$REG_C" --format '{{range .Config.Env}}{{println .}}{{end}}' | grep -E '^REGISTRY_AUTH_HTPASSWD_PATH=' | cut -d= -f2- || true)"
HTPASSWD_SRC=""
if [ -n "$HTPASSWD_DST" ]; then
  HTPASSWD_SRC="$(docker inspect "$REG_C" --format "{{range .Mounts}}{{if eq .Destination \"$HTPASSWD_DST\"}}{{println .Source}}{{end}}{{end}}" | head -1 || true)"
fi
echo "htpasswd in-container path: ${HTPASSWD_DST:-(unset)}"
echo "htpasswd host source:       ${HTPASSWD_SRC:-(no bind mount found)}"

say "3. host htpasswd content (usernames + hash scheme, hashes masked)"
if [ -n "$HTPASSWD_SRC" ] && [ -f "$HTPASSWD_SRC" ]; then
  echo "file: $HTPASSWD_SRC ($(stat -c '%s bytes, %y' "$HTPASSWD_SRC" 2>/dev/null || wc -c <"$HTPASSWD_SRC"))"
  if grep -q $'\r' "$HTPASSWD_SRC"; then
    echo "!! WARNING: file contains CRLF (Windows line endings) - registry will reject every login. Run: sed -i 's/\\r$//' $HTPASSWD_SRC"
  else
    echo "line endings: LF (ok)"
  fi
  while IFS=: read -r u h _; do
    [ -z "$u" ] && continue
    scheme="unknown"
    case "$h" in
      \$2y\$*|\$2a\$*|\$2b\$*) scheme="bcrypt (ok)";;
      \$apr1\$*) scheme="md5-apr1 (ok)";;
      \{SHA\}*) scheme="sha1 (ok)";;
    esac
    mark=""; [ "$u" = "$USER_NAME" ] && mark="  <-- login user"
    echo "user=$u scheme=[$scheme]$mark"
  done <"$HTPASSWD_SRC"
else
  echo "host htpasswd file not found at: ${HTPASSWD_SRC:-(unknown)}"
fi

say "4. in-container file matches host file?"
if [ -n "$HTPASSWD_DST" ]; then
  IN_CTR="$(docker exec "$REG_C" cat "$HTPASSWD_DST" 2>/dev/null | cut -d: -f1 | sort | tr '\n' ' ' || echo "(unreadable)")"
  echo "in-container users: $IN_CTR"
fi

say "5. recent registry auth errors"
docker logs --since 30m "$REG_C" 2>&1 | grep -iE 'error|unauthor|denied|fail' | tail -8 || echo "(no auth errors in last 30m)"

say "6. nginx upstream for hub.cszqq.top"
NGX_C="$(docker ps --format '{{.Names}}' 2>/dev/null | grep -i nginx | head -1 || true)"
if [ -n "$NGX_C" ]; then
  echo "nginx container: $NGX_C"
  docker exec "$NGX_C" sh -c 'grep -rl "hub.cszqq.top" /etc/nginx/ 2>/dev/null' | head -3
  for f in $(docker exec "$NGX_C" sh -c 'grep -rl "hub.cszqq.top" /etc/nginx/ 2>/dev/null' | head -2); do
    echo "--- $f ---"
    docker exec "$NGX_C" cat "$f" 2>/dev/null | grep -iE 'server_name|proxy_pass|listen|auth_basic|proxy_set_header Authorization' | head -12
  done
else
  echo "(nginx not in docker; checking host nginx)"
  grep -rl "hub.cszqq.top" /etc/nginx/ 2>/dev/null | head -3 || echo "(no hub vhost found on host)"
fi

say "7. login test: DIRECT backend vs VIA nginx (codes only)"
# direct: find a published registry port or the container IP
PUB_PORT="$(docker inspect "$REG_C" --format '{{range $p,$c := .NetworkSettings.Ports}}{{$p}} {{end}}' | grep -oE '[0-9]+/tcp' | head -1 | cut -d/ -f1 || true)"
HOST_PORT="$(docker inspect "$REG_C" --format "{{(index .NetworkSettings.Ports \"$PUB_PORT/tcp\" 0).HostPort}}" 2>/dev/null || true)"
if [ -n "$HOST_PORT" ]; then
  D_CODE="$(curl -sS -m 10 -o /dev/null -w '%{http_code}' -u "$USER_NAME:$USER_PASS" "http://127.0.0.1:$HOST_PORT/v2/" 2>/dev/null || echo "000")"
  echo "direct  http://127.0.0.1:$HOST_PORT/v2/ -> $D_CODE  (200 = htpasswd OK, registry fine; 401 = htpasswd/user wrong)"
else
  echo "direct: no published port, trying container IP"
  CIP="$(docker inspect "$REG_C" --format '{{.NetworkSettings.IPAddress}}' 2>/dev/null || true)"
  if [ -n "$CIP" ]; then
    D_CODE="$(curl -sS -m 10 -o /dev/null -w '%{http_code}' -u "$USER_NAME:$USER_PASS" "http://$CIP:5000/v2/" 2>/dev/null || echo "000")"
    echo "direct  http://$CIP:5000/v2/ -> $D_CODE"
  else
    echo "direct: cannot determine backend address"
  fi
fi
N_CODE="$(curl -sS -m 10 -o /dev/null -w '%{http_code}' -u "$USER_NAME:$USER_PASS" "https://hub.cszqq.top/v2/" 2>/dev/null || echo "000")"
echo "via-nginx https://hub.cszqq.top/v2/ -> $N_CODE"
echo
echo "Interpretation:"
echo "  direct=200 + via-nginx=401  => nginx strips/breaks Authorization or proxies elsewhere (check proxy_set_header, upstream)."
echo "  direct=401                  => registry never saw a valid entry: wrong file mounted, stale container, CRLF, or wrong hash."
echo "  both=200                    => registry is fine; the failing client uses wrong creds/URL."
