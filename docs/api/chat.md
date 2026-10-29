# Chat 流式 API（RAG）— 已下线

> **基线日期**：2026-10-28  
> **状态**：`POST /api/v1/chat/stream` 已从 Core Gateway 移除。新对话请使用 **[Agent v2 Run API](agent-v2.md)**。  
> GrayDraft 经 BFF `/ai/chat/stream` 自动路由至 v2；多模态仍走 GrayDraft BFF。

---

以下为历史文档，仅供迁移参考。

`POST /api/v1/chat/stream` — Server-Sent Events，完整 RAG 链路：答案缓存 → 检索（含 4D 瀑布）→ 检索分级 → Prompt 组装 → Gateway LLM 流式生成。

## 请求

```json
{
  "query": "什么是 index_ready？",
  "tenant_id": "tn_...",
  "kb_id": "default",
  "kb_type": "gd_abc...",
  "top_k": 5,
  "chat_mode": "hybrid",
  "session_id": "optional",
  "workspace_context": "扁平工作区背景文本",
  "context_profile": {
    "description": "领域说明",
    "entities": ["实体1"],
    "rules": ["规则1"]
  },
  "llm_override": { "api_key": "...", "base_url": "...", "model": "..." },
  "skip_cache": false,
  "skip_answer_cache": false
}
```

作用域：`tenant_id` + `kb_id`（PaaS）或 `kb_type`（legacy）；二者同时存在时优先 PaaS 解析。详见 [tenants.md](tenants.md)。

### 对话模式 `chat_mode`

| 值 | 说明 |
|----|------|
| `kb_only` | **仅知识库**：按检索分级 A/B/C/D 分支；有效命中时用 **fast 模型**整理检索片段为 **Markdown**；D 级返回「当前知识库未入录该知识点。」 |
| `hybrid` | **知识库 + LLM**（默认）：A/B 全量 RAG；C 稀疏注入；D 通用 LLM |
| 省略 | 等同 `hybrid` |

