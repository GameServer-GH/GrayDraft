# Agent v2 对话与 SSE

> **定位**：理解 `POST /api/v2/agent-runs` 控制面 + SSE 事件流；BFF 如何映射为前端 `meta/thinking/chunk/done`  
> **前置**：[检索链路](../02-pipeline/02-retrieval.md)、[AI Loop 机制](../01-foundations/02-agent-loop.md)
> **代码路径**：`internal/core/service/agent/`（Runtime）· `internal/core/handler/agent.go`  
> **API 契约**：[agent-v2.md](../../api/agent-v2.md)  
> **历史说明**：`POST /api/v1/chat/stream` 已下线；本文档原名为「SSE RAG 对话」，现以 Agent v2 为准。

---

## 0. 对话全景

```
Client / BFF
  │ POST /api/v2/agent-runs {prompt, policy_version, conversation_id, ...}
  │ Header: X-Tenant-ID, X-Actor-ID, X-Authorized-KBs
  ▼
Core Handler → 创建 agent_runs 记录 → JetStream 发布
  ▼
Agent Launcher（异步 Worker）
  │ 1. 加载 Memory（Redis 短期 + PG 长期 + 会话摘要）
  │ 2. Agent Loop（Planner → Tool / Final）
  │ 3. knowledge.lexical / knowledge.semantic / knowledge.read / knowledge.expand → Hybrid 或章节/实体证据
  │ 4. AnswerGate / GroundedAnswerValidator
  │ 5. append-only 事件写入 agent_run_events
  ▼
Client
  │ GET /api/v2/agent-runs/{run_id}/events/stream?after_seq=0
  ▼
SSE: run.started → step.* → tool.* → answer.delta × N → run.completed
     （BFF 映射为 meta / thinking / chunk / done）
```

**与旧 Chat 的区别**

| 维度 | 旧 `chat/stream` | Agent v2 |
|------|------------------|----------|
| 架构 | 单体 RAG Service | Loop + Tool + 事件溯源 |
| 检索 | Service 内直接调用 | `knowledge.semantic` 工具 |
| 持久化 | 无 Run 状态 | `agent_runs` + append-only events |
| 断线续传 | 无 | `after_seq` SSE 回放 |
| 多步工具 | 无 | 支持 knowledge.lexical 等 R0/R1/R2 工具 |

---

## 1. 控制面：创建 Run

### 1.1 请求示例

```bash
curl -X POST http://localhost:18080/api/v2/agent-runs \
  -H 'Content-Type: application/json' \
  -H 'X-Tenant-ID: demo' \
  -H 'X-Actor-ID: learner' \
  -H 'X-Authorized-KBs: tutorial,paper' \
  -d '{
    "agent_id": "edu",
    "policy_version": "edu_workspace_v1",
    "idempotency_key": "learn:001",
    "prompt": "top_k=5\nchat_mode=hybrid\n什么是 index_ready？"
  }'
```

| 字段 | 说明 |
|------|------|
| `policy_version` | 不可变策略，默认 `edu_workspace_v1` |
| `prompt` | 用户问题；BFF 常在首部注入 `top_k` / `chat_mode` |
| `conversation_id` | 可选，关联多轮 Memory |
| `idempotency_key` | 幂等键，重复创建返回已有 Run |

### 1.2 响应 `202 Accepted`

```json
{
  "run_id": "uuid",
  "trace_id": "uuid",
  "status": "queued",
  "policy_version": "edu_workspace_v1"
}
```

---

## 2. SSE 事件流

### 2.1 订阅

```bash
curl -N "http://localhost:18080/api/v2/agent-runs/{run_id}/events/stream?after_seq=0" \
  -H 'X-Tenant-ID: demo' \
  -H 'X-Actor-ID: learner'
```

### 2.2 Core 原生事件

| type | 触发时机 | 关键 payload |
|------|----------|--------------|
| `run.started` | Run 入队/开始 | `run_id`, `policy_version` |
| `step.started` | Loop 每一步 | `step_id`, `step_index` |
| `planner.thought` | Planner 短摘要（脱敏） | `message` |
| `tool.started` / `tool.completed` | 工具调用 | `name`, `arguments`, `output` |
| `llm.reasoning.delta` | 模型原始思考链 | 仅审计/Trace，**不**映射前端 |
| `answer.generating` | 开始生成最终答案 | — |
| `answer.delta` | Token 级流式增量 | `message` |
| `answer.validated` | 门禁通过 | gate JSON |
| `answer.rejected` | 门禁拒答 | `sentence_labels` |
| `run.completed` | 成功终态 | 最终答案 |
| `run.failed` / `run.cancelled` | 失败/取消 | `message` |

