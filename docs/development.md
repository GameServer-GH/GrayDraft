# 本地开发指南

系统整体逻辑见 [**architecture/系统逻辑总览.md**](architecture/系统逻辑总览.md)；文档索引见 [**README.md**](README.md)。

## 前置依赖

- Go 1.22+
- Python 3.11+
- Docker & Docker Compose
- [buf](https://buf.build/docs/installation)（Proto 代码生成）
- 可选：NVIDIA GPU + vLLM（本地小模型推理）

## 环境初始化

```bash
./scripts/setup-dev.sh
```

配置已在 `deploy/config/dev/.env.*` 并纳入 Git，`git pull` 后可直接 `make dev-up`（见 [`deploy/config/README.md`](../deploy/config/README.md)）。

## 服务开发

### Core（Go）

```bash
# 先启动 Postgres（或 make dev-up 仅基础设施）
make dev-up  # 仅 postgres/redis/nats 时可注释 compose 中其他服务

cd internal/core
CONFIG_PATH=config/config.dev.yaml \
  DB_HOST=localhost DB_PORT=25432 DB_PASSWORD=dev_password_123 \
  go run ./cmd/gateway
```

配置文件：`internal/core/config/config.dev.yaml`

检索 API 示例：

```bash
curl -X POST http://localhost:18080/api/v1/retrieval/search \
  -H 'Content-Type: application/json' \
  -d '{"query":"混合检索","kb_type":"tutorial","top_k":5}'
```

集成测试（需数据库）：

```bash
TEST_DATABASE_DSN="host=localhost port=25432 user=graydraft_admin password=dev_password_123 dbname=graydraft_kb sslmode=disable" \
  go test -tags=integration ./repository/postgres/...
```

### Worker（Python）

```bash
cd internal/worker
python3.11 -m venv .venv && source .venv/bin/activate
pip install -r requirements.txt

CONFIG_PATH=config/config.dev.yaml \
  DB_HOST=localhost DB_PORT=25432 DB_PASSWORD=dev_password_123 \
  NATS_ENABLED=false \
  uvicorn api.main:app --host 0.0.0.0 --port 18001 --reload
```

入库示例见 [`docs/api/worker.md`](api/worker.md)。

### Gateway（Embedding / LLM / Rerank API）

需配置 `DEEPSEEK_API_KEY`（见 `deploy/config/dev/.env.gateway`）。

```bash
cd internal/gateway
CONFIG_PATH=config/config.dev.yaml go run ./cmd

curl http://localhost:18002/health

curl -X POST http://localhost:18002/v1/embeddings \
  -H 'Content-Type: application/json' \
  -d '{"texts":["混合检索"],"model":"text-embedding-v4"}'
```

模型配置说明见 [`docs/api/models.md`](api/models.md)。

### 全链路检索（Core + Gateway + Worker）

```bash
# 终端 1: Gateway :18002
# 终端 2: Worker   :18001
# 终端 3: Core     :18080（确保 config 中 model_gateway / ai_worker URL 指向本地）

curl -X POST http://localhost:18080/api/v1/retrieval/search \
  -H 'Content-Type: application/json' \
  -d '{"query":"混合检索","kb_type":"tutorial","top_k":5}'
# strategy 预期含 hybrid_rrf+rerank_l1_l2
```

### UGC 异步入库（JetStream + Worker）

NATS 需 `-js` 与数据目录（compose 已挂载 `nats_data_dev`）。

```bash
docker compose -f deploy/docker-compose/docker-compose.dev.yml \
  --env-file deploy/config/dev/.env.ports up -d postgres nats redis worker core

curl -X POST http://localhost:18080/api/v1/ugc/notify \
  -H 'Content-Type: application/json' \
  -d '{
    "kb_type": "tutorial",
    "title": "教程说明",
    "content": "GrayDraft UGC 经 JetStream 持久化后由 Worker 异步入库。"
  }'
# 预期 channel: jetstream

curl http://localhost:18080/api/v1/ugc/tasks/<task_id>
curl -s http://localhost:28222/jsz?consumers=1 | head
```

详见 [`docs/api/ugc.md`](api/ugc.md)、[`docs/api/jetstream.md`](api/jetstream.md)。

### 查询分类器

规则分类器在检索前调整全文/向量权重；`hybrid` 意图可触发全局索引或跨区 boost。调试：

```bash
curl -X POST http://localhost:18080/api/v1/classifier/preview \
  -H 'Content-Type: application/json' \
  -d '{"query":"对比社区与产品文档的区别"}'

curl -X POST http://localhost:18080/api/v1/retrieval/search \
  -H 'Content-Type: application/json' \
  -d '{"query":"如何理解自适应混合检索","kb_type":"tutorial","top_k":5}'
# 响应含 query_intent、fulltext_weight、vector_weight
```

详见 [`docs/api/classifier.md`](api/classifier.md)。

### 三级缓存

检索默认走 L1 Ristretto → L2 Redis（版本感知 + 语义相似）→ L3 预测预取。配置见 [`docs/api/cache.md`](api/cache.md)。

```bash
# 第一次（未命中）
curl -X POST http://localhost:18080/api/v1/retrieval/search \
  -d '{"query":"混合检索","kb_type":"tutorial"}'

# 第二次（预期 cache_hit: l1）
curl -X POST http://localhost:18080/api/v1/retrieval/search \
  -d '{"query":"混合检索","kb_type":"tutorial"}'
```

### Agent v2 对话（Run + SSE）

```bash
# 需 gateway + core + postgres + redis + nats
RUN_ID=$(curl -sS -X POST http://localhost:18080/api/v2/agent-runs \
  -H 'Content-Type: application/json' \
  -H 'X-Tenant-ID: demo' \
  -H 'X-Actor-ID: learner' \
  -H 'X-Authorized-KBs: tutorial' \
  -d '{"agent_id":"edu","policy_version":"edu_workspace_v1","idempotency_key":"dev-1","prompt":"什么是 index_ready","input_config":{"top_k":5,"chat_mode":"hybrid"}}' \
  | python3 -c 'import json,sys; print(json.load(sys.stdin)["run_id"])')

curl -N "http://localhost:18080/api/v2/agent-runs/${RUN_ID}/events/stream?after_seq=0" \
  -H 'X-Tenant-ID: demo' \
  -H 'X-Actor-ID: learner'
```

GrayDraft 联调：`deploy/scripts/smoke-agent-v2.sh`

详见 [`docs/api/agent-v2.md`](api/agent-v2.md)、[`docs/api/answer-cache.md`](api/answer-cache.md)。

### RAGOps（Prometheus / Grafana）

```bash
# Core 指标
curl -s http://localhost:18080/metrics | grep graydraft_

# 可选：启动监控 profile
docker compose -f deploy/docker-compose/docker-compose.dev.yml \
  --env-file deploy/config/dev/.env.ports --profile monitoring up -d prometheus grafana
```

详见 [`docs/api/ragops.md`](api/ragops.md)。

### Console 管理面

```bash
cd internal/console/frontend && npm install && npm run dev
# http://localhost:5173 （API 代理到 Core :18080）

# 或 docker compose 启动 console 服务
```

详见 [`docs/api/console.md`](api/console.md)。

### Worker（Python）

```bash
cd internal/worker
python -m venv .venv && source .venv/bin/activate
pip install -r requirements.txt
uvicorn api.main:app --host 0.0.0.0 --port 18001 --reload
```

### 生成 Proto

```bash
make proto-gen
```

## 端口说明

开发环境统一使用非常用端口，完整映射见 `deploy/config/dev/.env.ports`，避免与本地已有服务冲突。

## 教育场景测试

场景样例、检索回归和 Agent 黄金集见 [`tests/README.md`](../tests/README.md)。

## 路线图

已完成里程碑与下一步优先级见 [`roadmap.md`](roadmap.md)。
