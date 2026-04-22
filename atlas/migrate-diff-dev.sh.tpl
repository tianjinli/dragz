#!/usr/bin/env bash
set -euo pipefail
SECONDS=0

MIGRATE_NAME="${1:-}"
if [[ -z "$MIGRATE_NAME" ]]; then
  echo "❌ Error: Migration name is required"
  echo "Usage: $0 <migration_name>"
  exit 1
fi

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
  export "ATLAS_DIAL=${DB_CLUSTER_DRIVER:-mysql}"
else
  export "ATLAS_DIAL=${DB_MASTER_DRIVER:-postgres}"
fi

if [ "$ATLAS_DIAL" = "mysql" ]; then
  export "ATLAS_DEV=docker://mysql/8-debian/"
else
  export "ATLAS_DEV=docker://postgres/18-alpine/?search_path=public&sslmode=disable"
fi

ATLAS_CMD="atlas migrate diff $MIGRATE_NAME --env gorm"

echo "Time: $(date '+%Y-%m-%d %H:%M:%S')"
echo "Running: $ATLAS_CMD" && eval "$ATLAS_CMD"

echo "Elapsed time: $SECONDS seconds."
