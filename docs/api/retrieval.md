# 检索 API

> **基线日期**：2026-10-28

## `POST /api/v1/retrieval/search`

混合检索接口。仅返回 `index_ready = true` 的切片。

### 请求体

```json
{
  "query": "自适应混合检索",
  "kb_type": "tutorial",
  "top_k": 10,
  "query_embedding": [0.1, 0.2],
  "use_global_index": false,
  "fulltext_weight": 0.6,
  "vector_weight": 0.4
}
```

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `query` | string | 是 | 用户查询 |
| `kb_type` | string | 否 | 分区过滤；空且 `use_global_index=true` 时走全局物化视图 |
| `top_k` | int | 否 | 默认 10，最大 50 |
| `query_embedding` | float[] | 否 | 1024 维向量；提供后与全文做 RRF 融合 |
| `use_global_index` | bool | 否 | 跨 `kb_type` 检索（走 `kb_global_index` 物化视图，O-17） |
| `include_cold` | bool | 否 | 是否包含 `storage_tier=cold` 文档（默认 false，O-18） |
| `skip_cache` | bool | 否 | 为 `true` 时跳过 L1/L2 |

缓存说明见 [`cache.md`](cache.md)。
查询分类见 [`classifier.md`](classifier.md)。

## `POST /api/v1/retrieval/diagnose`

检索诊断接口。用于排查 Agent 问答无结果，返回 Query4D、cache、waterfall、top chunks 与 **检索分级 gate**（`useful` / `score` / `threshold` / `reason`；对话侧已演进为 A/B/C/D `retrieval_grade`）。

请求体同 `/retrieval/search`，建议传 `query`、`kb_type`（或 `tenant_id`+`kb_id`）、`top_k`、`workspace_context`。

响应核心字段：

| 字段 | 说明 |
|------|------|
| `query_semantic_4d` | 当前 query 的 4D 解析结果 |
| `retrieval_trace` | 4D waterfall tier/suffix、实体对齐、scene/style 对齐、`retrieval_grade` |
| `gate` | `useful`、`score`、`threshold`、`reason`（与对话 `meta.gate_*` 一致） |
| `top_chunks` | 前 5 个候选的分数、标题、anchors、snippet、`entity_matched` |

### GrayDraft 3.5 语义身份注入

- 切片入库时写入 `kb_chunks.search_text`（标题 + 实体/extensions + 正文），全文检索优先使用该字段。
- 检索前按分区 `extensions` / 标题 / tags 做**别名扩展**。
- Rerank 输入为 `标题 | 实体 | 正文` 结构化 passage。
- 对话有效命中使用综合置信度与 **检索分级 A/B/C/D**：`EvaluateRetrievalGrade`（`confidence/gate.go`）；`Final = Rerank×0.6 + Vector×0.3 + TitleHitBonus(0.15)`；A/B 为高/中可信，C 为低可信稀疏注入，D 为无 chunk。

存量数据回填：`make backfill-search-text`（或 `./deploy/scripts/backfill-search-text.sh`）。

### 响应

```json
{
  "query": "自适应混合检索",
  "kb_type": "tutorial",
  "top_k": 10,
  "total": 2,
  "strategy": "fulltext",
  "catalog_version": 1,
  "chunks": [
    {
      "id": "b0000000-0000-4000-8000-000000000002",
      "doc_id": "a0000000-0000-4000-8000-000000000002",
      "kb_type": "tutorial",
      "content": "...",
      "score": 0.42,
      "chunk_index": 0,
      "source": "fulltext"
    }
  ]
}
```

### 策略说明

| `strategy` | 条件 |
|------------|------|
| `fulltext` | 无向量（Gateway 不可用或未开启 auto_embed） |
| `vector` | 仅有向量权重 |
| `hybrid_rrf` | Gateway 自动 Embedding + 全文 RRF 融合 |
| `hybrid_rrf+rerank_l1_l2` | 召回后经 Worker L1→L2 重排 |
| `...+rerank_skip` | 重排失败，降级为召回截断结果 |
| `global_index` | `use_global_index` 且未指定 `kb_type` |

Core 默认 `auto_embed: true`，会调用 Gateway `POST /v1/embeddings`；`rerank_enabled: true` 时调用 Worker `/v1/rerank/l1` 与 `/l2`。

## `GET /api/health`

服务与数据库连通性检查。
