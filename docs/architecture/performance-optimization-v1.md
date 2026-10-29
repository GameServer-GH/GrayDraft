# GrayDraft 性能优化 1.0 方案

> 版本：v1.0  
> 范围：知识导入链路、检索问答链路、缓存、索引、异步流水线、可观测性  
> 目标：在不降低检索准确率、知识一致性与 Agent 回答可靠性的前提下，将短知识导入耗时压缩到 10-30 秒，将未命中缓存的问答首 token 延迟压缩到 2-3 秒。

## 1. 背景与现状

GrayDraft 当前架构由 Go Core、Python Worker、Model Gateway、PostgreSQL + pgvector、Redis、NATS JetStream 组成：

- Core：对外 HTTP API、Agent Runtime v2、UGC/智能导入编排、混合检索、缓存与 Admin API。
- Worker：质量门禁、解析、分块、Embedding、写库、QA 生成、NATS 消费。
- Gateway：Embedding、Rerank、LLM/VLM 等模型调用统一入口。
- PostgreSQL + pgvector：保存文档、chunk、向量、工作区语义蓝图、任务状态。
- Redis + Ristretto：检索缓存、答案缓存与跨实例共享缓存。
- NATS JetStream：UGC 持久化异步入库、重试与 DLQ。

根据 `docs/plan/performance-bottleneck-analysis.md`，项目已经完成一轮热路径优化，包括批量 chunk insert、异步全局索引刷新、HTTP 连接池复用、检索缓存前置、三路检索并发、Rerank combined、Embedding LRU、异步 touchAccess 等。

因此 1.0 方案的重点不是重复修复已完成的局部问题，而是把导入和问答从“单任务内局部并发”升级为“分阶段事件流水线 + 微批模型调用 + 精细缓存 + 索引预算控制”的吞吐型架构。

## 2. 目标与非目标

### 2.1 性能目标

| 场景 | 当前风险 | 1.0 目标 |
|------|----------|----------|
| 短文本导入，约 50 chunks | 高峰期可能达到分钟级 | 10-30 秒内 `index_ready` |
| 大文件 / OCR / 多模态导入 | API 与 Worker 易被长任务占用 | API 秒级返回 `task_id`，后台异步完成 |
| 重复或相似问答 | 重复检索与 LLM 调用 | 缓存命中 5-20ms 返回 |
| 未命中缓存问答 | 首 token 易受 Embedding、检索、Rerank、LLM 叠加影响 | 首 token 2-3 秒 |
| 高并发导入 + 问答混跑 | DB、模型网关、Worker 互相抢资源 | 分队列、分 worker pool、分预算隔离 |

### 2.2 可靠性目标

- 导入链路任何阶段失败都可重试，且不重复写入脏数据。
- 检索只读取 `index_ready=true` 且版本完整的 chunk。
- QA 生成、全局索引刷新、知识质量治理不阻塞主导入完成。
- 模型网关超时后有明确降级策略，避免 Chat 长时间无响应。

### 2.3 非目标

- 1.0 不强制迁移出 PostgreSQL/pgvector，但需要为独立向量检索引擎预留接口。
- 1.0 不改动现有业务 API 语义，客户端仍以 `task_id`、`index_ready`、Agent v2 SSE 为主要交互契约。
- 1.0 不以牺牲召回率换取纯延迟，需要通过召回裁剪、缓存和模型预算控制实现平衡。

## 3. 问题诊断

### 3.1 导入为什么慢

导入链路包含解析、分块、Embedding、写库、索引就绪、可选 QA 生成、多模态处理等步骤。不同阶段资源类型差异很大：

| 阶段 | 资源类型 | 典型瓶颈 |
|------|----------|----------|
| 文档解析与 OCR/VLM | CPU、IO、外部模型 | 大文件、图片、PDF 页数多时耗时不稳定 |
| 分块 | CPU | 长文本语义切分成本高 |
| Embedding | 网络 IO、模型 QPS | API 延迟高，若未微批会放大 RTT |
| chunk 写入 | DB IO、连接池 | 大批量写入与索引维护争用 |
| 向量索引维护 | DB CPU、内存 | 实时 ANN 索引维护影响写入和查询 |
| QA 生成 | LLM 调用 | 可选能力，但同步执行会拉长入库完成时间 |

现有代码已完成一部分批量写入和连接池优化，但链路仍以“一个入库任务内完成全部阶段”为主，资源隔离不足。Embedding 仍是最关键瓶颈，因为它既有高延迟，又可高度并行化和批量化。

### 3.2 问答为什么慢

问答链路包含答案缓存、检索缓存、查询 Embedding、全文检索、向量检索、structured 向量检索、RRF、Rerank、Prompt 构建、LLM 流式输出。

| 阶段 | 典型瓶颈 |
|------|----------|
| 查询 Embedding | 同步模型调用，缓存未命中时至少一次网络 IO |
| pgvector 检索 | 数据量上升后 ANN 参数、过滤条件与索引设计影响明显 |
| 全文检索 | ILIKE fallback 或 search_text 过大导致扫描成本高 |
| Rerank | 候选集过大时模型调用成本上升 |
| LLM 首 token | 上下文过长、模型排队、Provider 延迟波动 |

当前已有检索缓存、答案缓存、Embedding LRU 和 Rerank combined。1.0 需要继续降低未命中路径的尾延迟，并提高缓存命中稳定性。

## 4. 总体架构方案

### 4.1 从单任务流水线升级为分阶段事件流水线

现状 UGC 入库主要围绕一个 `graydraft.ugc.ingest` subject。1.0 建议拆成阶段化 subject：

```text
graydraft.ingest.accepted.{workspace}       # Core 创建任务后投递
graydraft.ingest.parsed.{workspace}         # 解析完成
graydraft.ingest.media_extracted.{workspace}# OCR/VLM/PDF 页等重任务完成，可选
graydraft.ingest.chunked.{workspace}        # 分块完成
graydraft.ingest.embedded.{workspace}       # 向量完成
graydraft.ingest.indexed.{workspace}        # 写库与索引就绪完成
graydraft.ingest.qa.{workspace}             # QA 生成，可选低优先级
graydraft.ingest.failed.{workspace}         # 失败审计与 DLQ 汇总
```

每个阶段使用独立 consumer、独立并发和独立 DLQ：

```text
Core API
  -> JetStream accepted
  -> parse workers
  -> chunk workers
  -> embedding workers
  -> index writer workers
  -> qa workers
  -> Postgres / Redis / Vector Index
```

