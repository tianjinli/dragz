#!/usr/bin/env bash
# Shared helpers for deploy/*.sh — source-only, never executed directly.
#
#   source "$(dirname "${BASH_SOURCE[0]}")/lib.sh"
#
# Everything here is POSIX-ish bash that runs in Git Bash on Windows as well
# as Linux/macOS, so the same one-click commands work for local dev and prod.

set -euo pipefail

DEPLOY_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(cd "$DEPLOY_DIR/.." && pwd)"
BACKEND_DIR="$ROOT_DIR/backend"
WEB_DIR="$ROOT_DIR/frontend/admin"

# ---------------------------------------------------------------------------
# output
# ---------------------------------------------------------------------------
if [ -t 1 ] && [ "${NO_COLOR:-}" = "" ]; then
  C_DIM=$'\033[2m'; C_RED=$'\033[31m'; C_GRN=$'\033[32m'
  C_YEL=$'\033[33m'; C_CYN=$'\033[36m'; C_RST=$'\033[0m'
else
  C_DIM=""; C_RED=""; C_GRN=""; C_YEL=""; C_CYN=""; C_RST=""
fi

log()  { printf '%s==>%s %s\n' "$C_CYN" "$C_RST" "$*"; }
ok()   { printf '%s  ok%s %s\n' "$C_GRN" "$C_RST" "$*"; }
warn() { printf '%s  !!%s %s\n' "$C_YEL" "$C_RST" "$*" >&2; }
die()  { printf '%s  xx%s %s\n' "$C_RED" "$C_RST" "$*" >&2; exit 1; }
note() { printf '%s     %s%s\n' "$C_DIM" "$*" "$C_RST"; }

# ---------------------------------------------------------------------------
# prerequisites
# ---------------------------------------------------------------------------
# Docker Compose moved from the standalone `docker-compose` binary to the
# `docker compose` plugin. Prefer the plugin, fall back to the old binary.
COMPOSE_BIN=""
detect_compose() {
  [ -n "$COMPOSE_BIN" ] && return 0
  if docker compose version >/dev/null 2>&1; then
    COMPOSE_BIN="docker compose"
  elif command -v docker-compose >/dev/null 2>&1; then
    COMPOSE_BIN="docker-compose"
  else
    die "docker compose not found — install Docker Desktop or the compose plugin"
  fi
}

require_docker() {
  command -v docker >/dev/null 2>&1 || die "docker not found in PATH"
  docker info >/dev/null 2>&1 \
    || die "docker daemon is not running — start Docker Desktop and retry"
  detect_compose
}

require_pnpm() {
  command -v pnpm >/dev/null 2>&1 \
    || die "pnpm not found — run 'corepack enable pnpm' or install pnpm"
}

# ---------------------------------------------------------------------------
# compose wrapper
# ---------------------------------------------------------------------------
# run_compose <file> [<file>...] -- <compose args...>
#   run_compose docker-compose.yml docker-compose.local.yml -- up -d
# File names are resolved against deploy/ so the scripts work from any cwd.
run_compose() {
  detect_compose
  local -a files=()
  while [ "$#" -gt 0 ] && [ "$1" != "--" ]; do
    case "$1" in
      /*|[A-Za-z]:*) files+=(-f "$1") ;;
      *)            files+=(-f "$DEPLOY_DIR/$1") ;;
    esac
    shift
  done
  [ "$#" -gt 0 ] || die "run_compose: missing '--' separator"
  shift
  # COMPOSE_BIN is intentionally word-split ("docker compose").
  # shellcheck disable=SC2086
  $COMPOSE_BIN --project-directory "$DEPLOY_DIR" "${files[@]}" "$@"
}

# The local stack is split across two files: the base stack (postgres, redis,
# api, worker) and the web frontend that depends on it.
LOCAL_FILES=(docker-compose.yml docker-compose.local.yml)
INFRA_FILES=(docker-compose.infra.yml)

# Optional extra compose overlay, e.g. to dodge host port collisions:
#   DRAGZ_EXTRA_COMPOSE=/abs/path/ports.override.yml bash deploy/up.sh
if [ -n "${DRAGZ_EXTRA_COMPOSE:-}" ]; then
  read -r -a _extra_files <<< "$DRAGZ_EXTRA_COMPOSE"
  LOCAL_FILES+=("${_extra_files[@]}")
fi

# ---------------------------------------------------------------------------
# health polling
# ---------------------------------------------------------------------------
# curl on Windows picks up the system proxy and fails on 127.0.0.1 — always
# bypass it explicitly.
http_ok() {
  curl -fsS --noproxy '*' --max-time 3 "$1" >/dev/null 2>&1
}

# wait_http <url> <seconds> <label>
wait_http() {
  local url="$1" limit="${2:-120}" label="${3:-$1}" waited=0
  printf '    waiting for %s ' "$label"
  while [ "$waited" -lt "$limit" ]; do
    if http_ok "$url"; then
      printf ' %sready%s\n' "$C_GRN" "$C_RST"
      return 0
    fi
    printf '.'
    sleep 2
    waited=$((waited + 2))
  done
  printf ' %stimeout%s\n' "$C_RED" "$C_RST"
  return 1
}

# ---------------------------------------------------------------------------
# frontend build
# ---------------------------------------------------------------------------
# The web image copies a prebuilt frontend/admin/dist, so the SPA has to be
# compiled before the image is assembled.
build_web() {
  require_pnpm
  [ -d "$WEB_DIR/node_modules" ] || {
    log "installing frontend dependencies"
    ( cd "$WEB_DIR" && pnpm install )
  }
  # Clear the output dir ourselves: it is a pure build artifact, and letting
  # the bundler empty it can trip filesystem guards on managed machines.
  rm -rf "$WEB_DIR/dist"
  log "building frontend (tsc + vite)"
  ( cd "$WEB_DIR" && pnpm build )
  [ -f "$WEB_DIR/dist/index.html" ] || die "frontend build produced no dist/index.html"
  ok "frontend dist ready"
}

# ---------------------------------------------------------------------------
# misc
# ---------------------------------------------------------------------------
# Print the host port actually published for a running service, falling back
# to the compose default when the container is not up yet. Keeps the summary
# and health probes correct when ports are remapped via an override file.
svc_host_port() {
  local service="$1" container_port="$2" fallback="$3" published
  published="$(run_compose "${LOCAL_FILES[@]}" -- port "$service" "$container_port" 2>/dev/null \
    | head -1 | sed 's/.*://' | tr -d '[:space:]')"
  if [ -n "$published" ]; then
    printf '%s' "$published"
  else
    printf '%s' "$fallback"
  fi
}

confirm_yes() {
  local prompt="$1" answer
  printf '%s%s%s\n' "$C_YEL" "$prompt" "$C_RST"
  printf 'type "yes" to continue: '
  read -r answer
  [ "$answer" = "yes" ] || die "aborted"
}
