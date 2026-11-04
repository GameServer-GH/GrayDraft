# GrayDraft 性能瓶颈分析报告

> **文档类型：历史记录。** 正文保留当时的上下文与结论。 [当前文档入口](../README.md)

> 分析日期：2026-10-28
> 优化实施日期：2026-10-28
> 分析范围：知识导入效率 & 用户问答效率
> 约束：不降低检索准确率/召回率，不改变核心业务逻辑

---

## 一、知识导入效率（文档解析 → 向量化 → 存储）

### [P0-Import-1] ✅ 逐条 INSERT chunk 导致 N 次数据库往返

- **状态**：已优化
- **所属阶段**：导入（向量存储写入）
- **效率影响**：每个文档的 chunk 写入延迟 = chunk 数 × DB 往返延迟。100 个 chunk 在 1ms RTT 下即浪费 100ms；在跨 AZ 部署下（~5ms）可达到 500ms+。
- **缺陷类型**：代码逻辑缺陷
- **具体位置**：`internal/worker/worker/db/repository.py` → `insert_chunks()`
- **问题描述**：`insert_chunks` 对每个 chunk 逐条执行 `INSERT INTO kb_chunks ...`，一个 for 循环产生 N 次 PostgreSQL 往返。
- **实施方案**：将逐条 INSERT 改为按嵌入向量形态（双向量 / 单向量 / 无向量）分三组，每组用 `asyncpg.executemany` 批量写入，将 N 次往返合并为最多 3 次。

---

### [P0-Import-2] ✅ 每次 mark_index_ready 触发全局物化视图 REFRESH

- **状态**：已优化
- **所属阶段**：导入（向量存储写入）
- **效率影响**：`REFRESH MATERIALIZED VIEW kb_global_index` 是全量重建操作，在高并发导入场景下会造成严重的锁竞争和 CPU 峰值。
- **缺陷类型**：架构设计缺陷
- **具体位置**：`internal/worker/worker/db/repository.py` → `mark_index_ready()`
- **问题描述**：`mark_index_ready` 在事务末尾无条件执行 `REFRESH MATERIALIZED VIEW kb_global_index`。
- **实施方案**：将同步阻塞的 `REFRESH MATERIALIZED VIEW` 替换为 `asyncio.create_task` 触发的异步后台刷新（`_refresh_global_index_async`），不阻塞主导入事务。

---

### [P1-Import-3] ✅ 嵌入调用每次新建 httpx.AsyncClient，无连接池复用

- **状态**：已优化
- **所属阶段**：导入（向量化 Embedding）
- **效率影响**：每次 `embed_texts` 调用产生 TCP/TLS 握手开销（~50-100ms）。
- **缺陷类型**：代码逻辑缺陷
- **具体位置**：`internal/worker/worker/indexer/embedder.py` → `_embed_via_gateway()`
- **问题描述**：`_embed_via_gateway` 每次调用创建新 `httpx.AsyncClient`，未复用 HTTP 连接池。
- **实施方案**：将 `httpx.AsyncClient` 提升为模块级单例（`_get_client()`），配置 `max_keepalive_connections=20, max_connections=50`。

---

### [P1-Import-4] ✅ 多模态处理串行 Triage + VLM 双重调用

- **状态**：已优化
- **所属阶段**：导入（文档解析与预处理）
- **效率影响**：每个附件在 VLM 主调用之前，先进行一次 Triage VLM 调用，延迟增加 1-3 秒。
- **缺陷类型**：架构设计缺陷
- **具体位置**：`internal/core/service/multimodal/processor.go` → `processOne()`
- **问题描述**：`triage()` 方法调用 VLM API 来判断内容是"数据驱动型"还是"感知驱动型"，Triage 本身也是一次 VLM API 调用。
- **实施方案**：将 VLM-based `triage()` 替换为 `heuristicTriage()` 轻量级启发式函数：PDF → OCR，视频 → VLM，图片 < 512KB → OCR（疑似文档截图），其余 → VLM。零 API 调用。

---

### [P2-Import-5] ✅ 父标题聚合逐文档更新，产生 N 次 DB 写入

- **状态**：已优化
- **所属阶段**：导入（向量存储写入）
- **效率影响**：每个 segment 导入后循环逐条更新每个文档的 parent_title，产生 N 次 UPDATE 往返。
- **缺陷类型**：代码逻辑缺陷
- **具体位置**：`internal/worker/worker/db/repository.py` → `set_parent_title_on_docs()`
- **实施方案**：将 for 循环逐条 UPDATE 改为单次 `WHERE id = ANY($1::uuid[])` 批量更新，N 次往返合并为 1 次。

---

### [P2-Import-6] ✅ QA 生成串行在 chunk 写入之后