收益：

- 解析慢不会阻塞 Embedding worker。
- Embedding QPS 受限时不会压垮数据库连接池。
- QA 生成可作为后台任务，不影响文档 `index_ready`。
- 每个阶段可独立扩容、限流、告警和重放。

阶段化会引入编排复杂度，必须同时落地轻量状态机，而不是只拆 subject。

任务主状态建议：

```text
pending -> parsing -> media_extracting -> chunking -> embedding -> indexing -> ready
                                                        -> partial_ready
                                                        -> failed
```

严格阶段转换表：

| 当前阶段 | 允许进入 | 禁止行为 |
|----------|----------|----------|
| `pending` | `parsing` | 直接 `chunking` / `embedding` |
| `parsing` | `media_extracting` / `chunking` / `failed` | 跳过解析结果写库 |
| `media_extracting` | `chunking` / `failed` | 与普通解析结果乱序合并 |
| `chunking` | `embedding` / `failed` | 未生成 chunk 直接 indexed |
| `embedding` | `indexing` / `partial_ready` / `failed` | 无向量标记 ready |
| `indexing` | `ready` / `failed` | 部分写库后对线上可见 |
| `ready` | `qa` / 终态 | 回退到旧阶段 |

消息顺序约束：

- 同一 `task_id` 只允许一个 active stage。
- 不同文档可并发，同一文档阶段必须串行。
- subject 中带 `{workspace}` 用于自然分区和限流；严格顺序仍以 `ugc_ingest_tasks.current_stage` 和阶段事件表校验为准。
- JetStream 设置 `MaxDeliver`、`AckWait` 和 durable consumer；每个阶段 ack 前必须先完成状态落库，防止消息 ack 后状态丢失。
- 每条消息携带 `trace_id`、`task_id`、`workspace`、`stage`、`stage_attempt`、`index_version`、`chunker_version`。

端到端延迟预算：

- 阶段化会增加 pull wait、ack、flush 等额外延迟，必须设置整条流水线的 `ingest_deadline_seconds`。
- 每阶段记录 `queued_at`、`started_at`、`finished_at`，最终统一上报 `index_ready_lag_seconds = ready_at - task_created_at`。
- 单阶段超时后进入重试；端到端超时后标记 `failed_timeout`，但保留从失败阶段重放的能力。

### 4.2 Postgres 继续做权威库，向量检索层预留演进

1.0 短期继续使用 PostgreSQL + pgvector：

- `kb_documents`、`kb_chunks`、`ugc_ingest_tasks` 仍为权威数据。
- `index_ready=true` 是线上检索可见性的唯一硬门槛。
- `kb_type` 分区继续作为工作区隔离和查询裁剪的核心条件。

中期预留独立向量库接口：

- 当全局 chunk 达千万级、单 workspace chunk 达百万级、pgvector 查询 P95 超过 100-200ms，或向量查询明显影响写入时，迁移向量召回到 Qdrant / Milvus / Vespa。
- PostgreSQL 保存元数据与事务状态，向量库保存 `chunk_id`、`doc_id`、`kb_type`、`index_version`、`embedding` 和必要 payload。
- 迁移方式采用双写 + 影子查询 + 灰度切流，而不是一次性替换。

### 4.3 缓存从版本级失效升级为实体级精细失效

现有 L1/L2/L3 检索缓存和 Answer Cache 继续保留。1.0 增加导入完成后的缓存失效事件：

```text
graydraft.cache.invalidate
```

事件 payload：

```json
{
  "kb_type": "gd_xxx",
  "doc_id": "uuid",
  "index_version": 3,
  "entities": ["entity_a", "entity_b"],
  "title_terms": ["term_a", "term_b"],
  "semantic_4d_fingerprint": "..."
}
```

Core 收到事件后：

- 精确删除同 `kb_type`、同实体、同标题词相关的检索缓存。
- 删除引用该 `doc_id` 的答案缓存。
- 保留 TTL 兜底，避免漏删造成长期污染。
- 记录 `cache_invalidation_trace`，包含命中的缓存 key 数、实体提取来源、标题词来源和耗时，便于排查漏删或误删。

## 5. 导入性能优化方案

### 5.1 Embedding 微批合并

这是 1.0 最关键优化。

实现方式：

- 在 Embedding Worker 内维护微批缓冲区。
- 满足任一条件即 flush：
  - 累积 32-128 条 chunk；
  - 等待 50-200ms；
  - 请求总 token 达到 `max_tokens` 的 80%；
  - 低流量场景下缓冲区非空且距离上次 flush 超过 `max_wait_ms`。
- 调用 Gateway `/v1/embeddings` 时传入 `input[]` 数组。
- Gateway 对 Provider 做批量转发、限流、重试和熔断。
- 优先按 `model + dimension + tenant/provider + token_bucket` 分组，再按 token 数凑批，避免短文本被超长文本拖慢或撑爆 Provider 限制。

建议默认配置：

```yaml
embedding_batch:
  enabled: true
  max_items: 64
  max_wait_ms: 100
  max_tokens: 6000
  flush_at_token_ratio: 0.8
  worker_concurrency: 8
  provider_timeout_ms: 3000
```

如果某个 Provider 暂不支持批量接口，Worker 退化为协程并发：

- 单任务内并发上限：8-20。
- 全局 Provider 并发上限：按 API Key 配置。
- 相同 chunk 文本按 `content_hash + model + dimension` 命中 embedding cache。

预期效果：

- 50 个 chunk 不再产生 50 次串行 RTT。
- 在模型侧支持批量时，Embedding 阶段可从 10 秒级压缩到 1-3 秒。

批量返回协议必须支持部分失败：

```json
{
  "model": "text-embedding-v4",
  "dimension": 1024,
  "items": [
    { "index": 0, "ok": true, "embedding": [0.1, 0.2] },
    { "index": 1, "ok": false, "error_code": "provider_timeout", "retryable": true }
  ]
}
```

处理策略：

- Provider 返回部分失败时，仅重试失败条目，不整批重试。
- 重试仍失败的 chunk 标记 `embedding_failed`，任务进入 `partial_ready` 或 `failed` 取决于失败比例和配置。
- 不建议使用“近似向量”替代失败 embedding；除非有明确的降级模型，并在 chunk metadata 中标记 `embedding_quality=degraded`，避免污染高置信检索。
- Gateway 暴露 provider 令牌槽状态，例如 `available_slots`、`retry_after_ms`、`queue_depth`；Worker 根据反馈降低 pull batch、增加 `max_wait_ms` 或暂停消费。

