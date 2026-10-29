.PHONY: help dev dev-gpu dev-mon dev-down dev-restart prod-up prod-down build-all proto-gen proto-lint test-unit lint db-init health logs-all clean

help: ## 显示帮助
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | \
	awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-20s\033[0m %s\n", $$1, $$2}'

dev-up: ## 启动开发环境（基础服务）
	docker compose -f deploy/docker-compose/docker-compose.dev.yml --env-file deploy/config/dev/.env.ports up -d

dev-up-gpu: ## 启动开发环境（含 GPU 模型服务）
	docker compose -f deploy/docker-compose/docker-compose.dev.yml --env-file deploy/config/dev/.env.ports --profile gpu up -d

dev-up-mon: ## 启动开发环境（含监控）
	docker compose -f deploy/docker-compose/docker-compose.dev.yml --env-file deploy/config/dev/.env.ports --profile monitoring up -d

dev-down: ## 停止开发环境
	docker compose -f deploy/docker-compose/docker-compose.dev.yml down

dev-restart: dev-down dev-up ## 重启开发环境

rebuild-apps: ## 重建应用镜像并重启（core/worker/gateway/console/ops，不含 DB/Redis）
	@chmod +x deploy/scripts/rebuild-apps.sh
	./deploy/scripts/rebuild-apps.sh

rebuild-apps-prod: ## 生产环境重建应用服务
	@chmod +x deploy/scripts/rebuild-apps.sh
	./deploy/scripts/rebuild-apps.sh --prod

backfill-search-anchors: ## LLM 回填 search_anchors（用法: make backfill-search-anchors KB_TYPE=xxx）
	@chmod +x deploy/scripts/backfill-search-anchors.sh
	./deploy/scripts/backfill-search-anchors.sh $(KB_TYPE)

backfill-search-text: ## 回填 kb_chunks.search_text（语义锚点注入）
	@chmod +x deploy/scripts/backfill-search-text.sh
	./deploy/scripts/backfill-search-text.sh

LIMIT ?= 50
backfill-semantic-4d: ## LLM 回填文档 semantic_tags_4d（用法: make backfill-semantic-4d KB_TYPE=xxx LIMIT=50）
	@CONFIG_PATH=internal/core/config/config.dev.yaml go run ./internal/core/cmd/backfill-semantic4d -kb_type=$(KB_TYPE) -limit=$(LIMIT)

backfill-semantic-4d-all: backfill-semantic-4d backfill-search-text ## 4D 回填后刷新 search_text（用法: make backfill-semantic-4d-all KB_TYPE=xxx）

backfill-search-text: ## 回填 kb_chunks.search_text（Worker build_search_text + chunk metadata）
	@chmod +x deploy/scripts/backfill-search-text.sh
	./deploy/scripts/backfill-search-text.sh $(KB_TYPE)

prod-up: ## 启动生产环境
	go run ./pkg/gocommon/cmd/writetls -out deploy/nginx/ssl/graydraft.com.key
	docker compose -f deploy/docker-compose/docker-compose.prod.yml --env-file deploy/config/prod/.env.ports up -d

prod-down: ## 停止生产环境
	docker compose -f deploy/docker-compose/docker-compose.prod.yml down

build-core: ## 构建 Core 镜像
	docker build -f deploy/dockerfiles/core.Dockerfile -t graydraft-core:latest .

build-worker: ## 构建 Worker 镜像
	docker build -f deploy/dockerfiles/worker.Dockerfile -t graydraft-worker:latest .

build-all: build-core build-worker ## 构建所有服务镜像
	docker build -f deploy/dockerfiles/gateway.Dockerfile -t graydraft-gateway:latest .
	docker build -f deploy/dockerfiles/console.Dockerfile -t graydraft-console:latest .
	docker build -f deploy/dockerfiles/ops.Dockerfile -t graydraft-ops:latest .

proto-gen: ## 生成 Proto 代码
	cd protos && buf generate

proto-lint: ## 检查 Proto 规范
	cd protos && buf lint

test-unit: ## 单元测试
	cd internal/core && go test ./...
	cd internal/worker && python -m pytest -q || true

lint: ## 全量代码检查
	./scripts/lint.sh

db-init: ## 初始化数据库
	./deploy/scripts/init-db.sh

db-migrate: ## 运行数据库迁移
	./deploy/scripts/migrate.sh

health: ## 健康检查
	./deploy/scripts/health-check.sh

logs-all: ## 查看所有服务日志
	docker compose -f deploy/docker-compose/docker-compose.dev.yml logs -f

clean: ## 清理开发容器与卷
	docker compose -f deploy/docker-compose/docker-compose.dev.yml down -v
