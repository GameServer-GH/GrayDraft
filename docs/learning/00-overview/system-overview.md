# GrayDraft 系统总览

上一篇：[Learning 首页](../README.md) · 下一篇：[代码地图](code-map.md)

---

## 1. GrayDraft 是什么

GrayDraft 是一个面向 AI 应用的独立知识与 Agent 服务。它把知识入库、混合检索、Agent Runtime、模型路由、缓存、可靠性治理和可观测能力放在同一套工程体系中，对产品侧提供可复用的知识与 Agent API。

一句话主链路：

```text
原始知识 → 解析与分块 → 向量化入库 → 混合检索
                                      ↓
用户问题 → Agent Run → Tool 调用 → 可靠性门控 → SSE 回答
```

当前对外对话入口是 Agent v2：

```text
POST /api/v2/agent-runs
GET  /api/v2/agent-runs/{run_id}/events/stream
```

旧的 `POST /api/v1/chat/stream` 已不再是当前对外主入口。

---

## 2. 五类核心服务

| 服务 | 代码位置 | 核心职责 |
|------|----------|----------|
| Core | `internal/core` | HTTP API、Agent Runtime、检索、UGC、缓存与治理 |
| Worker | `internal/worker` | 解析、质量门禁、分块、向量化、索引 |
| Gateway | `internal/gateway` | Embedding、Rerank、LLM 的多 Provider 路由 |
| GrayDraft BFF | `internal/graydraft` | 工作区作用域映射、字段注入、协议适配 |
| Console / Ops | `internal/console`、`internal/ops` | 管理控制台、评估、诊断与运维闭环 |

底层基础设施主要是 PostgreSQL + pgvector、Redis 和 NATS JetStream。

---

## 3. 三条必学主链路

### 3.1 入库

```text
ugc/notify 或 smart-import
  → Core 创建任务并投递 JetStream
  → Worker 质量门禁、解析、分块、Embedding
  → PostgreSQL / pgvector
  → index_ready
```

学习重点：异步任务状态、幂等、质量门禁、分块与索引何时完成。

### 3.2 检索

```text
检索请求
  → 意图分类与 Query4D
  → 全文 / 向量 / 结构化召回
  → RRF 融合
  → L1 / L2 Rerank
  → 分级、门控与结果返回
```

学习重点：不同召回源为何不能直接比较原始分数，RRF 与 Rerank 的职责边界，低质量结果如何降级或拒答。

### 3.3 Agent v2

```text
创建 Agent Run
  → JetStream 异步启动 Runtime
  → Planner / Loop 决定下一步
  → knowledge.semantic 等 Tool
  → Memory 与 Answer Gate
  → 持久化事件
  → SSE 回放 + 实时推送
```

学习重点：Run 状态、停止条件、工具治理、断线恢复和最终回答的可靠性。

---

## 4. 学习边界

`docs/learning/` 解释“为什么、如何工作、从哪里读代码”；以下目录承担权威参考：

| 目录 | 用途 |
|------|------|
| `docs/architecture/` | 当前架构、ADR、技术方案 |
| `docs/api/` | 请求、响应与事件契约 |
| `docs/features/` | 功能实现说明 |
| `docs/deploy/`、`deploy/` | 部署配置与运行事实 |
| `internal/` | 最终实现事实 |
| `tests/` 与各包测试 | 行为规格与回归边界 |

---

## 5. 本阶段自检

- [ ] 能用一句话说明 GrayDraft 的输入、处理过程与输出
- [ ] 能说出 Core、Worker、Gateway、BFF 的职责边界
- [ ] 能画出入库、检索、Agent v2 三条主链路
- [ ] 知道当前 Agent v2 的创建 Run 与 SSE 入口

继续：[代码地图](code-map.md)。
