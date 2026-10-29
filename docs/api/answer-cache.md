# 对话答案缓存（Answer Cache）

> **基线日期**：2026-10-28

在 **检索三级缓存** 之上，缓存完整 RAG 答案，避免重复调用 LLM。

> **对话入口**：答案缓存仍由 Runtime 内部复用；对外 HTTP 为 [Agent v2](agent-v2.md)（`POST /api/v2/agent-runs`），不再使用已下线的 `chat/stream`。

## 层级

| 层级 | 实现 | 说明 |
|------|------|------|
| **L1** | Ristretto 进程内 | 精确 Key，默认 TTL 30min |
| **L2** | Redis `knex:ans:*` | 跨实例共享 |

## 版本感知 Key

```
MD5(normalize(query) | kb_type | top_k | catalog_version | latest_doc_updated_at | semantic_4d_fingerprint)
```

`semantic_4d_fingerprint` 与检索缓存一致，仅在启用 4D 预过滤且 query 标签非空时写入。

与检索缓存共用 `kb_catalog_version`：入库后版本递增，旧答案自动失效。

## 流程

```
Agent v2 Run（edu_workspace_v1）
  → Answer Cache Lookup（命中则跳过检索+LLM，直接流式回放）
  → 未命中：教育检索工具 → LLM 流式 → Store Answer
```

## 不写入 / 不回放缓存

以下答案**不会**写入 Answer Cache，已存在的同类坏缓存条目在 Lookup 时也会被忽略并重新走检索+LLM：

- 检索片段数为 0（`chunks_used == 0`）
- 正文命中「无相关信息」「未能检索到」「无法回答」等拒答话术（见 `chat.IsAnswerCacheable`）

## 配置

```yaml
answer_cache:
  enabled: true
  redis_enabled: true
  ttl_seconds: 1800
  l1_max_items: 5000
```

环境变量：`ANSWER_CACHE_ENABLED`、`ANSWER_CACHE_TTL_SECONDS`

## 跳过缓存

在 Agent v2 中，通过 prompt 前缀或 BFF 注入 `skip_answer_cache=true` / `skip_cache=true`（由 Launcher 解析）。

## SSE / 事件字段

BFF 映射后的 `meta` / `done` 仍可能携带：

| 字段 | 说明 |
|------|------|
| `answer_cache_hit` | `l1` \| `l2`（答案缓存命中） |
| `cache_hit` | 检索缓存层级 |
| `strategy` | 命中时为 `...+answer_cache` 或 `agent_v2` |

## 验证

```bash
# 创建 Run（同一 prompt 重复执行可观察缓存行为）
curl -sS -X POST http://localhost:18080/api/v2/agent-runs \
  -H 'Content-Type: application/json' \
  -H 'X-Tenant-ID: demo' \
  -H 'X-Actor-ID: learner' \
  -H 'X-Authorized-KBs: tutorial' \
  -d '{"agent_id":"edu","policy_version":"edu_workspace_v1","idempotency_key":"cache-test-1","prompt":"top_k=5\n什么是 index_ready"}'
```

## 相关

- [`cache.md`](cache.md) — 检索三级缓存
- [`agent-v2.md`](agent-v2.md) — 当前对话 API
- [`chat.md`](chat.md) — 旧 Chat API（已下线，仅供迁移参考）