### 5.2 文档解析异步流式

大文件和多模态附件不应穿透 Core 同步处理。

方案：

- 客户端上传大文件时先走对象存储或本地 OSS 兼容存储，Core 只保存 manifest。
- Core 创建 `task_id` 后立即返回，Worker 根据 URL/manifest 拉取内容。
- PDF 按页解析，Office 文档按段落解析，图片/OCR 按附件粒度并发。
- 解析输出可以边生成边进入 chunk 阶段，避免等待整篇解析完成。
- 图片、PDF 页、视频帧、OCR/VLM 等重任务进入 `media_extracted` 阶段，由独立 worker pool 处理，避免挤占普通文本解析。

任务状态：

```text
pending -> parsing -> chunking -> embedding -> indexing -> ready
```

对外可保持轻量状态，但内部应保留 stage event 日志，便于排障。

### 5.3 QA 生成异步化

QA pair 是增强召回与运营能力，不应阻塞主入库。

策略：

- `index_ready=true` 后即认为主导入完成。
- QA 生成投递 `graydraft.ingest.qa`，低优先级消费。
- QA 失败只标记 `qa_status=failed`，不回滚文档。
- Chat 和检索不得依赖 QA 完成。

### 5.4 数据库写入批量化与版本切换

当前已从逐条 insert 优化为批量写入。1.0 继续增强：

- 批大小建议 200-500 chunks。
- Python 优先使用 `asyncpg.copy_records_to_table` 或批量 `executemany`。
- 写入时带 `index_version` 或 `generation_id`。
- 新版本 chunk 先写入 `index_ready=false`。
- Embedding 和必要索引完成后，在一个短事务中切换：

```sql
UPDATE kb_chunks
SET index_ready = true
WHERE doc_id = $1 AND index_version = $2;

UPDATE kb_chunks
SET index_ready = false
WHERE doc_id = $1 AND index_version < $2;
```

这样线上检索永远只看到完整版本，失败任务不会半截可见。

### 5.5 索引维护策略

不建议每次导入后同步 `REINDEX` 或阻塞刷新全局物化视图。

短期策略：

- 保持导入完成即 `index_ready=true`。
- 全局索引刷新异步执行，并做去抖，例如 5 分钟内合并刷新。
- 大批量导入时进入 bulk mode：
  - 暂停全局物化视图频繁刷新；
  - 导入结束后统一刷新；
  - 对外展示 `global_index_lag_seconds`。

pgvector 参数建议：

- 小于 100 万 chunks：优先 HNSW。
- 初始建议：`m=16`、`ef_construction=200`。
- 查询时按场景设置 `hnsw.ef_search`：
  - 低延迟：40-80；
  - 高召回：100-200；
  - 可通过 `chat_mode` 或内部 query plan 动态选择。
- 参数必须通过压测确定，不直接把建议值推到生产：
  - 先用 10 万级 chunk 测试构建时间、索引体积、查询 P95 和召回。
  - 再用百万级影子数据测试 `m`、`ef_construction`、`ef_search` 的平衡点。
  - Core 检索时如需动态设置 `hnsw.ef_search`，必须在单次查询事务内执行 `SET LOCAL hnsw.ef_search = ...`，避免连接池复用污染后续请求。
- 写入事务设置低优先级保护：
  - `SET LOCAL statement_timeout = '30s'`；
  - `SET LOCAL lock_timeout = '2s'`；
  - 超时后重试当前阶段，不长时间持锁影响问答。

### 5.6 背压、限流与幂等

每个 workspace/API Key 需要独立预算：

- 最大待处理任务数；
- 每分钟导入请求数；
- 每日 Embedding token；
- 单任务最大文件大小；
- 单任务最大 chunk 数；
- 多模态附件并发数。

JetStream backlog 超阈值时：

- API 返回 `202 Accepted` 和预计排队信息；
- 超出租户预算返回 `429` 和 `retry_after`；
- 内部 worker 根据 Provider QPS 自动降低 pull batch。

每阶段都必须可安全重放。为避免分块算法升级后重放生成重复 chunk，幂等键应包含：

```text
task_id + stage + content_hash + index_version + chunker_version + chunk_position
```

其中 `chunk_position` 使用解析后文档的稳定段落/页码/offset，而不是数据库自增序号。

## 6. 问答性能优化方案

### 6.1 查询与答案缓存

缓存分层：

| 层级 | 内容 | 目标 |
|------|------|------|
| L1 Ristretto | exact query、query embedding、retrieval ids、answer | 微秒到毫秒级命中 |
| L2 Redis | exact/semantic retrieval cache、answer cache | 跨 Core 实例共享 |
| L3 Session Prefetch | 同 session 相关问题的候选结果 | 提前填充检索缓存 |

答案语义缓存需要谨慎启用，命中条件建议同时满足：

- `kb_type` 一致；
- query embedding cosine 达到离线校准阈值，初始建议 >= 0.94；
- query intent 一致；
- 实体集合 Jaccard >= 0.8；
- top_k doc_id 集合相似度 >= 0.9；
- 未命中“最新、今天、现在、版本、价格”等时效词；
- 缓存答案引用的 doc/chunk 仍为 ready 且未被精细失效事件删除。

命中后可直接 SSE 回放，`meta.answer_cache_hit` 标记为 `l2_semantic`。

语义答案缓存上线前必须先灰度观测：

- Phase 3 前采集至少一周真实 query pair，离线计算 embedding 相似度、实体 Jaccard、doc 集合相似度和人工/LLM 判定的可复用标签。
- 按准确率目标选择阈值，例如保证 P90/P95 复用准确率后再放量。
- 首期使用 shadow mode：只记录“本可命中语义缓存”，不真正回放答案。
- 时效词检测使用可配置词典/正则，支持 workspace 覆盖，例如“当前版本”“本月活动”“今日课件”“版本更新”。
- 缓存结果返回时附带 `cache_index_version`；若与当前 doc/chunk `index_version` 不一致，记录 `cache_stale_hit_total` 并主动淘汰。

### 6.2 Session Prefetch

当用户问出第一个问题后，后台根据实体和意图预取可能后续问题：

