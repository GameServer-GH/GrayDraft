# RAGOps 可观测性与自愈（O-19）

Core 暴露 **Prometheus** 指标；Ops 服务提供 **评估扫描 + 自动修复** 入口，与 3.0 §六「从观察到自愈」对齐。

## 指标端点（Core）

```
GET /metrics
```

详见下文「核心指标」；配置 `observability.metrics_enabled: true`。

## Admin API（Core · 需 JWT）

| 方法 | 路径 | 说明 |
|------|------|------|
| POST | `/api/v1/admin/global-index/refresh` | O-17：重建 `kb_global_index` 物化视图 |
| GET | `/api/v1/admin/global-index/stats` | 全局索引行数 / 向量覆盖 |
| POST | `/api/v1/admin/tier/apply` | O-18：将长期未访问文档标为 `cold` |
| GET | `/api/v1/admin/tier/stats` | 热/冷文档计数 |
| POST | `/api/v1/admin/ragops/scan` | O-19：扫描低质 chunk，创建 heal job |
| GET | `/api/v1/admin/ragops/jobs` | 列出 heal 任务 |
| POST | `/api/v1/admin/ragops/jobs/:id/run` | 执行 rechunk / demote |

### 全局索引（O-17）

- 物化视图 `kb_global_index`：跨 `kb_type` 热文档 snippet + **embedding**
- 检索：`use_global_index=true` 且未指定 `kb_type` → 全局全文 + 向量 RRF
- 分区检索 + `global_index_boost`：仍可用（见 [`classifier.md`](classifier.md)）
- 自动刷新：`global_index.auto_refresh_hours`（0=关闭）

```bash
curl -X POST http://localhost:18080/api/v1/admin/global-index/refresh \
  -H "Authorization: Bearer $TOKEN"
```

### 冷热分层（O-18）

- `kb_documents.storage_tier`：`hot` | `cold`
- 检索默认 **排除 cold**；`include_cold: true` 可搜冷数据
- 命中检索时更新 `last_accessed_at`
- `tier/apply`：超过 `storage_tier.cold_after_days`（默认 90 天）未访问 → `cold`，并刷新全局索引

### RAGOps 自愈（O-19）

闭环：

1. 用户 **点踩** → 记录 `ragops_chunk_strikes`
2. 连续 N 天达阈值 → 创建 `ragops_heal_jobs`（默认 action=`rechunk`）
3. Admin / Ops 执行 `run` → 通过 UGC 重新入库或 `demote` 降权

配置（`config.*.yaml`）：

```yaml
ragops:
  enabled: true
  strike_days: 3
  strike_threshold: 3
  quality_below: 0.45
  default_action: rechunk   # rechunk | demote
```

## Ops 服务（`internal/ops` · :18003）

转发 Core Admin API，便于定时任务 / Console 集成：

| 方法 | 路径 |
|------|------|
| POST | `/v1/global-index/refresh` |
| GET | `/v1/global-index/stats` |
| POST | `/v1/tier/apply` |
| GET | `/v1/tier/stats` |
| POST | `/v1/ragops/scan` |
| GET | `/v1/ragops/jobs` |
| POST | `/v1/ragops/jobs/{id}/run` |
| POST | `/v1/heal/run-all` | 扫描 + 批量执行 pending jobs |

环境变量（`deploy/config/*/`.env.ops`）：`CORE_URL`、`ADMIN_USER`、`ADMIN_PASSWORD`。

## 核心指标

| 指标 | 标签 | 说明 |
|------|------|------|
| `graydraft_http_requests_total` | method, route, code | HTTP 请求量 |
| `graydraft_retrieval_requests_total` | kb_type, cache_hit, query_intent, status | 检索次数 |
| `graydraft_retrieval_strategy_total` | strategy_prefix | 含 `global_index` 前缀 |
| `graydraft_chat_requests_total` | kb_type, answer_cache_hit, ... | Chat 会话 |
| `graydraft_chat_retrieval_gate_total` | kb_type, useful, reason | Chat useful gate 放行/拦截原因 |
| `graydraft_chat_waterfall_total` | kb_type, tier, suffix | Query4D waterfall 层级，含 `_wf_soft` |
| `graydraft_chat_llm_empty_total` | kb_type, chat_mode | LLM 空输出次数 |

## Prometheus + Grafana（dev）

```bash
docker compose -f deploy/docker-compose/docker-compose.dev.yml \
  --env-file deploy/config/dev/.env.ports \
  --profile monitoring up -d prometheus grafana core

curl -s http://localhost:18080/metrics | grep graydraft_retrieval
```

导入仪表盘：`deploy/monitoring/grafana/dashboards/graydraft-rag.json`

## 相关

- [`retrieval.md`](retrieval.md) — `use_global_index` / `include_cold`
- [`cache.md`](cache.md)、[`answer-cache.md`](answer-cache.md)
- [`classifier.md`](classifier.md)
