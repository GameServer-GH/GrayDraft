# 项目目录结构（速查）

> **逻辑与数据流**请以 [`architecture/系统逻辑总览.md`](architecture/系统逻辑总览.md) 为准；本文仅作目录导航。Compose / Nginx / 端口见 [`deployment.md`](deployment.md)。

## 顶层

| 目录 | 说明 |
|------|------|
| `internal/core` | Go API：Agent Runtime v2、检索、UGC、智能导入、Admin |
| `internal/worker` | Python：入库流水线、NATS 消费、分块与索引 |
| `internal/gateway` | Go：Embedding / LLM / Rerank |
| `internal/graydraft` | GrayDraft BFF（工作区映射、v2 对话代理） |
| `internal/console` | 管理后台（React + BFF） |
| `internal/ops` | RAGOps（可选） |
| `pkg/` | 跨服务共享库 |
| `protos/` | gRPC 契约 |
| `deploy/` | Docker、Nginx、迁移、监控、`.env.*` |
| `tests/` | 集成测试、场景样例与检索回归 |
| `docs/` | [文档中心](README.md) |

## Core 分层（`internal/core`）

```
cmd/gateway/main.go     # HTTP 入口、路由注册
handler/                # HTTP 绑定（含 agent*.go）
service/
  agent/                # Agent Runtime v2：Loop、Launcher、Tool、Memory
  retriever/            # 混合检索（knowledge.semantic 底层）
  chat/                 # 分级/答案缓存内部模块（v1 HTTP 已下线）
  ugc/                  # 入库下发
  smartimport/          # 智能/拆分导入、LLM 标题
  multimodal/           # VL/OCR、缓存、多图合并
  classifier/           # 查询意图
  observability/agentlog/  # Agent 会话日志
  cache/                # 检索缓存 L1/L2/L3
  answercache/          # 答案缓存
  confidence/           # 导入/对话置信度、分级门控
  embedder/ reranker/ llmclient/
repository/postgres|redis|nats/
model/ config/
```

## Worker 分层（`internal/worker`）

```
api/main.py             # FastAPI
worker/pipeline/        # ingest、nats_consumer、jetstream_setup
worker/parsers/ chunkers/ qa_generator/ quality_gate/
worker/indexer/         # embedding 写库
worker/db/repository.py
```

## 入口与端口（dev）

| 服务 | 入口 | 宿主机端口 |
|------|------|------------|
| Core | `internal/core/cmd/gateway` | 18080 |
| Worker | `internal/worker/api/main.py` | 18001 |
| Gateway | `internal/gateway/cmd` | 18002 |
| GrayDraft BFF | `internal/graydraft/cmd/graydraftd` | 8787 |
| Console | compose `console` | 13000 |
| Nginx | `deploy/nginx` | 8443 |

端口全集：`deploy/config/dev/.env.ports`。