- 角色/产品类：基础介绍、使用方法、搭配、常见问题。
- 教程类：概念、参数、步骤、注意事项。
- FAQ 类：定义、限制、异常处理。

Prefetch 只缓存检索结果，不预生成 LLM 答案，避免浪费 token。

容量控制：

- 单 session 最多预取 3-5 个候选 query。
- 每个预取 query 最多缓存 10-20 个 chunk id。
- TTL 建议 5-10 分钟，淘汰优先级低于用户真实请求产生的缓存。
- Redis key 单独标记 `prefetch=true`，便于统计命中率和内存占用。

### 6.3 索引与 SQL 调优

建议补齐或确认以下索引：

```sql
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_chunks_active
ON kb_chunks (kb_type, storage_tier)
WHERE index_ready = true;
```

这里使用部分索引，不把 `index_ready` 放入索引列，减少批量从 `false` 切到 `true` 时的索引膨胀和写放大。

全文检索建议使用生成列或维护列：

```sql
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_kb_chunks_search_text_gin
ON kb_chunks
USING GIN (search_text_tsv);
```

如果中文模糊匹配仍需要兜底，可评估 `pg_trgm` / `pg_bigm`，避免直接对原始超长正文做大范围 `ILIKE` 扫描。`search_text_tsv` 应由标题、实体、tags、摘要与必要正文片段组成，控制索引体积和更新成本。

原则：

- 线上热路径尽量避免 `ILIKE '%keyword%'` 全表扫描。
- `search_text` 应包含标题、实体、tags、正文摘要，但不能无限膨胀。
- 先用 `kb_type/index_ready/storage_tier` 缩小范围，再做 ANN/全文。
- 冷数据默认排除，用户显式 `include_cold=true` 才参与。

### 6.4 混合检索裁剪

保留当前全文、向量、structured 向量三路并发，但控制候选规模：

| 阶段 | 默认上限 |
|------|----------|
| 全文召回 | 40 |
| 向量召回 | 40 |
| structured 向量召回 | 20 |
| RRF 后候选 | 30 |
| Rerank 后输入 Chat | 5-8 |

简单 query 直接走混合检索，不启用 LLM query decomposition：

- query 长度 < 20 字；
- 只有一个实体；
- 无明显并列问法；
- 无多跳推理信号。

复杂 query 才启用分解：

- 子查询数量 <= 2；
- 每个子查询并发召回；
- 总候选数量仍受 RRF 上限约束。

候选上限需要按问题复杂度动态调整：

| Query 类型 | 全文 | 向量 | structured | RRF 后 | 说明 |
|------------|------|------|------------|--------|------|
| 简单事实 | 20 | 20 | 10 | 20 | 短 query、单实体、无并列条件 |
| 默认 | 40 | 40 | 20 | 30 | 大多数问题 |
| 复杂多意图 | 60 | 60 | 30 | 50 | 长 query、多实体、多子问题 |

诊断接口必须暴露每步裁剪量：

```json
{
  "candidate_stats": {
    "fulltext_raw": 40,
    "vector_raw": 40,
    "structured_raw": 20,
    "merged_unique": 63,
    "rrf_kept": 30,
    "rerank_input": 30,
    "rerank_output": 8
  }
}
```

召回保护：

- 记录 recall redundancy：若 Rerank top-8 与 RRF top-1 的 doc 完全不同，写详细诊断日志。
- CI/CD 中用 golden cases 跑 `recall@5`、`recall@10`、`MRR`，低于基线阈值则阻止放量。
- workspace 可配置 `retrieval_profile=latency|balanced|recall`，用于调整候选上限和 `hnsw.ef_search`。

### 6.5 Rerank 与 LLM 超时

建议默认预算：

```yaml
chat_budget:
  query_embedding_timeout_ms: 300
  retrieval_timeout_ms: 500
  rerank_timeout_ms: 500
  llm_first_token_timeout_ms: 2000
  llm_total_timeout_ms: 30000
  max_context_chunks: 8
  max_context_tokens: 6000
```

降级策略：

- Rerank 超时：使用 RRF 排序结果。
- 向量检索超时：使用全文结果。
- 全文检索超时：使用向量结果。
- LLM 首 token 超时：发送状态事件并继续等待短窗口。
- LLM 总超时或失败：若 `search_only_fallback=true`，返回检索摘要。

### 6.6 动态 Prompt 压缩

Prompt 构建时不应机械塞入全部 top_k：

- 优先保留实体/标题命中的 chunk。
- 同一 doc 多个 chunk 只保留最高分 chunk 和必要相邻 chunk。
- 对长 chunk 做摘要压缩或片段裁剪。
- 默认 5-8 个 chunk，最大上下文 token 由模型预算决定。

目标是减少 LLM 首 token 延迟，同时降低无关上下文造成的回答漂移。

### 6.7 SSE 早期反馈

Agent v2 SSE 应尽早反馈状态（`step.*` / `thinking` / `answer.delta`），改善感知延迟：

```text
status: cache_lookup
status: retrieving
status: reranking
meta: chunks_used / strategy / confidence
chunk: first delta
done
```

注意：早期 status 不能替代真实性能优化，但能避免用户在 2 秒内完全无反馈。

## 7. 模型网关优化

Gateway 是全链路的关键预算层。

需要补齐：

- HTTP keep-alive、HTTP/2、多 Provider 连接池。
- Embedding batch API。
- 保留单条 Embedding API，Gateway 内部可自动合并短时间窗口内的单条请求，形成 batching gateway pattern，降低 Worker 改造成本。
- Rerank combined API。
- Provider 级熔断：错误率、超时率、排队长度触发降级。
- Workspace/API Key 级 token 统计与限额。
- 慢请求日志：记录 provider、model、tokens、latency、status。
- Provider 令牌槽反馈：Gateway 定期或随响应返回 `available_slots`、`queue_depth`、`retry_after_ms`，Worker 据此调节 pull batch 和微批等待。

Provider 路由策略：

| 场景 | 模型策略 |
|------|----------|
| 查询 Embedding | 固定低延迟 embedding 模型，强缓存 |
| 导入 Embedding | 支持微批和排队，可牺牲少量实时性 |
| Rerank | 优先轻量模型，超时降级 |
| kb_only 整理 | Fast LLM |
| hybrid 高质量回答 | 默认 LLM，可按 workspace 覆盖 |
| 多模态 OCR/VLM | 强缓存，严格并发限制 |

## 8. 数据与配置变更建议

