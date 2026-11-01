#!/usr/bin/env bash
# Generate a Docker registry (hub.cszqq.top) username/password and add it to
# the registry's htpasswd file.
#
# Run ON THE REGISTRY HOST (134.175.157.238), not on the k3s node:
#   sudo bash gen-registry-user.sh [username] [password] [--htpasswd PATH]
#
# - If username is omitted you will be prompted (default: zhhuyu).
# - If password is omitted a strong 24-char one is generated.
# - Uses bcrypt (-B), supported by registry:2.
# - Appends to (or creates) the htpasswd file, then prints credentials
#   and the exact docker command to verify login.
#
# After adding a user, make the registry use the file (if not already) and
# restart it, e.g.:
#   docker run -d --restart unless-stopped --name registry -p 443:5000 \
#     -v /opt/registry-auth/htpasswd:/auth/htpasswd:ro \
#     -v /opt/registry-data:/var/lib/registry \
#     -e REGISTRY_AUTH=htpasswd \
#     -e REGISTRY_AUTH_HTPASSWD_REALM="Registry Realm" \
#     -e REGISTRY_AUTH_HTPASSWD_PATH=/auth/htpasswd \
#     registry:2
set -euo pipefail

HTPASSWD_FILE="/opt/registry-auth/htpasswd"
USER_NAME=""
USER_PASS=""

while [ $# -gt 0 ]; do
  case "$1" in
    --htpasswd) HTPASSWD_FILE="$2"; shift 2;;
    --htpasswd=*) HTPASSWD_FILE="${1#*=}"; shift;;
    -h|--help)
      echo "Usage: sudo bash gen-registry-user.sh [username] [password] [--htpasswd PATH]"; exit 0;;
    *)
      if [ -z "$USER_NAME" ]; then USER_NAME="$1";
      elif [ -z "$USER_PASS" ]; then USER_PASS="$1";
      else echo "unexpected arg: $1" >&2; exit 1; fi
      shift;;
  esac
done

if [ -z "$USER_NAME" ]; then
  read -r -p "Registry username [zhhuyu]: " USER_NAME || true
  USER_NAME="${USER_NAME:-zhhuyu}"
fi
if [ -z "$USER_PASS" ]; then
  # URL-safe alphanumeric: no shell/Docker/URL escaping issues.
  USER_PASS="$(openssl rand -base64 18 | tr -dc 'A-Za-z0-9' | head -c 24)"
  echo "(generated a random password)"
fi
if [ "${#USER_PASS}" -lt 12 ]; then
  echo "password must be at least 12 characters" >&2; exit 1
fi

mkdir -p "$(dirname "$HTPASSWD_FILE")"

# Prefer the host htpasswd binary; fall back to a throwaway httpd container.
if command -v htpasswd >/dev/null 2>&1; then
  if [ -f "$HTPASSWD_FILE" ]; then
    htpasswd -B -b "$HTPASSWD_FILE" "$USER_NAME" "$USER_PASS"
  else
    htpasswd -B -b -c "$HTPASSWD_FILE" "$USER_NAME" "$USER_PASS"
  fi
elif command -v docker >/dev/null 2>&1; then
  touch "$HTPASSWD_FILE"
  # Hash inside a throwaway httpd container with the file mounted in:
  docker run --rm \
    -v "$HTPASSWD_FILE:/auth/htpasswd" \
    --entrypoint htpasswd httpd:2 -B -b /auth/htpasswd "$USER_NAME" "$USER_PASS"
else
  echo "need 'htpasswd' or 'docker' to hash the password" >&2; exit 1
fi

chmod 600 "$HTPASSWD_FILE"
echo
echo "=== registry credentials (store these) ==="
echo "server:   hub.cszqq.top"
echo "username: $USER_NAME"
echo "password: $USER_PASS"
echo "htpasswd: $HTPASSWD_FILE"
echo
echo "=== verify (run anywhere with docker) ==="
echo "echo '$USER_PASS' | docker login hub.cszqq.top -u '$USER_NAME' --password-stdin"
echo
echo "After changing users, restart the registry container so it reloads htpasswd."