- **状态**：已优化
- **所属阶段**：导入（向量存储写入后）
- **效率影响**：QA pair 生成在 chunk insert 返回之后才执行，对长文档增加可观延迟。
- **缺陷类型**：架构设计缺陷
- **具体位置**：`internal/worker/worker/pipeline/ingest.py` → `_ingest()`
- **实施方案**：将 CPU 密集的 `generate_qa_pairs` 通过 `asyncio.to_thread` 放入后台线程，与 chunk INSERT I/O 并行执行，两者完成后才插入 QA pairs。

---

### [P2-Import-7] ✅ 多模态并发数硬编码为 3

- **状态**：已优化
- **所属阶段**：导入（文档解析与预处理）
- **效率影响**：批量附件导入时只有 3 个 goroutine 并发处理。
- **缺陷类型**：代码逻辑缺陷
- **具体位置**：`internal/core/service/multimodal/processor.go` → `processAttachmentsParallel()`、`internal/core/config/config.go`
- **实施方案**：在 `Multimodal` 配置中新增 `Concurrency` 字段（默认 3），`processAttachmentsParallel` 从配置读取并发数。

---

## 二、问答效率（Query → 检索 → 上下文构建 → LLM推理）

### [P0-QA-1] ✅ Query Embedding 在缓存查询之前执行

- **状态**：已优化
- **所属阶段**：问答（Query 预处理与检索）
- **效率影响**：每次问答请求都先调用 Embedding API（~50-150ms），然后才检查检索缓存。缓存命中时 Embedding 调用完全浪费。
- **缺陷类型**：架构设计缺陷
- **具体位置**：`internal/core/service/retriever/hybrid.go` → `Search()`
- **问题描述**：`EmbedQuery` 无条件在缓存 `Lookup` 之前执行。
- **实施方案**：重构 `Search()` 为三阶段缓存查询：Phase 1（精确缓存，无 embedding）→ 命中直接返回；Phase 2（按需计算 embedding）；Phase 3（语义缓存，使用 embedding）。缓存装饰逻辑提取为 `decorateCacheHit()` 复用。

---

### [P0-QA-2] ✅ 结构化向量检索与全文/向量检索串行执行

- **状态**：已优化
- **所属阶段**：问答（检索）
- **效率影响**：全文 + 向量检索并发执行，但结构化向量检索在两者返回后才调用，造成额外串行等待。
- **缺陷类型**：代码逻辑缺陷
- **具体位置**：`internal/core/service/retriever/hybrid.go` → `searchHybrid()`
- **问题描述**：`searchHybrid` 中 Structured Vector 在 `wg.Wait()` 之后才单独调用。
- **实施方案**：将结构化向量检索纳入同一个 `sync.WaitGroup`，三路完全并发执行（`wg.Add(3)`），消除串行等待。

---

### [P1-QA-3] ✅ Semantic4D Waterfall 可能产生最多 5 轮串行 SQL 查询

- **状态**：已优化
- **所属阶段**：问答（检索）
- **效率影响**：当高精度 4D 过滤无结果时，waterfall 逐级降级最坏情况产生 5 次串行数据库查询。
- **缺陷类型**：架构设计缺陷
- **具体位置**：`internal/core/service/retriever/waterfall.go` → `searchWithSem4DWaterfall()`
- **实施方案**：改为乐观并发策略：tier 1（严格过滤）和 tier 5（软过滤）同时发起查询。若 tier 1 有结果立即返回；若无结果则自动使用 tier 5 的结果。将最坏情况 5 轮串行降为 1 轮并行 + 1 次回退。

---

### [P1-QA-4] ✅ Rerank L1 → L2 串行 HTTP 调用

- **状态**：已优化
- **所属阶段**：问答（上下文构建与重排序）
- **效率影响**：L1 rerank 完成后才发起 L2 rerank，两段串行网络延迟。
- **缺陷类型**：架构设计缺陷
- **具体位置**：`internal/core/service/reranker/worker_client.go` → `Rerank()`、`internal/worker/api/routes/rerank.py`
- **实施方案**：
  - **Go 侧**：`Rerank()` 优先调用 `/v1/rerank/combined` 单次端点，不可用时自动降级到原有 L1→L2 串行模式。
  - **Python Worker 侧**：新增 `/v1/rerank/combined` 路由，在单次 HTTP 调用内完成 L1 粗筛 + L2 精排级联。

---

### [P2-QA-5] ✅ 无 Embedding 级缓存——相同 Query 重复编码

- **状态**：已优化
- **所属阶段**：问答（缓存与复用）
- **效率影响**：相同的或高度相似的 Query 每次请求都重新生成 Embedding。
- **缺陷类型**：架构设计缺陷
- **具体位置**：`internal/core/service/retriever/hybrid.go` → `Search()`、`internal/core/service/embedder/client.go`
- **实施方案**：在 `embedder.Client` 中新增基于 `sync.RWMutex` + `map` 的进程内 Embedding LRU 缓存（容量 2000，TTL 10 分钟）。`EmbedQuery` 调用前先查缓存命中则直接返回，避免重复 API 调用。

