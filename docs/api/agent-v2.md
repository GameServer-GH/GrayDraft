# Agent v2 Run API

> **基线日期**：2026-10-28

新对话统一进入 Agent Runtime。`POST /api/v1/chat/stream` 已下线，请使用本文档的控制面 + SSE 事件流。

GrayDraft BFF 通过 `internal/graydraft/pkg/nexus/chat_stream.go` 将 v2 事件映射为前端兼容的 `meta | thinking | chunk | done | error` SSE。

## 认证头（必填）

| Header | 说明 |
|--------|------|
| `X-Tenant-ID` | 租户 ID，由 workspace / 认证上下文派生，禁止 body 覆盖 |
| `X-Actor-ID` | 调用主体（用户或服务账号，如 `edu-bff`） |
| `X-Authorized-KBs` | 逗号分隔的授权 `kb_type` 列表 |
| `X-Trace-Id` / `X-Request-ID` | 可选，跨服务 trace 关联 |
| `Authorization` | JWT 开启时：`Bearer <token>` |

## `POST /api/v2/agent-runs`

创建 Run 并异步执行（JetStream 队列）。

### 请求

```json
{
  "agent_id": "edu",
  "policy_version": "edu_workspace_v1",
  "idempotency_key": "edu:session:1730000000:abc",
  "conversation_id": "sess-optional",
  "deadline_seconds": 90,
  "prompt": "top_k=5\nchat_mode=hybrid\nInternVL 在这条链路里负责什么？"
}
```

| 字段 | 说明 |
|------|------|
| `policy_version` | 不可变策略版本，默认 `edu_workspace_v1` |
| `idempotency_key` | 幂等键，同键重复创建返回已有 Run |
| `prompt` | 用户问题；BFF 会在前部注入 `top_k` / `chat_mode` 提示 |

### 响应 `202 Accepted`

```json
{
  "run_id": "uuid",
  "trace_id": "uuid",
  "status": "queued",
  "policy_version": "edu_workspace_v1",
  "deadline_at": "2026-10-28T12:00:00Z"
}
```

## `GET /api/v2/agent-runs/{run_id}/events/stream`

SSE 回放 + 实时推送。Query：`after_seq=0`。

### 事件类型

| type | 说明 |
|------|------|
| `run.started` | Run 入队 |
| `step.started` | Loop 步开始 |
| `tool.started` / `tool.completed` | 工具调用 |
| `answer.generating` | 开始生成最终答案 |
| `planner.thought` | **面向用户的短规划摘要**（已脱敏，来自 `thought_summary`） |
| `llm.reasoning.delta` | 模型原始思考链（仅审计/Trace，**不**映射到前端） |
| `answer.delta` | **Token 级流式增量**（`payload.message` 为文本片段） |
| `answer.validated` | 门禁通过，`payload.message` 为 gate JSON |
| `answer.rejected` | 门禁拒答，含 `sentence_labels` / `hit_summaries` |
| `run.completed` | 终态成功，`payload.message` 为最终答案 |
| `run.failed` / `run.cancelled` / … | 终态失败 |

Gate JSON 字段（`answer.validated` / `answer.rejected`）：

- `answer_confidence`, `retrieval_grade`, `coverage_score`
- `high_risk`, `threshold_used`
- `sentence_labels[]` — `{sentence, source: kb|supplement}`（优先 NLI entailment，失败回退词重叠）
- `hit_summaries[]` — `{chunk_id, doc_id, title}`

## GrayDraft BFF 映射

| v2 事件 | 前端 SSE |
|---------|----------|
| `run.started` | `meta`（`strategy: agent_v2`）+ `thinking` |
| `step.started` | `thinking`（第 N 步规划） |
| `tool.started` / `tool.completed` | `thinking`（工具名可读化） |
| `tool.approval_required` | `thinking` |
| `answer.generating` | `thinking` |
| `planner.thought` | `thinking`（`thinking_phase: plan`，短摘要） |
| `llm.reasoning.delta` | （不转发，避免暴露内部 CoT / kb_type / 协议细节） |
| `answer.delta` | `chunk`（尽早出首 token，避免等 `run.completed`） |
| `answer.validated` | `meta`（含 `retrieval_grade`）+ `thinking` |
| `answer.rejected` | `thinking`（门禁拒答提示） |
| `run.completed` | `chunk`（仅未流式过时）+ `done` |
| 失败终态 | `error` |

### `thinking` 事件字段

```json
{
  "type": "thinking",
  "message": "正在调用「知识库检索」…",
  "thinking_phase": "tool",
  "tool_name": "knowledge.semantic",
  "step_id": 1
}
```

| 字段 | 说明 |
|------|------|
| `message` | 面向用户的进度文案 |
| `thinking_phase` | `run` / `step` / `plan` / `tool` / `validate` / `answer` |
| `tool_name` | 原始工具名（可选） |
| `step_id` | Loop 步序号（可选） |

前端在思考中展开进度列表，结束后收起为「思考过程」可展开标签。

配置见 GrayDraft `graydraft` 段：

| 字段 | 说明 |
|------|------|
| `agent_v2_enabled` | 文本对话走 v2 |
| `agent_policy_version` | 默认 `edu_workspace_v1` |
| `agent_actor_id` | JWT `sub` / `X-Actor-ID` |
| `agent_jwt_secret` | **与 Core `JWT_SECRET` 相同**，本地签发并自动轮换 `agent_service` Token |
| `agent_jwt_ttl_hours` | Token TTL，默认 12 |
| `agent_jwt` | 无 secret 时的静态 Token 兜底 |

Core 也可由 Admin 签发服务 Token：`POST /api/v1/auth/service-token`（需 Admin JWT）。

## 联调脚本

```bash
# Core v2
curl -s -X POST http://localhost:18080/api/v2/agent-runs \
  -H 'Content-Type: application/json' \
  -H 'X-Tenant-ID: tenant_edu' \
  -H 'X-Actor-ID: dev' \
  -H 'X-Authorized-KBs: tutorial' \
  -d '{"agent_id":"edu","policy_version":"edu_workspace_v1","idempotency_key":"dev-1","prompt":"InternVL 负责什么"}'
```

## 相关文档

- [Agent 实现说明](../architecture/Agent流程技术实现方案.md)
- [模型配置](models.md)
- [chat.md](chat.md) — 旧 Chat API（已下线）
