#!/usr/bin/env bash
# 清空 GrayDraft 知识库、UGC 任务、租户元数据、检索/答案缓存，以及 GrayDraft 工作区本地数据。
# 用法：从仓库根目录执行 ./deploy/scripts/reset-all-data.sh
# 可选：GRAYDRAFT_DATA_DIR=/path/to/GrayDraft_Go/data

set -euo pipefail

ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
cd "$ROOT"

ENV_PORTS="${ENV_PORTS:-$ROOT/deploy/config/dev/.env.ports}"
ENV_PG="${ENV_PG:-$ROOT/deploy/config/dev/.env.postgres}"
COMPOSE_FILE="$ROOT/deploy/docker-compose/docker-compose.dev.yml"

# shellcheck disable=SC1090
[[ -f "$ENV_PORTS" ]] && set -a && source "$ENV_PORTS" && set +a
# shellcheck disable=SC1090
[[ -f "$ENV_PG" ]] && set -a && source "$ENV_PG" && set +a

POSTGRES_PORT="${POSTGRES_PORT:-25432}"
POSTGRES_USER="${POSTGRES_USER:-graydraft_admin}"
POSTGRES_PASSWORD="${POSTGRES_PASSWORD:-dev_password_123}"
POSTGRES_DB="${POSTGRES_DB:-graydraft_kb}"
REDIS_PORT="${REDIS_PORT:-26379}"
REDIS_PASSWORD="${REDIS_PASSWORD:-dev_redis_pass}"

GRAYDRAFT_DATA="${GRAYDRAFT_DATA_DIR:-$ROOT/../GrayDraft_Go/data}"

echo "==> 启动 Postgres / Redis（若已存在容器）"
docker start graydraft-postgres-dev graydraft-redis-dev 2>/dev/null || true
for i in $(seq 1 30); do
  if PGPASSWORD="$POSTGRES_PASSWORD" pg_isready -h 127.0.0.1 -p "$POSTGRES_PORT" -U "$POSTGRES_USER" -d "$POSTGRES_DB" >/dev/null 2>&1; then
    break
  fi
  sleep 1
done

echo "==> 清空 GrayDraft Postgres（知识库 + 任务 + 租户）"
SQL_FILE="$(mktemp)"
cat >"$SQL_FILE" <<'SQL'
BEGIN;

TRUNCATE TABLE
  ragops_heal_jobs,
  ragops_chunk_strikes,
  ugc_ingest_tasks,
  kb_doc_tags,
  kb_qa_pairs,
  kb_chunks,
  kb_documents
RESTART IDENTITY CASCADE;

DO $$
BEGIN
  IF to_regclass('public.knowledge_bases') IS NOT NULL THEN
    TRUNCATE TABLE knowledge_bases, tenants RESTART IDENTITY CASCADE;
  END IF;
END $$;

DELETE FROM kb_catalog_version
WHERE kb_type NOT IN ('tutorial', 'paper', 'note');

DELETE FROM kb_types
WHERE name NOT IN ('tutorial', 'paper', 'note');

UPDATE kb_catalog_version
SET version = 1, latest_doc_updated_at = now(), updated_at = now();

REFRESH MATERIALIZED VIEW kb_global_index;

COMMIT;
SQL

if docker exec -i graydraft-postgres-dev psql -U "$POSTGRES_USER" -d "$POSTGRES_DB" -v ON_ERROR_STOP=1 <"$SQL_FILE"; then
  echo "    Postgres 已清空"
elif command -v psql >/dev/null 2>&1; then
  export PGPASSWORD="$POSTGRES_PASSWORD"
  psql -h 127.0.0.1 -p "$POSTGRES_PORT" -U "$POSTGRES_USER" -d "$POSTGRES_DB" -v ON_ERROR_STOP=1 -f "$SQL_FILE"
  echo "    Postgres 已清空（host psql）"
else
  rm -f "$SQL_FILE"
  echo "错误: 需要 graydraft-postgres-dev 容器或本机 psql" >&2
  exit 1
fi
rm -f "$SQL_FILE"

echo "==> 清空 Redis（检索缓存 knex:sr:* + 答案缓存 knex:ans:*）"
if docker exec graydraft-redis-dev redis-cli -a "$REDIS_PASSWORD" FLUSHALL >/dev/null 2>&1; then
  echo "    Redis FLUSHALL via container"
else
  if command -v redis-cli >/dev/null 2>&1; then
    redis-cli -h 127.0.0.1 -p "$REDIS_PORT" -a "$REDIS_PASSWORD" FLUSHALL >/dev/null
    echo "    Redis FLUSHALL via host redis-cli"
  else
    echo "    警告: 无法连接 Redis，请手动 FLUSHALL"
  fi
fi

echo "==> 清空 NATS JetStream 卷（UGC / DLQ 队列）"
docker stop graydraft-nats-dev 2>/dev/null || true
VOL="$(docker volume ls -q | grep -E 'nats_data_dev$' | head -1 || true)"
if [[ -n "$VOL" ]]; then
  docker volume rm "$VOL" 2>/dev/null && echo "    已删除 volume: $VOL" || echo "    跳过 volume（可能被占用）: $VOL"
else
  echo "    未找到 nats_data_dev volume，跳过"
fi

echo "==> 清空 GrayDraft 本地工作区: $GRAYDRAFT_DATA"
if [[ -d "$GRAYDRAFT_DATA" ]]; then
  rm -rf "${GRAYDRAFT_DATA:?}/workspaces"/*
  mkdir -p "$GRAYDRAFT_DATA/workspaces"
  printf '[]\n' >"$GRAYDRAFT_DATA/notebooks.json"
  rm -rf "$GRAYDRAFT_DATA/export_jobs" 2>/dev/null || true
  echo "    已重置 workspaces/ 与 notebooks.json"
else
  echo "    目录不存在，跳过 GrayDraft"
fi

echo ""
echo "完成。建议："
echo "  1. make dev-up  重新拉起服务（NATS 会新建空卷）"
echo "  2. GrayDraft 前端：重新 Open Workspace（会重新 EnsureKbType）"
echo "  3. 浏览器 localStorage 中的工作区密钥/布局如需彻底清空，请在应用内删除或清站点数据"
