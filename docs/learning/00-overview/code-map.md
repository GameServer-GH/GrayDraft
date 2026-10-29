# GrayDraft 代码地图

上一篇：[系统总览](system-overview.md) · 下一篇：[术语表](glossary.md)

---

## 1. 先按请求读代码

不要从 `internal/` 第一层开始逐文件阅读。先选择一条外部请求，从入口跟到存储或模型，再回头补公共模块。

| 主线 | HTTP / 消息入口 | 核心实现 | 数据或下游 |
|------|-----------------|----------|------------|
| 入库 | `internal/core/handler/ugc.go` | `internal/core/service/ugc/`、`internal/worker/worker/pipeline/` | NATS、PostgreSQL、Gateway |
| 检索 | Core retrieval handler | `internal/core/service/retriever/`、`classifier/`、`reranker/` | PostgreSQL / pgvector、Redis、Gateway |
| Agent v2 | `internal/core/handler/agent.go` | `internal/core/service/agent/` | Run/Event Repository、Tool、Memory、SSE |
| 智能导入 | Smart Import handler | `internal/core/service/smartimport/`、`multimodal/` | UGC 入库主线 |
| BFF 代理 | `internal/graydraft/internal/server/` | `internal/graydraft/pkg/nexus/` | Core API |

路由装配从 `internal/core/cmd/gateway/main.go` 开始看。

---

## 2. 仓库分层

| 路径 | 阅读目的 |
|------|----------|
| `internal/core/handler/` | 确认 HTTP 契约如何进入业务层 |
| `internal/core/service/` | 理解 Agent、检索、缓存、可靠性等核心行为 |
| `internal/core/repository/` | 理解 PostgreSQL、Redis、NATS 的持久化边界 |
| `internal/core/model/` | 理解领域对象与 4D 语义模型 |
| `internal/worker/worker/` | 理解文档加工与索引流水线 |
| `internal/gateway/service/` | 理解模型能力路由与 Provider 选择 |
| `internal/graydraft/pkg/nexus/` | 理解产品侧工作区如何映射为 GrayDraft 作用域 |
| `deploy/` | 理解真实服务、端口、环境变量与依赖 |
| `protos/` | 理解内部 gRPC 契约 |
| `tests/` 与 `*_test.go` | 理解行为边界与回归要求 |

---

## 3. Agent Runtime 精读顺序

```text
handler/agent.go
  → service/agent/launcher.go
  → service/agent/runtime.go
  → planner.go / runner.go
  → tools.go / knowledge_tool.go
  → memory 相关实现
  → answer_gate.go / groundedness.go
  → Run/Event 持久化与 SSE
```

先读相邻测试，再进入实现：

- `runtime_test.go`：Loop 主行为
- `planner_test.go`：规划与停止条件
- `knowledge_tool_test.go`：检索工具契约
- `memory_test.go`、`long_memory_test.go`：记忆边界
- `answer_gate_test.go`、`groundedness_test.go`：回答门控
- `answer_stream_test.go`：流式回答

---

## 4. 检索精读顺序

```text
retriever/hybrid.go
  → classifier/
  → query4d 相关实现
  → repository/postgres 的全文 / 向量查询
  → RRF 融合
  → reranker/
  → confidence/ 与 Agent Answer Gate
```

不要把“召回、融合、重排、分级、回答门控”混为一个算法，它们处在不同阶段，失败策略也不同。

---

## 5. 本阶段自检

- [ ] 能从路由装配找到 Agent v2 handler
- [ ] 能指出入库的 Core 与 Worker 分界
- [ ] 能指出混合检索、模型 Gateway、缓存所在包
- [ ] 能为一个核心实现找到对应测试

继续：[术语表](glossary.md)。
