#!/usr/bin/env bash
# 按序执行 deploy/migrations/*.up.sql
set -euo pipefail

MIGRATIONS_DIR="${MIGRATIONS_DIR:-/docker-entrypoint-initdb.d/migrations}"
DB_USER="${POSTGRES_USER:-graydraft_admin}"
DB_NAME="${POSTGRES_DB:-graydraft_kb}"

if [[ -d "$MIGRATIONS_DIR" ]]; then
  for f in $(ls "$MIGRATIONS_DIR"/*.up.sql 2>/dev/null | sort); do
    echo "Applying migration: $(basename "$f")"
    psql -v ON_ERROR_STOP=1 --username "$DB_USER" --dbname "$DB_NAME" -f "$f"
  done
else
  echo "Migrations dir not found: $MIGRATIONS_DIR"
  exit 1
fi

echo "GrayDraft database migrations complete."