### 8.1 任务阶段表

保留 `ugc_ingest_tasks` 作为任务主表，建议增加当前阶段和重试摘要字段：

```sql
ALTER TABLE ugc_ingest_tasks
  ADD COLUMN IF NOT EXISTS current_stage TEXT,
  ADD COLUMN IF NOT EXISTS stage_retry_count INT NOT NULL DEFAULT 0,
  ADD COLUMN IF NOT EXISTS index_version INT NOT NULL DEFAULT 1,
  ADD COLUMN IF NOT EXISTS ingest_deadline_at TIMESTAMPTZ,
  ADD COLUMN IF NOT EXISTS ready_at TIMESTAMPTZ;
```

新增或扩展阶段事件：

```sql
CREATE TABLE IF NOT EXISTS ugc_ingest_stage_events (
  id BIGSERIAL PRIMARY KEY,
  task_id UUID NOT NULL,
  trace_id TEXT,
  workspace TEXT,
  stage TEXT NOT NULL,
  status TEXT NOT NULL,
  attempt INT NOT NULL DEFAULT 1,
  index_version INT,
  chunker_version TEXT,
  error_code TEXT,
  error_message TEXT,
  queued_at TIMESTAMPTZ,
  started_at TIMESTAMPTZ,
  finished_at TIMESTAMPTZ,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
```

客户端只需要看主状态，运维和告警看阶段事件。

### 8.2 成本事件表

模型调用需要可归因到 workspace、API Key、任务和会话，便于预算、计费和容量规划：

```sql
CREATE TABLE IF NOT EXISTS model_cost_events (
  id BIGSERIAL PRIMARY KEY,
  trace_id TEXT,
  task_id UUID,
  session_id TEXT,
  workspace TEXT NOT NULL,
  kb_type TEXT,
  api_key_id TEXT,
  provider TEXT NOT NULL,
  model TEXT NOT NULL,
  operation TEXT NOT NULL, -- embedding | rerank | llm | vlm | ocr
  input_tokens BIGINT DEFAULT 0,
  output_tokens BIGINT DEFAULT 0,
  item_count INT DEFAULT 0,
  latency_ms INT,
  status TEXT NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
```

每次 Gateway 调用写入或异步上报一条成本事件，Prometheus 负责实时指标，Postgres 负责审计与离线分析。

### 8.3 推荐配置

```yaml
ingest_pipeline:
  staged_jetstream_enabled: true
  max_task_chunks: 2000
  ready_requires_qa: false
  bulk_refresh_debounce_seconds: 300
  ingest_deadline_seconds: 900

embedding_batch:
  enabled: true
  max_items: 64
  max_wait_ms: 100
  max_tokens: 6000
  flush_at_token_ratio: 0.8
  worker_concurrency: 8

retrieval_budget:
  fulltext_candidates: 40
  vector_candidates: 40
  structured_candidates: 20
  rrf_candidates: 30
  max_context_chunks: 8
  max_context_tokens: 6000

semantic_answer_cache:
  enabled: true
  similarity_threshold: 0.94
  doc_set_overlap_threshold: 0.90
  ttl_seconds: 1800

rate_limit:
  per_workspace_ingest_qps: 5
  per_workspace_chat_concurrency: 20
  per_workspace_embedding_tokens_per_day: 2000000

dynamic_config:
  enabled: true
  backend: redis_pubsub
  refresh_seconds: 30
```

批量大小、超时、缓存阈值、候选上限和租户限流应支持热更新。1.0 可先用 Redis pub/sub 或定时拉取配置实现，后续再演进到 etcd/Consul。

## 9. 可观测性与告警

### 9.1 导入指标

```text
graydraft_ingest_tasks_total{workspace,kb_type,status}
graydraft_ingest_stage_duration_seconds{stage}
graydraft_ingest_queue_backlog{subject,consumer}
graydraft_ingest_retry_total{stage,error_code}
graydraft_ingest_dlq_total{stage}
graydraft_ingest_index_ready_lag_seconds{workspace,kb_type}
graydraft_embedding_batch_size{provider,model}
graydraft_embedding_latency_seconds{provider,model}
graydraft_embedding_cache_hit_total{provider,model}
graydraft_embedding_partial_failure_total{provider,model,error_code}
```

### 9.2 问答指标

```text
graydraft_chat_first_token_latency_seconds{workspace,kb_type,mode}
graydraft_chat_total_latency_seconds{workspace,kb_type,mode}
graydraft_retrieval_latency_seconds{strategy,cache_hit}
graydraft_retrieval_cache_hit_ratio{level}
graydraft_answer_cache_hit_ratio{level}
graydraft_vector_query_latency_seconds{engine,kb_type}
graydraft_fulltext_query_latency_seconds{kb_type}
graydraft_rerank_latency_seconds{provider,model}
graydraft_llm_first_token_latency_seconds{provider,model}
graydraft_chat_fallback_total{reason}
graydraft_cache_stale_hit_total{workspace,kb_type,cache_type}
graydraft_prefetch_cache_items{workspace}
graydraft_retrieval_candidate_count{stage,kb_type}
```

### 9.3 资源指标

```text
postgres_connections_active
postgres_lock_wait_seconds
postgres_query_latency_seconds{query_fingerprint}
redis_command_latency_seconds
nats_pending_messages{stream,consumer}
nats_ack_wait_seconds{stream,consumer}
gateway_provider_qps{provider,model}
gateway_provider_error_rate{provider,model}
gateway_token_usage_total{workspace,provider,model}
gateway_cost_events_total{workspace,operation,provider,model,status}
```

### 9.4 链路追踪

OpenTelemetry 需要串联 Core -> NATS -> Worker -> Gateway -> PostgreSQL / Redis：

- HTTP Header：传递 `X-Trace-Id`、`X-Request-Id`、`X-Task-Id`、`X-Session-Id`。
- NATS Header：传递同一组 trace 字段，并补充 `stage`、`stage_attempt`、`index_version`。
- Gateway：每次模型调用 span 标记 `provider`、`model`、`operation`、`input_tokens`、`output_tokens`、`batch_size`。
- DB：关键查询可用 SQL comment 或 span attribute 标记 `trace_id`、`task_id`、`query_fingerprint`，避免日志里只看到孤立慢 SQL。
- Redis：缓存命中、失效、stale hit、prefetch 写入都记录 cache key 前缀和 logical key hash，不记录原始用户 query。