### 2.3 BFF 映射（GrayDraft）

产品层仍消费 `meta | thinking | chunk | done | error`，由 BFF 转换：

| Core v2 事件 | BFF legacy 事件 | 说明 |
|--------------|-----------------|------|
| `run.started` + 策略 | `meta` | `strategy: agent_v2` |
| `planner.thought` / `step.started` | `thinking` | 可展开思考过程 |
| `answer.delta` | `chunk` | 流式正文 |
| `answer.validated` / `run.completed` | `done` | 含 citations / confidence |
| `run.failed` | `error` | — |

代码：`internal/graydraft/pkg/nexus/chat_stream.go` · `internal/graydraft/pkg/nexus/chat_stream.go`

---

## 3. RAG 策略（Runtime 内部）

### 3.1 教育检索工具

`knowledge.lexical` 与 `knowledge.semantic` 调用 Hybrid Retriever，与 [检索链路](../02-pipeline/02-retrieval.md) 共用同一实现。`knowledge.read` 读取相邻块，`knowledge.expand` 沿实体边扩展。输出含 `retrieval_grade`、`coverage_score`、`hits`，供 AnswerGate 使用。

### 3.2 笔记

`note.draft` 在审批通过后进入现有入库流水线，笔记类型为 `note`。

### 3.3 chat_mode 与分级（逻辑仍有效）

检索完成后仍走 A/B/C/D 分级与注入策略（实现在 `confidence/gate.go`，由 Runtime 的 AnswerGate 消费）：

| Grade | 典型行为 |
|-------|----------|
| A / B | 强证据 RAG 流式生成 |
| C | 稀疏上下文 |
| D | 拒答或通用 LLM（视 policy） |

`chat_mode=hybrid` / `kb_only` 通过 prompt 前缀或 BFF 注入，由 Launcher 解析。

---

## 4. 答案缓存与 Memory

### 4.1 答案缓存

`service/chat/answer_cacheable.go` 等模块仍服务 Runtime：`catalog_version` + grade 感知的答案缓存逻辑与旧链路一致。详见 [缓存体系](../04-architecture/03-cache.md)、[answer-cache.md](../../api/answer-cache.md)。

### 4.2 会话 Memory

| 层级 | 实现 |
|------|------|
| 短期 | Redis 最近消息 + 24K 字符预算 Compact |
| 长期 | `memory_items` + pgvector 语义召回 + 关系 RRF |
| 同意/冲突 | 显式同意策略、冲突人工确认 API |

详见 [AI记忆机制 (Memory)](../01-foundations/04-memory.md)。

---

## 5. 可靠性：AnswerGate 与 Groundedness

- **AnswerGate**：组合 retrieval_grade、coverage、高风险阈值（0.80）决定是否拒答
- **GroundedAnswerValidator**：句级 citation、专名/数字一致性
- 拒答时发出 `answer.rejected`，审计字段见 [问答可靠性治理](../03-algorithms/05-reliability-governance.md)

---

## 6. 异常与排障

| 场景 | 行为 |
|------|------|
| 无授权 KB | Run Fail-Closed |
| JetStream 不可用 | Run 无法异步执行（强依赖，无进程内降级） |
| 工具超时/预算超限 | `run.failed`，Prometheus 指标 `graydraft_agent_runs_total` |
| SSE 断线 | 重连 `after_seq=N` 回放遗漏事件 |

联调脚本：`deploy/scripts/smoke-agent-v2.sh`

---

## 7. 学习练习

1. 用 `smoke_agent_v2.sh` 或 curl 创建 Run，标注每个 SSE 事件的 `type` 和 `seq`。
2. 对比 Core 原生 SSE 与 GrayDraft `/ai/chat/stream` 映射后的事件差异。
3. 阅读 `planner.go`：教育工具如何选择词法、语义、相邻章节和实体扩展？
4. 构造无 KB 授权的请求，观察 Fail-Closed 行为。
5. 断线后带 `after_seq` 重连，验证事件回放完整性。

---

## 8. 相关文档

- [agent-v2.md](../../api/agent-v2.md) · [Agent 流程技术实现方案](../../architecture/Agent流程技术实现方案.md)
- [检索链路](../02-pipeline/02-retrieval.md) · [问答可靠性治理](../03-algorithms/05-reliability-governance.md)
- [缓存体系](../04-architecture/03-cache.md) · [AI Loop 机制](../01-foundations/02-agent-loop.md)
- [bff-proxy-capabilities.md](../04-architecture/06-bff-proxy-capabilities.md) · [retrieval-and-chat.md](../../features/retrieval-and-chat.md)
