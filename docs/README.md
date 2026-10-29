# GrayDraft 文档中心

> **基线日期**：2026-10-28

文档描述 GrayDraft 的当前实现，以代码为准。产品介绍见仓库根目录 [README](../README.md)。

## 推荐阅读顺序

| 顺序 | 文档 | 适合谁 |
|------|------|--------|
| 1 | [架构 · 系统逻辑总览](architecture/系统逻辑总览.md)（**§0 整体架构图**） | 所有人：服务边界、数据流、状态机 |
| 2 | [Agent 实现说明](architecture/Agent流程技术实现方案.md) | 查找、作答、证据层和笔记 |
| 3 | [本地开发指南](development.md) | 开发者：启动、调试、端口 |
| 4 | [部署手册](deployment.md) | 运维：Compose、Nginx、环境变量 |
| 5 | [API 索引](#api-文档) | 集成方：HTTP 契约与示例 |
| — | [学习目录](learning/README.md) | 按入库、检索、对话阅读代码 |

## 学习目录

| 文档 | 内容 |
|------|------|
| [learning/README.md](learning/README.md) | 按入库、检索、对话阅读代码 |
| [知识点规划](learning/00-overview/learning-map.md) | 工程知识点与阅读路线 |
| [动手路径](learning/05-practice/learning-roadmap-ai-agent.md) | 从启动到一次带引用的讲解 |
| [BFF](learning/04-architecture/05-bff.md) · [GrayDraft BFF 代理](learning/04-architecture/06-bff-proxy-capabilities.md) | 工作台如何调用 Core |

## 架构与设计

| 文档 | 内容 |
|------|------|
| [系统逻辑总览](architecture/系统逻辑总览.md) | **核心**：入库 / 检索 / 对话 / 智能导入 / 多模态 |
| [设计理念](architecture/设计理念.md) | AI 为操作主体；质量靠自动门禁与使用约定，不靠逐条人审 |
| [Agent 实现说明](architecture/Agent流程技术实现方案.md) | `edu_workspace_v1` 的 Loop、工具、证据层与记忆 |
| [架构决策记录 (ADR)](architecture/README.md) | Monorepo、pgvector、JetStream、缓存等决策 |
| [性能优化 1.0 方案](architecture/performance-optimization-v1.md) | 导入流水线、Embedding 微批、检索缓存、索引与可观测性优化 |
| [PROJECT_STRUCTURE.md](PROJECT_STRUCTURE.md) | 仓库目录与 Core/Worker 分层速查 |
| [问答可靠性治理](architecture/agent-import-chat-reliability.md) | 导入→问答断点与排障 |
| [roadmap.md](roadmap.md) | 已落地能力与后续项 |

## 功能实现

| 文档 | 内容 |
|------|------|
| [智能快速导入](features/smart-import.md) | 多模态预处理、LLM 标题、拆分 2.0、置信度、UGC 提交 |
| [多模态内容处理](features/multimodal.md) | VL/OCR 分流、缓存、pHash、双向量、工作区 Prompt |
| [检索与 Agent v2 对话](features/retrieval-and-chat.md) | 混合检索、Agent Runtime、SSE、AnswerGate、kb_only |
| [GrayDraft 集成](integrations/graydraft.md) | BFF 代理、工作区分区、Agent 设置与日志 |
| [工作区背景](features/workspace-background.md) | 结构化画像、导入/向量化/问答注入 |

## API 文档

| 文档 | 端点概要 |
|------|----------|
| [tenants.md](api/tenants.md) | 多租户 `tenant_id` + `kb_id`（PaaS API） |
| [ugc.md](api/ugc.md) | `POST /api/v1/ugc/notify`、任务轮询 |
| [extensions.md](api/extensions.md) | 业务扩展字段 `extensions` |
| [smart-import.md](api/smart-import.md) | `POST /api/v1/ugc/smart-import` |
| [retrieval.md](api/retrieval.md) | `POST /api/v1/retrieval/search` |
| [classifier.md](api/classifier.md) | `POST /api/v1/classifier/preview` |
| [agent-v2.md](api/agent-v2.md) | **`POST /api/v2/agent-runs`** + SSE 事件流（当前对话入口） |
| [chat.md](api/chat.md) | ~~`POST /api/v1/chat/stream`~~ 已下线（历史参考） |
| [cache.md](api/cache.md) | 检索三级缓存 |
| [answer-cache.md](api/answer-cache.md) | 答案缓存 |
| [jetstream.md](api/jetstream.md) | UGC 消息流与 DLQ |
| [dlq.md](api/dlq.md) | Admin DLQ 重放/丢弃 |
| [worker.md](api/worker.md) | Worker HTTP 能力 |
| [gateway.md](api/gateway.md) | 模型网关 Embedding / LLM / Rerank |
| [models.md](api/models.md) | 模型与 Provider 配置 |
| [embedding.md](api/embedding.md) | DashScope Embedding（O-13） |
| [console.md](api/console.md) | 管理后台 Admin API |
| [agent-eval.md](api/agent-eval.md) | Agent 评估回放与成本看板 |
| [ragops.md](api/ragops.md) | Prometheus / Grafana / 自愈 |

**Core 路由注册**（实现为准）：`internal/core/cmd/gateway/main.go`。

## 测试与样例

| 路径 | 说明 |
|------|------|
| [tests/README.md](../tests/README.md) | 集成、性能与回归测试入口 |

## 仓库根目录速查

```
GrayDraft/
├── internal/core/      # Go：Agent Runtime、检索、缓存、UGC、智能导入、多模态
├── internal/worker/    # Python：解析、分块、索引、QA、NATS 消费
├── internal/gateway/   # Go：Embedding / LLM / Rerank
├── internal/console/   # React + Go BFF 管理面
├── internal/ops/       # RAGOps（可选）
├── deploy/             # Docker、Nginx、迁移、监控
├── protos/             # gRPC 契约（buf）
└── docs/               # 本目录
```