检索分级（A/B/C/D）取代旧版二元 gate；`meta.gate_score` / `gate_threshold` / `gate_reason` 仍保留供诊断。完整规则见 [`architecture/系统逻辑总览.md`](../architecture/系统逻辑总览.md#72-retrieval_grade-字段详解)。

## SSE 事件

每条消息格式：`data: {json}\n\n`

### 事件顺序

```
retrieval_grade → thinking → meta → chunk × N → done
```

答案缓存命中时：`meta → chunk → done`（无 retrieval_grade / thinking）。

### 事件类型一览

| `type` | 中文 | 说明 |
|--------|------|------|
| `retrieval_grade` | 检索分级 | 首个事件；A/B/C/D 等级、覆盖度、`chunks_summary` |
| `thinking` | 思考中 | 生成前过渡文案 |
| `meta` | 检索元数据 | 引用、策略、缓存、4D trace、门控分数 |
| `chunk` | 答案增量 | `delta` 流式文本 |
| `done` | 完成 | `answer_confidence`、`sentence_labels`、`warnings` |
| `error` | 错误 | `message`，通常 `confidence: 0` |

### `meta` 字段（检索元数据）

| 字段 | 说明 |
|------|------|
| `chat_mode` | `hybrid` / `kb_only` |
| `strategy` | 检索策略链，如 `hybrid_rrf+rerank_l1_l2` |
| `cache_hit` | 检索缓存 `l1` / `l2` / 空 |
| `answer_cache_hit` | 答案缓存 `l1` / `l2` |
| `query_intent` | `fact` / `concept` / `proper_noun` / `hybrid` |
| `fulltext_weight` / `vector_weight` | 本次权重 |
| `total` | 召回总数 |
| `chunks_used` | 注入 Prompt 的 chunk 数 |
| `confidence` | 检索置信度 0–1 |
| `gate_score` / `gate_threshold` / `gate_reason` | 门控诊断 |
| `retrieval_grade` / `coverage_score` / `grade_reason` | 分级信息 |
| `missing_topics` / `chunks_summary` | 覆盖度与子主题 |
| `message` | 路由说明，如 `hybrid: rag with kb context` |
| `sources` | 按 chunk 引用（含 `extensions`） |
| `citations` | 按 doc 聚合引用 |
| `query_semantic_4d` | Query 4D 标签 |
| `semantic_4d_filter_applied` | 是否 4D 硬过滤 |
| `retrieval_trace` | 瀑布 tier、实体对齐等 |
| `retrieval_chunk_previews` | Agent 日志用 snippet |

### `done` 扩展字段

| 字段 | 说明 |
|------|------|
| `answer_confidence` | 答案与 chunk 一致性（需 `chat.answer_self_check: true`） |
| `answer_confidence_detail` | 自检明细 JSON |
| `sentence_labels` | 逐句来源：`source` = `kb` / `supplement` |
| `warnings` | 矛盾检测等警告 |

**置信度**：

- `confidence`：检索综合分 `Rerank×0.6 + Vector×0.3 + TitleHitBonus(0.15)` + 工作区 bonus
- `answer_confidence`：答案自检分

实现：`internal/core/service/confidence/`、`internal/core/model/chat.go`。

### 示例

```
data: {"type":"retrieval_grade","retrieval_grade":"B","coverage_score":0.55,"message":"检索分级: B (中可信)"}

data: {"type":"thinking","retrieval_grade":"B","message":"正在基于 2 条中可信参考整合回答..."}

data: {"type":"meta","chat_mode":"hybrid","strategy":"hybrid_rrf+rerank_l1_l2","chunks_used":2,"confidence":0.68,"retrieval_grade":"B","sources":[...]}

data: {"type":"chunk","delta":"根据知识库"}

data: {"type":"done","done":true,"confidence":0.68,"answer_confidence":0.82,"strategy":"hybrid_rrf+rerank_l1_l2+hybrid_rag"}
```

## 答案缓存

命中时跳过检索与 LLM，直接回放缓存答案（`answer_cache_hit`: `l1` \| `l2`）。Key 含 `semantic_4d_fingerprint`。详见 [`answer-cache.md`](answer-cache.md)。

## 点赞 / 点踩反馈（更新知识置信度）

`POST /api/v1/chat/feedback`

对 Agent 回答评价时，调整本次引用 chunk / 文档的 `quality_score`（持久化知识置信度，影响检索排序与治理）。

```json
{
  "vote": "up",
  "query": "什么是 index_ready？",
  "kb_type": "tutorial",
  "chunk_ids": ["uuid-chunk-1", "uuid-chunk-2"],
  "confidence": 0.68
}
```

| 字段 | 说明 |
|------|------|
| `vote` | `up` 点赞（提高置信度）或 `down` 点踩（降低置信度） |
| `chunk_ids` | SSE `meta`/`done` 中 `sources[].chunk_id` |
| `doc_ids` | 可选，按文档作用于其全部 chunk |

| 投票 | chunk | 文档 quality 聚合 | 其它 |
|------|-------|-------------------|------|
| `up` | `+boost`（上限 1） | 取关联 chunk 的 **MAX** | 不删文档 |
| `down` | `-penalty`（下限 0） | 取关联 chunk 的 **MIN** | 低于阈值则**删除文档** |

响应含 `vote`、`docs[].old_quality` / `new_quality`、`deleted_doc_ids`（仅点踩可能非空）。

配置（`config.dev.yaml` → `chat`）：

| 项 | 默认 | 说明 |
|----|------|------|
| `feedback_upvote_boost` | `0.08` | 每次点赞 quality_score 增量 |
| `feedback_downvote_penalty` | `0.12` | 每次点踩 quality_score 减量 |
| `feedback_delete_below` | `0.35` | 点踩后文档质量低于此值则删除 |
| `answer_self_check` | `false` | 开启后 `done` 含 `answer_confidence` |

流式响应引用（含业务扩展字段）示例见 [`extensions.md`](extensions.md)。

## 降级

- Gateway LLM 不可用且 `search_only_fallback: true` 时，直接返回检索片段文本（`strategy` 含 `+search_only`）。
- `chat.llm_total_timeout_ms` 限制整次 LLM 调用超时。
- Nginx 需关闭缓冲，见 `deploy/nginx/dev/conf.d/gateway.conf` 中 `/api/v1/chat/stream`。

## Gateway

Core 调用 `POST {gateway}/v1/chat/completions`，`stream: true`。`kb_only` 使用 fast tier，`hybrid` RAG 使用 complex tier。开发环境默认 **pseudo-chat** 本地流式；配置 `OPENAI_API_KEY` 后走 OpenAI 兼容 API。