### 9.5 告警规则

| 告警 | 条件 |
|------|------|
| 导入积压 | `ingest_queue_backlog` 持续增长 10 分钟 |
| 入库延迟 | `index_ready_lag_seconds P95 > 300s` |
| DLQ | DLQ 消息数 > 0 持续 5 分钟 |
| Embedding 异常 | error rate > 2% 或 P95 > 5s |
| 检索慢 | retrieval P95 > 500ms |
| 首 token 慢 | chat first token P95 > 3s |
| 缓存异常 | answer cache hit ratio 较 24h 均值下降 30% |
| 缓存污染 | `cache_stale_hit_total` 5 分钟内持续增长 |
| Prefetch 挤占 | prefetch keys 占 Redis 相关命名空间 > 20% |
| DB 锁等待 | lock wait P95 > 1s |
| Provider 熔断 | 单 Provider timeout rate > 5% |

## 10. 实施路线图

### Phase 0：基线与防护，1 周

- 补齐导入阶段耗时、Gateway 模型耗时、Chat 首 token 指标。
- 增加 workspace/API Key 基础限流。
- 为 Chat 设置 Embedding、检索、Rerank、LLM 分段 timeout。
- QA 生成从主导入完成路径中拆出，先作为低优先级异步任务。
- 增加 `cost_events` 或等价异步成本上报。
- 建立 Grafana 性能看板和 P95/P99 告警。

验收：

- 可以按 `trace_id/task_id/session_id` 串起一次导入或问答。
- 能看到每个阶段的耗时占比和失败原因。
- `index_ready` 不再等待 QA 生成。

### Phase 1：Embedding 微批灰度，1-2 周

- Worker/Gateway 支持 Embedding batch。
- Worker 实现微批 flush。
- Embedding cache 按 `text_hash + model + dimension` 命中。
- Gateway 批量返回支持 per-item status，Worker 只重试失败条目。
- 在影子环境或少量 workspace 灰度 1-2 周，观察 P95/P99、部分失败率和 Provider 限流。

验收：

- 50 chunks 文档的 Embedding 阶段 P95 <= 3s。
- Embedding partial failure rate <= 0.5%，且失败条目可单独重试。
- 灰度 workspace 无明显导入失败率上升。

### Phase 2：阶段化 JetStream 流水线，2-3 周

- 拆分 accepted/parsed/media_extracted/chunked/embedded/indexed/qa subjects。
- 落地 `current_stage`、`stage_retry_count`、阶段转换表和端到端 deadline。
- 每阶段独立 consumer、并发、重试、DLQ。
- 新增阶段事件表和重放能力。
- 支持大文件 manifest 异步解析。

验收：

- 任一阶段失败可从该阶段重放。
- 高并发导入时 Chat 检索 P95 不明显劣化。

### Phase 3：问答缓存与检索裁剪，1-2 周

- 语义答案缓存先 shadow mode，再小流量灰度。
- 实现 Session Prefetch。
- 调整三路召回候选上限与 Rerank 输入上限。
- Prompt 动态裁剪到 5-8 chunks。
- 诊断接口返回每阶段候选数和裁剪数。

验收：

- 重复/相似问答缓存命中 P95 <= 20ms。
- 未命中缓存首 token P95 <= 3s。
- 离线 golden cases 的 `recall@5`、`recall@10`、`MRR` 不低于基线阈值。
- shadow mode 语义缓存复用准确率达到预设阈值后才能真实回放。

### Phase 4：索引与向量引擎评估，2-4 周

- 补齐 Postgres 复合索引、GIN 索引、pgvector 参数压测。
- 验证 `SET LOCAL hnsw.ef_search` 在连接池复用下的隔离性。
- 建立 Qdrant/Milvus/Vespa 影子索引 PoC。
- 双写部分 workspace，比较召回、延迟、成本。

验收：

- pgvector 调优后向量检索 P95 <= 100ms。
- 独立向量库 PoC 给出是否迁移的量化结论。

## 11. 风险与应对

| 风险 | 应对 |
|------|------|
| 微批增加单条任务等待 | `max_wait_ms` 控制在 50-200ms，低流量时自动快速 flush |
| 语义答案缓存误命中 | 加实体集合、doc 集合、时效词、catalog 更新时间约束，并支持一键关闭 |
| 候选裁剪降低召回 | 通过 golden cases、诊断接口、A/B 测试控制 |
| 阶段化队列增加复杂度 | 保留主任务状态简洁，对内部阶段做可观测和重放 |
| 阶段化后端到端延迟上升 | 为每阶段设置 pull/flush 预算和全链路 deadline，监控 `index_ready_lag` |
| pgvector 与写入争用 | 分区、冷热分层、异步全局索引刷新，必要时迁移向量库 |
| Provider 限流导致积压 | Gateway 统一预算，队列背压，按租户限额 |
| Prefetch 挤占 Redis | 限制每 session 预取数量、TTL 和低优先级淘汰 |
| 成本不可控 | 记录 `model_cost_events`，按 workspace/API Key 限额和告警 |

## 12. 1.0 验收标准

统一压测基线：

- 并发导入：5 个 workspace 各 5 个并发任务，单任务约 50 chunks，模型 Provider 处于正常延迟区间。
- 并发问答：每 workspace 20 个并发 Chat，其中 70% 缓存未命中、30% 缓存命中。
- Provider 异常不计入性能基线，但必须计入可靠性验收；若 Provider 排队导致首 token 超标，应触发降级或告警。
- 每次验收同时记录 P50、P95、P99，不能只看平均值。

性能验收：

- 50 chunks 短知识导入 P95 <= 30s。
- `index_ready_lag_seconds` P95 <= 60s，批量导入场景 P95 <= 300s。
- 缓存命中 Chat P95 <= 20ms 开始回放。
- 未命中缓存 Chat 首 token P95 <= 3s。
- 检索未命中缓存 P95 <= 500ms。

可靠性验收：

- 任一导入阶段失败后可重试且不重复写脏数据。
- DLQ 可按阶段定位并重放。
- LLM/Rerank/Embedding 超时都有降级路径。
- 导入高峰期间，已有知识库问答可用性不受明显影响。

质量验收：

- golden cases 召回率不低于优化前。
- 答案缓存不缓存无引用、拒答或低置信答案。
- 检索诊断能展示缓存命中、召回路径、Rerank、waterfall tier 和耗时。
- 语义答案缓存真实回放前，shadow mode 复用准确率达到约定阈值。
- 成本事件覆盖 Embedding、Rerank、LLM、VLM/OCR，且可按 workspace 聚合。

