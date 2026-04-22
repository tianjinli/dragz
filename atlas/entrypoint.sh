#!/bin/sh
set -e

exec atlas migrate apply \
  --url "postgres://${DB_MASTER_USER}:${DB_MASTER_PASSWORD}@${DB_MASTER_HOST}:${DB_MASTER_PORT}/${DB_MASTER_DBNAME}?search_path=public&sslmode=disable" \
  --dir "file:///app/migrations" \
  --revisions-schema public \
  --baseline 20260115160432 \
  --tx-mode file
