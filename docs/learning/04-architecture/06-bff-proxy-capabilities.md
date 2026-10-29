# GrayDraft BFF 代理能力详解

> 先读概念：[BFF 是什么？](05-bff.md)
> **架构总图**：[系统逻辑总览 §0.2](../../architecture/系统逻辑总览.md#02-graydraft-bff-五项代理能力)
> 集成契约：[integrations/graydraft.md](../../integrations/graydraft.md)
> 代码入口：`internal/graydraft/internal/server/` · `internal/graydraft/pkg/nexus/`

**GrayDraft BFF 代理**指的是运行在 `:8787` 的 Go 服务（`graydraftd`），专门为 GrayDraft Web / GrayDraft 等前端「量身定制」的中间层。

简单说：前端不再直接调 GrayDraft Core 的一堆 REST API，也不再自己拼 `tenant_id`、`kb_type`、`llm_override`、工作区 Prompt——**只调自己的 BFF**，由 BFF 去和 Core、Gateway 打交道。

```
GrayDraft Web (Vue3)
  → GrayDraft BFF (:8787)     ← 本文重点
    → GrayDraft Core (:18080)
      ↔ Gateway / Worker / PG / Redis / NATS
```

---

### 为什么 GrayDraft 需要 BFF？

GrayDraft Core 是 **PaaS 级 AI 中台**，接口面向「租户 + 知识库分区 + 原始 JSON 字段」。GrayDraft 的产品语言是 **工作区、Agent 设置、快速导入预览**——两边概念不一致。

如果让前端直连 Core，会遇到这些问题：

- **概念不匹配**：用户选的是「工作区 A」，Core 要的是 `tenant_id` + `kb_id` + `hint_kb_type`。
- **请求要拼太多**：每次 Smart Import 或 Chat 都要带上 `workspace_context`、`context_profile`、`llm_override`、工作区 Prompt。
- **长任务易断**：智能导入、VLM 识图可能跑几分钟；浏览器断开不应取消 Core 处理。
- **多模态前置**：问答前先把图片/视频转成文字，不应塞在 Vue 组件里。
- **产品细节外泄**：`settings.json`、本地工作区路径、Agent 日志不应暴露给浏览器直连 Core。

BFF 把这些「产品层脏活」收拢到服务端，前端只发业务语义请求。

---

### GrayDraft BFF 做什么？

它作为 GrayDraft 的专属后端，通常会：

- **映射**：`X-Graydraft-Workspace-Key` → `KnowledgeScope`（`tenant_id`、`kb_id`、`storage_partition`）。
- **注入**：从 `settings.json` 读取模型配置、工作区 Prompt、结构化背景，写入 Core 请求体。
- **前置处理**：图片/视频/URL → VLM 描述 → 清洗 → 拼进 `raw_text` 或 `query`。
- **协议适配**：前端 `callBackend('AgentSmartImport')` → Core `POST /api/v1/ugc/smart-import`；SSE 逐行透传。
- **长任务隔离**：Smart Import / Chat 用独立 context，用户关页面不中断下游。
- **体验兜底**：422 置信度不足时仍返回预览 + 友好 `error` 文案；入库后广播 `library-changed` 刷新星系图。
- **可观测**：按 `workspace_key` 写 Agent 日志，与 Core 的 `X-Trace-Id` 对齐排障。

**工作区与知识库的一一对应**（`knowledgeScopeForWorkspace()`）：

| 产品侧 | GrayDraft 侧 |
|--------|-----------|
| 1 个工作区 | 1 个 `kb_type` / `storage_partition` |
| Header `X-Graydraft-Workspace-Key` | 规范化后作分区名 |
| `OpenWorkspace` | `BindNexusKbType` 写入 `settings.json` |
| GrayDraft 默认 | `kb_id = default`（多租户见 [tenants.md](../../api/tenants.md)） |

**每次代理常注入的字段**：

| 增强项 | 来源 | 写入 Core |
|--------|------|-----------|
| 知识库作用域 | `settings.json` | `tenant_id`、`kb_id`、`hint_kb_type` |
| 模型路由 | `agent_model: auto` 或 `vendor:model` | `llm_override` |
| 工作区 Prompt | Agent 配置 Tab | `workspace_prompt_override` |
| 结构化背景 | `contextProfile` | `workspace_context` + `context_profile` |
| 链路追踪 | `X-Trace-Id` | 透传 |

`auto` 表示走 Gateway 默认路由；`vendorId:modelId` 则从工作区 `settings.json` 解析 API Key（见 `pkg/workspace/agent_llm.go`）。

---

### BFF 与「直连 Core」「API 网关」的区别

| | **直连 GrayDraft Core** | **GrayDraft BFF** | **Nginx API 网关** |
|---|---|---|---|
| 关注点 | AI 能力契约（检索、入库、SSE） | GrayDraft 产品语义 | 路由、TLS、限流、SSE 缓冲 |
| 谁维护 | Core 团队 | 产品侧（`internal/graydraft/`） | 运维 / 部署 |
| 工作区 | 前端自己拼 `kb_type` | Header 自动映射 | 不处理 |
| Prompt / 模型 | 前端或调用方注入 | 从 `settings.json` 注入 | 不处理 |
| 多模态前置 | 调用方自己做 | BFF 内 VLM 预处理 | 不处理 |
| 典型路径 | `POST /api/v2/agent-runs` + SSE | `POST /api/knowledge/chat/stream` | BFF 映射 v2 → legacy SSE |

实践中，GrayDraft 的请求常是：

`Vue 前端 → GrayDraft BFF (:8787) → Nginx (:8443) → Core (:18080) → Gateway / Worker`

GrayDraft 走同一条 BFF 路径，按工作区授权的 `tutorial` / `paper` / `note` 问答。

---

### 好处与代价

**好处**

- 前端只认识工作区和 `callBackend` 方法名，Core API 变更可收口在 BFF。
- 工作区配置、Prompt、模型密钥留在服务端，不进浏览器。
- 多模态与长任务逻辑集中，便于 Agent 日志与排障。
- 同一套 Core 可服务 GrayDraft、GrayDraft 等多个产品 BFF。

**代价**

- 多一层服务，部署需配置 `GRAYDRAFT_CORE_URL`。
- BFF 挂了则前端无法访问知识库（Core 仍可能独立存活）。
- 部分逻辑（如 VLM 识图）在 BFF 与 Core 各有一份，需对照文档理解边界。

---

### 一个具体例子：Agent 流式问答

场景：用户在工作区里输入「这次版本更新有哪些优化？」并附带一张活动截图。

**不用 BFF（前端直连 Core）**，客户端需要自己做：

1. 从本地读出 `nexusKbType`、`tenant_id`、`contextProfile`。
2. 若有图片：先调 Gateway 多模态接口，把识别结果拼进 `query`。
3. 解析 `agent_model`，构造 `llm_override`。
4. `POST /api/knowledge/chat/stream`（BFF 内部调 Core v2），自己消费 SSE，处理 `meta` / `thinking` / `chunk` / `done`。
5. 自己处理 Nginx 缓冲、超时、trace 日志。

**使用 GrayDraft BFF**，客户端只发：

```http
POST /api/knowledge/chat/stream
X-Graydraft-Workspace-Key: my-game-wiki
Content-Type: application/json
Accept: text/event-stream

{
  "query": "这次版本更新有哪些优化？",
  "chat_mode": "hybrid",
  "top_k": 5,
  "agent_model": "auto",
  "image_base64": "...",
  "image_mime_type": "image/png"
}
```

BFF 在服务端完成：识图 → 整理问题 → 注入工作区背景 → 调 Core SSE → 原样流式返回。前端 `streamKnowledgeChat()` 只处理事件回调。

---

### 五项代理能力分别解决什么？

下面五项对应 [graydraft.md](../../integrations/graydraft.md) 中的代理表，按「前端方法 → BFF 行为 → Core 接口」说明。

#### 1. `AgentSmartImport` — 智能快速导入

| 层 | 说明 |
|----|------|
| 前端 | `agentSmartImport({ raw_text, attachments, submit, … })` |
| BFF | `POST /api/AgentSmartImport` |
| Core | `POST /api/v1/ugc/smart-import` |

**BFF 多做的事**：可选媒体识图入正文；注入 `llm_override`、工作区 Prompt、`workspace_context`；长任务与客户端断开解耦；422 时附带预览与友好错误。

**Core 做的事**：多模态预处理 → 骨架扫描 → LLM 拆分 → 置信度门槛（0.55）→ `ugc.Notify` 异步入库。

学习路径：**A3 Agent 编排** · **A4 多模态**。详见 [smart-import.md](../../features/smart-import.md)。

---

#### 2. `RetrievalSearch` — 混合检索调试

| 层 | 说明 |
|----|------|
| 前端 | `retrievalSearch({ query, top_k })` |
| BFF | `POST /api/RetrievalSearch` |
| Core | `POST /api/v1/retrieval/search` |

**BFF 多做的事**：注入 `workspace_context` / `context_profile`，限定在当前工作区分区。

**用途**：Agent 调试台、检索可视化——不跑完整 Chat 也能看 chunk 分数与召回。

学习路径：**A2 混合检索**。详见 [retrieval-and-chat.md](../../features/retrieval-and-chat.md)。

---

#### 3. `ListKnowledgeEntries` 等 — 文档 CRUD 与入库轮询

| 前端方法 | Core 行为 |
|----------|-----------|
| `ListKnowledgeEntries` / `GetKnowledgeEntry` | 文档列表与详情 |
| `CreateKnowledgeEntry` / `UpdateKnowledgeEntry` | `ugc/notify` |
| `DeleteKnowledgeEntry` | 删除文档 |
| `GetIngestTask` | 轮询 `pending → completed` |
| `GetKnowledgeGraph` | 文档关系图 |

**BFF 多做的事**：作用域映射；Create/Update 后 SSE 广播 `library-changed`。

**与 Smart Import 的区别**：CRUD 是标准入库，无 LLM 拆分；Smart Import 是 Agent 感知的长文智能导入。

学习路径：**B1 微服务数据流** · **B2 异步流水线**。

---

#### 4. `POST /api/knowledge/chat/stream` — Agent v2 流式问答

| 层 | 说明 |
|----|------|
| 前端 | `streamKnowledgeChat()`，`Accept: text/event-stream` |
| BFF | `handleKnowledgeChatStream` → `nexus.Client.StreamChatV2` |
| Core | `POST /api/v2/agent-runs` → `GET .../events/stream` |

**BFF 多做的事**：多模态识图与问题整理；`chat_mode`（`hybrid` / `kb_only`）、`llm_override`、工作区背景；将 v2 事件映射为 legacy SSE（`meta/thinking/chunk/done`）；SSE 透传（`X-Accel-Buffering: no`）。

**Core 行为摘要**（`policy_version=edu_workspace_v1`）：

| 阶段 | 行为 |
|------|------|
| 创建 Run | `202` + `run_id`，JetStream 异步执行 |
| Loop | `knowledge.semantic` → 分级 → 流式 `answer.delta` |
| 门禁 | AnswerGate / Groundedness，可 `answer.rejected` |

前端 SSE 事件顺序（BFF 映射后）：`meta` → `thinking`（可选）→ `chunk` × N → `done`（或 `error`）。

学习路径：**A1 Agent v2 对话**。详见 [SSE-RAG对话](../02-pipeline/03-agent-v2-sse.md) · [agent-v2.md](../../api/agent-v2.md)。

---

#### 5. `GetMultimodalPromptDefaults` — Prompt 分层配置

| 层 | 说明 |
|----|------|
| 前端 | 设置页读取 Base + 工作区补充 |
| BFF | 合并 Core Base 与本地 `settings.json` |
| Core | `GET /api/v1/multimodal/prompt-defaults`（只读系统 Base） |

**Prompt 三层堆叠**：

```
最终 Prompt = Core DefaultBasePrompt
            + 工作区补充（settings.json，用户可编辑）
            + workspace_prompt_override（单次请求，Smart Import 时注入）
```

保存走 `SaveMultimodalWorkspacePrompt`，只写本地，不改 Core。

学习路径：**A4 Prompt / 多模态**。详见 [multimodal.md](../../features/multimodal.md)。

---

### 与学习路径怎么对照？

| BFF 能力 | 学习路径 | 建议验证 | 面试可讲 |
|----------|----------|----------|----------|
| `chat/stream`（→ Core v2） | A1 | `curl -N` 或 `smoke_agent_v2.sh` | Agent Loop 与 RAG 如何分层？ |
| `RetrievalSearch` | A2 | 固定 query 调 `top_k` | 为何用 RRF 融合？ |
| `AgentSmartImport` | A3、A4 | 导入长文+图片 | 0.55 置信度门槛含义？ |
| 文档 CRUD + Task | B2 | 轮询 `GetIngestTask` | `index_ready` 是什么？ |
| `GetMultimodalPromptDefaults` | A4 | 改工作区 Prompt 对比回答 | 三层 Prompt 各管什么？ |

完整 12 周节奏见 [学习路径 · AI Agent / 应用开发](../05-practice/learning-roadmap-ai-agent.md)。

---

### 排障提示

用 **`X-Trace-Id`** 关联 BFF 与 Core 日志：

| 侧 | 路径 |
|----|------|
| BFF | `{data}/logs/agent/{workspace_key}/{date}/*.log` |
| Core | `{AGENT_LOG_ROOT}/logs/agent/{date}/*_core.log` |

| 现象 | 常见原因 |
|------|----------|
| `missing X-Graydraft-Workspace-Key` | 未 `OpenWorkspace` |
| `GRAYDRAFT_CORE_URL not configured` | BFF 未连 Core |
| Smart Import 422 | 置信度不足，看 `confidence` / `import_allowed` |
| Chat 收不到 SSE | Nginx 需对 stream 路由关缓冲 |

---

### 相关文档

- [BFF 是什么？（通用概念）](05-bff.md)
- [GrayDraft 集成（契约速查）](../../integrations/graydraft.md)
- [工作区背景](../../features/workspace-background.md)
- [Agent 导入与问答可靠性](../../architecture/agent-import-chat-reliability.md)

---

*文档版本：v2.0 · 写法对齐 [05-bff.md](05-bff.md)；实现以 `internal/graydraft/` 为准*
