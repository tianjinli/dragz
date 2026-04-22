#!/usr/bin/env bash
set -euo pipefail
SECONDS=0

echo "=================================================="
echo "Loading environment variables from .env.dev"
echo "=================================================="

cd "$(dirname "$0")"
ENV_FILE="../.env.dev"

clean_line() {
  echo "$1" | tr -d '\r' | sed 's/^[ \t]*//;s/[ \t]*$//'
}

if [[ -f "$ENV_FILE" ]]; then
  while IFS= read -r line || [[ -n "$line" ]]; do
    line="$(clean_line "$line")"
    [[ -z "$line" || "$line" =~ ^# ]] && continue

    key="${line%%=*}"
    value="${line#*=}"

    key="$(clean_line "$key")"
    value="$(clean_line "$value")"

    [[ -z "$value" ]] && continue

    export "$key=$value"
    echo "Set $key=$value"
  done < "$ENV_FILE"
else
  echo "❌ $ENV_FILE not found"
  exit 1
fi

if [ "$DB_PRIMARY" = "cluster" ]; then
  DB_HOST="${DB_CLUSTER_HOST:-localhost}"
  DB_PORT="${DB_CLUSTER_PORT:-}"
  DB_USER="${DB_CLUSTER_USER:-}"
  DB_PASSWORD="$DB_CLUSTER_PASSWORD"
  DB_NAME="${DB_CLUSTER_DBNAME:-dragz}"
  export "ATLAS_DIAL=${DB_CLUSTER_DRIVER:-mysql}"
else
  DB_HOST="${DB_MASTER_HOST:-localhost}"
  DB_PORT="${DB_MASTER_PORT:-}"
  DB_USER="${DB_MASTER_USER:-}"
  DB_PASSWORD="$DB_MASTER_PASSWORD"
  DB_NAME="${DB_MASTER_DBNAME:-dragz}"
  export "ATLAS_DIAL=${DB_MASTER_DRIVER:-postgres}"
fi

if [ "$ATLAS_DIAL" = "mysql" ]; then
  export "ATLAS_URL=mysql://${DB_USER:-root}:$DB_PASSWORD@$DB_HOST:${DB_PORT:-3306}/$DB_NAME"
  export "ATLAS_DEV=docker://mysql/8-debian/"
else
  export "ATLAS_URL=postgres://${DB_USER:-postgres}:$DB_PASSWORD@$DB_HOST:${DB_PORT:-5432}/$DB_NAME?search_path=public&sslmode=disable"
  export "ATLAS_DEV=docker://postgres/18-alpine/?search_path=public&sslmode=disable"
fi

if ! command -v atlas >/dev/null 2>&1; then
  echo "🛠️ Atlas is not installed, installing now..."
  curl -sSf https://atlasgo.sh | sh
fi

ATLAS_CMD="atlas schema apply --env $ATLAS_DIAL --auto-approve"
ATLAS_URL="https://release.ariga.io/atlas/atlas-linux-amd64-latest"

echo "Time: $(date '+%Y-%m-%d %H:%M:%S')"
echo "Running: $ATLAS_CMD" && eval "$ATLAS_CMD"

echo "Elapsed time: $SECONDS seconds."