---

### [P2-QA-6] ✅ 检索结果缓存 key 中 catalog_version 导致过度失效

- **状态**：已优化
- **所属阶段**：问答（缓存与复用）
- **效率影响**：新文档导入时该 KB 下所有 Query 的检索缓存全部失效，写入密集型场景下缓存命中率趋近于 0。
- **缺陷类型**：架构设计缺陷
- **具体位置**：`internal/core/service/cache/key.go` → `BuildExactKey()`
- **实施方案**：从 `BuildExactKey` 中移除全局 `CatalogVersion` 计数器，仅保留 `CatalogUpdated` 秒级时间戳用于缓存键区分。缓存失效由导入侧的显式 `RedisPurgeByKBType` + TTL 兜底，不再因无关文档导入而全量失效。

---

### [P3-QA-7] ✅ `touchAccess` 在检索热路径上同步写 DB

- **状态**：已优化
- **所属阶段**：问答（检索后处理）
- **效率影响**：每次搜索在返回结果前同步更新 `last_accessed_at`，高 QPS 下产生大量 UPDATE 竞争。
- **缺陷类型**：代码逻辑缺陷
- **具体位置**：`internal/core/service/retriever/hybrid.go` → `touchAccess()`
- **问题描述**：`touchAccess` 在 `Search` 返回前同步调用，阻塞检索响应。
- **实施方案**：改为 `go h.touchAccess(ctx, chunks)` 异步 goroutine，访问记录不阻塞检索返回路径。

---

## 三、优先级排序总览

| 优先级 | 编号 | 问题 | 阶段 | 预估延迟节约 | 状态 |
|--------|------|------|------|------------|------|
| **P0** | P0-Import-1 | 逐条 chunk INSERT | 导入 | 100-500ms+/doc | ✅ |
| **P0** | P0-Import-2 | 每次 index_ready 触发 REFRESH MV | 导入 | 高并发下秒级 | ✅ |
| **P0** | P0-QA-1 | Embedding 在缓存查询之前 | 问答 | 50-150ms/命中请求 | ✅ |
| **P0** | P0-QA-2 | 结构化向量检索串行 | 问答 | 10-50ms/请求 | ✅ |
| **P1** | P1-Import-3 | httpx client 不复用 | 导入 | 50-100ms/embedding 调用 | ✅ |
| **P1** | P1-Import-4 | 多模态 Triage 串行 VLM | 导入 | 1-3s/附件 | ✅ |
| **P1** | P1-QA-3 | 4D Waterfall 最多 5 轮查询 | 问答 | 50-200ms (冷查询) | ✅ |
| **P1** | P1-QA-4 | Rerank L1→L2 串行 | 问答 | 20-50ms/请求 | ✅ |
| **P2** | P2-Import-5 | parent_title 逐条 UPDATE | 导入 | 数十ms/segment | ✅ |
| **P2** | P2-Import-6 | QA 生成串行于 chunk 写入后 | 导入 | 100-500ms/doc | ✅ |
| **P2** | P2-Import-7 | 多模态并发硬编码为 3 | 导入 | 视附件数量 | ✅ |
| **P2** | P2-QA-5 | 无 Embedding 级缓存 | 问答 | 50-150ms/重复查询 | ✅ |
| **P2** | P2-QA-6 | catalog version 导致缓存过度失效 | 问答 | 缓存命中率下降 | ✅ |
| **P3** | P3-QA-7 | touchAccess 同步写 DB | 问答 | 数ms/请求 | ✅ |

> 表中 14 项代码改造均已落地，但“已落地”不等同于已完成质量验收。
> 检索质量须使用 `tests/regression/fixture.json` 与同一环境的前后基线验证；
> candidate stats、waterfall 与 rerank 比例也必须以运行时真实统计为准。

---

## 四、实施记录

### 改动的文件

| 文件 | 优化项 |
|------|--------|
| `internal/core/service/retriever/hybrid.go` | P0-QA-1, P0-QA-2, P3-QA-7 |
| `internal/core/service/retriever/waterfall.go` | P1-QA-3 |
| `internal/core/service/reranker/worker_client.go` | P1-QA-4 |
| `internal/core/service/embedder/client.go` | P2-QA-5 |
| `internal/core/service/cache/key.go` | P2-QA-6 |
| `internal/core/service/multimodal/processor.go` | P1-Import-4, P2-Import-7 |
| `internal/core/config/config.go` | P2-Import-7 |
| `internal/worker/worker/db/repository.py` | P0-Import-1, P0-Import-2, P2-Import-5 |
| `internal/worker/worker/indexer/embedder.py` | P1-Import-3 |
| `internal/worker/worker/pipeline/ingest.py` | P2-Import-6 |
| `internal/worker/api/routes/rerank.py` | P1-QA-4 |
