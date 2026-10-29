#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/../.." && pwd)"

DB_HOST="${DB_HOST:-localhost}"
DB_PORT="${DB_PORT:-25432}"
DB_USER="${DB_USER:-graydraft_admin}"
DB_NAME="${DB_NAME:-graydraft_kb}"
DB_PASSWORD="${DB_PASSWORD:-dev_password_123}"
export PGPASSWORD="${DB_PASSWORD}"

POSTGRES_CONTAINER="${POSTGRES_CONTAINER:-graydraft-postgres-dev}"

use_docker_psql() {
  docker ps --format '{{.Names}}' 2>/dev/null | grep -qx "$POSTGRES_CONTAINER"
}

run_psql() {
  if command -v psql >/dev/null 2>&1 && ! use_docker_psql; then
    psql -h "$DB_HOST" -p "$DB_PORT" -U "$DB_USER" -d "$DB_NAME" -v ON_ERROR_STOP=1 "$@"
  elif use_docker_psql; then
    docker exec -i "$POSTGRES_CONTAINER" psql -U "$DB_USER" -d "$DB_NAME" -v ON_ERROR_STOP=1 "$@"
  else
    echo "psql not found and container $POSTGRES_CONTAINER is not running"
    echo "Start stack with: make dev-up"
    exit 1
  fi
}

run_migration_file() {
  local f="$1"
  if use_docker_psql; then
    docker exec -i "$POSTGRES_CONTAINER" psql -U "$DB_USER" -d "$DB_NAME" -v ON_ERROR_STOP=1 <"$f"
  else
    psql -h "$DB_HOST" -p "$DB_PORT" -U "$DB_USER" -d "$DB_NAME" -v ON_ERROR_STOP=1 -f "$f"
  fi
}

ensure_migrations_table() {
  run_psql <<'SQL'
CREATE TABLE IF NOT EXISTS schema_migrations (
    name TEXT PRIMARY KEY,
    applied_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
SQL
}

migration_applied() {
  local name="$1"
  run_psql -tAc "SELECT 1 FROM schema_migrations WHERE name = '$name' LIMIT 1;" | grep -q 1
}

mark_migration_applied() {
  local name="$1"
  run_psql -c "INSERT INTO schema_migrations (name) VALUES ('$name') ON CONFLICT DO NOTHING;"
}

ensure_migrations_table

for f in $(ls "$ROOT/deploy/migrations"/*.up.sql 2>/dev/null | sort); do
  name="$(basename "$f")"
  if migration_applied "$name"; then
    echo "Skip $name (already applied)"
    continue
  fi
  echo "Applying $name..."
  run_migration_file "$f"
  mark_migration_applied "$name"
done

echo "Migrations applied."
