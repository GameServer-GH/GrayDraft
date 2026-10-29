#!/usr/bin/env bash
# 重建并重启常改动的应用服务（core / worker / gateway / console / ops / web / graydraft）。
# 不触碰 postgres、redis、nats、nginx 等基础设施容器。
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "$ROOT"

ENV="${GRAYDRAFT_ENV:-dev}"
NO_CACHE=0
DEFAULT_SERVICES=(core worker gateway console ops web graydraft)
SERVICES=()

usage() {
  cat <<'EOF'
用法: deploy/scripts/rebuild-apps.sh [选项] [服务名…]

重建并 force-recreate 指定应用容器（默认全部应用服务）。
不会 build/up postgres、redis、nats、nginx。

选项:
  --dev         开发 compose（默认）
  --prod        生产 compose
  --no-cache    docker build 时不使用缓存
  -h, --help    显示帮助

环境变量:
  GRAYDRAFT_ENV    dev | prod（与 --dev / --prod 等效，命令行优先）

默认服务: core worker gateway console ops web graydraft

示例:
  ./deploy/scripts/rebuild-apps.sh
  ./deploy/scripts/rebuild-apps.sh core
  ./deploy/scripts/rebuild-apps.sh --no-cache core gateway
  GRAYDRAFT_ENV=prod ./deploy/scripts/rebuild-apps.sh core worker
EOF
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --dev)
      ENV=dev
      shift
      ;;
    --prod)
      ENV=prod
      shift
      ;;
    --no-cache)
      NO_CACHE=1
      shift
      ;;
    -h | --help)
      usage
      exit 0
      ;;
    -*)
      echo "未知选项: $1" >&2
      usage >&2
      exit 1
      ;;
    *)
      SERVICES+=("$1")
      shift
      ;;
  esac
done

if [[ ${#SERVICES[@]} -eq 0 ]]; then
  SERVICES=("${DEFAULT_SERVICES[@]}")
fi

COMPOSE_FILE="deploy/docker-compose/docker-compose.${ENV}.yml"
ENV_FILE="deploy/config/${ENV}/.env.ports"

if [[ ! -f "$COMPOSE_FILE" ]]; then
  echo "缺少 compose 文件: $COMPOSE_FILE" >&2
  exit 1
fi
if [[ ! -f "$ENV_FILE" ]]; then
  echo "缺少端口配置: $ENV_FILE" >&2
  exit 1
fi

COMPOSE=(docker compose -f "$COMPOSE_FILE" --env-file "$ENV_FILE")

echo "==> GrayDraft 应用重建 (${ENV})"
echo "    服务: ${SERVICES[*]}"
echo "    compose: $COMPOSE_FILE"

if [[ "$NO_CACHE" -eq 1 ]]; then
  "${COMPOSE[@]}" build --no-cache "${SERVICES[@]}"
else
  "${COMPOSE[@]}" build "${SERVICES[@]}"
fi
"${COMPOSE[@]}" up -d --force-recreate --no-deps "${SERVICES[@]}"

# shellcheck disable=SC1090
set -a
source "$ENV_FILE"
set +a

CORE_PORT="${CORE_HTTP_PORT:-18080}"
HEALTH_URL="http://127.0.0.1:${CORE_PORT}/api/health"

echo ""
echo "==> 完成。Core 健康检查: ${HEALTH_URL}"
if curl -sf "$HEALTH_URL" >/dev/null 2>&1; then
  echo "OK  core /api/health"
else
  echo "WARN core 尚未就绪，可稍后执行: curl -s ${HEALTH_URL}" >&2
fi