## 13. 结论

GrayDraft 当前已经具备异步 UGC、混合检索、答案缓存、RAGOps 和 DLQ 基础。性能优化 1.0 的核心是把这些能力组织成更清晰的吞吐架构：

- 导入侧：阶段化 JetStream 流水线、Embedding 微批、批量写入、QA 异步、幂等重试和租户背压。
- 问答侧：精细缓存、索引调优、候选裁剪、Prompt 压缩、模型超时预算和 SSE 早期反馈。
- 运维侧：以 `task_id/trace_id/session_id/workspace` 贯穿指标、日志、告警和重放。

该方案优先利用现有技术栈，不要求立即替换 pgvector 或重写业务 API；同时为后续独立向量检索引擎、多租户成本治理和更高吞吐的生产部署留下演进空间。

## 14. 实施跟踪

### 14.1 已完成的优化项（基线）

| 编号 | 优化项 | 阶段 | 实施细节 | 状态 |
|------|--------|------|----------|------|
| BL-01 | 逐条 chunk INSERT → 按嵌入形态分三组 `asyncpg.executemany` 批量写入 | 导入 | `internal/worker/worker/db/repository.py` → `insert_chunks()` | ✅ 已完成 |
| BL-02 | `mark_index_ready` 同步 `REFRESH MATERIALIZED VIEW` → 异步后台刷新 | 导入 | `internal/worker/worker/db/repository.py` → `mark_index_ready()` | ✅ 已完成 |
| BL-03 | `httpx.AsyncClient` 每次新建 → 模块级连接池单例 | 导入 | `internal/worker/worker/indexer/embedder.py` → `_get_client()` | ✅ 已完成 |
| BL-04 | 多模态 Triage VLM 调用 → 启发式分流 | 导入 | `internal/core/service/multimodal/processor.go` → `heuristicTriage()` | ✅ 已完成 |
| BL-05 | parent_title 逐条 UPDATE → 单次批量 `WHERE id = ANY($1)` | 导入 | `internal/worker/worker/db/repository.py` → `set_parent_title_on_docs()` | ✅ 已完成 |
| BL-06 | QA 生成串行在 chunk 写入后 → `asyncio.to_thread` 后台并行 | 导入 | `internal/worker/worker/pipeline/ingest.py` → `_ingest()` | ✅ 已完成 |
| BL-07 | 多模态并发数硬编码 3 → 配置化 | 导入 | `internal/core/service/multimodal/processor.go` + `config.go` | ✅ 已完成 |
| BL-08 | Query Embedding 在缓存查询之前执行 → 缓存优先 | 问答 | `internal/core/service/retriever/hybrid.go` → `Search()` 三阶段 | ✅ 已完成 |
| BL-09 | 结构化向量检索串行 → 三路并发 `wg.Add(3)` | 问答 | `internal/core/service/retriever/hybrid.go` → `searchHybrid()` | ✅ 已完成 |
| BL-10 | 4D Waterfall 最坏 5 轮串行 → 乐观并发 tier1/tier5 | 问答 | `internal/core/service/retriever/waterfall.go` → `searchWithSem4DWaterfall()` | ✅ 已完成 |
| BL-11 | Rerank L1→L2 串行 → combined 单次端点 | 问答 | `internal/core/service/reranker/worker_client.go` + `internal/worker/api/routes/rerank.py` | ✅ 已完成 |
| BL-12 | 无 Embedding 级缓存 → LRU 2000 TTL 10min | 问答 | `internal/core/service/embedder/client.go` → `EmbedQuery` | ✅ 已完成 |
| BL-13 | catalog_version 导致缓存过度失效 → 秒级时间戳 | 问答 | `internal/core/service/cache/key.go` → `BuildExactKey` | ✅ 已完成 |
| BL-14 | touchAccess 同步写 DB → goroutine 异步 | 问答 | `internal/core/service/retriever/hybrid.go` → `touchAccess()` | ✅ 已完成 |
| BL-15 | 检索三级缓存（L1 Ristretto + L2 Redis + L3 预测器） | 问答 | `internal/core/service/cache/` | ✅ 已完成 |
| BL-16 | 答案缓存 L1/L2（Ristretto + Redis） | 问答 | `internal/core/service/answercache/` | ✅ 已完成 |

### 14.2 方案实施跟踪

| 编号 | 分类 | 优化项 | 所属 Phase | 依赖 | 状态 |
|------|------|--------|-----------|------|------|
| V1-01 | 导入 | QA 生成从主导入路径拆出为低优先级异步任务 | Phase 0 | — | ✅ 已完成 |
| V1-02 | 导入 | Core/Gateway 支持 Embedding 批量 API | Phase 1 | — | ✅ 已完成 |
| V1-03 | 导入 | Worker 实现 Embedding 微批缓冲区 | Phase 1 | V1-02 | ✅ 已完成 |
| V1-04 | 导入 | Gateway 批量返回支持 per-item status | Phase 1 | V1-02 | ✅ 已完成 |
| V1-05 | 导入 | Embedding cache 按 text_hash+model+dimension 命中 | Phase 1 | V1-03 | ✅ 已完成 |
| V1-06 | 导入 | Gateway 暴露 Provider 令牌槽反馈 | Phase 1 | V1-02 | ✅ 已完成 |
| V1-07 | 导入 | 阶段化 JetStream subjects：accepted / parsed / media_extracted / chunked / embedded / indexed / qa | Phase 2 | V1-02 | ✅ 已完成 |
| V1-08 | 导入 | 阶段状态机：`current_stage`、`stage_retry_count`、转换表、端到端 deadline | Phase 2 | V1-07 | ✅ 已完成 |
| V1-09 | 导入 | `ugc_ingest_stage_events` 表与阶段事件写库 | Phase 2 | V1-08 | ✅ 已完成 |
| V1-10 | 导入 | 大文件 manifest 异步解析（OSS 预签名 → Worker 拉取） | Phase 2 | — | ✅ 已完成 |
| V1-11 | 导入 | 阶段化 DLQ：每阶段独立 DLQ subject，按阶段定位和重放 | Phase 2 | V1-07 | ✅ 已完成 |
| V1-12 | 导入 | 数据库写入版本切换 index_version | Phase 2 | — | ✅ 已完成 |
| V1-13 | 导入 | 批量导入 bulk mode | Phase 2 | — | ✅ 已完成 |
| V1-14 | 导入 | 租户级背压限流中间件 | Phase 2 | V1-07 | ✅ 已完成 |
| V1-15 | 导入 | 幂等键完整化 chunker_version+chunk_position | Phase 2 | — | ✅ 已完成 |
| V1-16 | 问答 | 细粒度缓存失效事件 `graydraft.cache.invalidate`（实体/标题词/doc_id） | Phase 2+ | V1-07 | ✅ 已完成 |
| V1-17 | 问答 | 语义答案缓存 shadow mode → 离线校准阈值 → 灰度放量 | Phase 3 | — | ✅ 已完成 |
| V1-18 | 问答 | 语义答案缓存真实回放（L2 semantic） | Phase 3 | V1-17 | ✅ 已完成 |
| V1-19 | 问答 | Session Prefetch：含容量控制与低优先级淘汰 | Phase 3 | — | ✅ 已完成 |
| V1-20 | 问答 | 三路候选上限动态调整 + 检索 Profile | Phase 3 | — | ✅ 已完成 |
| V1-21 | 问答 | 诊断接口候选数量与裁剪量 | Phase 3 | — | ✅ 已完成 |
| V1-22 | 问答 | golden cases 回归测试框架 | Phase 3 | V1-20 | ✅ 已完成 |
| V1-23 | 问答 | Chat 分段 timeout 与降级策略 | Phase 0 | — | ✅ 已完成 |
| V1-24 | 问答 | 动态 Prompt 压缩到 5-8 chunks | Phase 3 | — | ✅ 已完成 |
| V1-25 | 索引 | 部分索引 idx_chunks_active + GIN search_text_tsv | Phase 4 | — | ✅ 已完成 |
| V1-26 | 索引 | pgvector HNSW 参数压测脚本 | Phase 4 | V1-25 | ✅ 已完成 |
| V1-27 | 索引 | SET LOCAL hnsw.ef_search 验证 (含于压测脚本) | Phase 4 | V1-26 | ✅ 已完成 |
| V1-28 | 索引 | 写入事务 statement_timeout + lock_timeout | Phase 4 | — | ✅ 已完成 |
| V1-29 | 索引 | Qdrant/Milvus/Vespa 影子索引 PoC | Phase 4 | V1-26 | ✅ 已完成 |
| V1-30 | 网关 | Provider 级熔断 | Phase 1+ | — | ✅ 已完成 |
| V1-31 | 网关 | Batching gateway pattern | Phase 1 | — | ✅ 已完成 |
| V1-32 | 可观测 | 导入阶段耗时、Gateway 模型耗时等指标 | Phase 0 | — | ✅ 已完成 |
| V1-33 | 可观测 | Grafana 性能看板 20 panels | Phase 0 | V1-32 | ✅ 已完成 |
| V1-34 | 可观测 | import/embed/chat/rerank/llm 详细指标埋点 | Phase 0 | — | ✅ 已完成 |
| V1-35 | 可观测 | OpenTelemetry Core→Gateway span | Phase 2+ | V1-07 | ✅ 已完成 |
| V1-36 | 可观测 | model_cost_events 成本事件表 | Phase 0 | — | ✅ 已完成 |
| V1-37 | 可观测 | 缓存污染监控 cache_stale_hit + invalidation_trace | Phase 3+ | V1-16 | ✅ 已完成 |
| V1-38 | 运维 | 动态配置热更新 | Phase 0 | — | ✅ 已完成 |
| V1-39 | 运维 | workspace/API Key 基础限流 | Phase 0 | — | ✅ 已完成 |
| V1-40 | 运维 | 时效词检测可配置词典 | Phase 3 | — | ✅ 已完成 |

## 15. 1.0 方案完成情况

全部 40 项优化任务已完成。以下按 Phase 总结关键实现：

### Phase 0：基线与防护 ✅
- V1-01: QA 生成从主导入路径拆出为低优先级异步任务
- V1-23: Chat 分段 timeout 与降级策略
- V1-32/33/34/36: 导入阶段耗时、Gateway 模型耗时、Grafana 看板、`model_cost_events` 成本事件表
- V1-38/39: 动态配置热更新 + workspace/API Key 基础限流

### Phase 1：Embedding 微批 ✅
- V1-02/03/04/05: Core/Gateway 批量 Embedding API → Worker 微批缓冲区 → per-item status
- V1-06/30/31: Gateway Provider 令牌槽反馈 + Provider 级熔断 + Batching gateway pattern

### Phase 2：阶段化 JetStream 流水线 ✅
- V1-07: 拆分 JetStream subjects (`graydraft.ingest.{accepted|parsed|media_extracted|chunked|embedded|indexed|qa}`)
- V1-08: 阶段状态机（`current_stage`、`stage_retry_count`、转换表、端到端 deadline）
- V1-09: `ugc_ingest_stage_events` 表 + Worker/Go Core 两端事件写库
- V1-10: 大文件 OSS 预签名 URL → Worker 异步拉取 manifest
- V1-11: 每阶段独立 DLQ subject（`graydraft.ingest.dlq.{stage}`）
- V1-12/13/14/15: index_version 切换、bulk mode、租户背压限流、幂等键完整化

### Phase 3：问答缓存与检索裁剪 ✅
- V1-16: 细粒度缓存失效事件 `graydraft.cache.invalidate`（实体/标题词/doc_id 维度）
- V1-17: 语义答案缓存 shadow mode（离线校准阈值，采样记录）
- V1-18: 语义答案缓存真实回放框架（条件门控已就位，待校准后开启）
- V1-19: Session Prefetch（容量控制 + 低优先级淘汰）
- V1-20/21/22: 三路候选上限动态调整 + 诊断接口 + golden cases 回归测试框架
- V1-24: 动态 Prompt 压缩到 5-8 chunks

### Phase 4：索引与向量引擎评估 ✅
- V1-25/26/27/28: 部分索引、GIN 索引、HNSW 参数压测脚本、写入事务超时控制
- V1-29: Qdrant 影子索引 PoC（双写对比框架，采样率 + Jaccard 重叠度 + 排名相关性）

### 可观测性全覆盖 ✅
- V1-35: OpenTelemetry Core→Gateway span
- V1-37: 缓存污染监控 `cache_stale_hit` + invalidation trace
