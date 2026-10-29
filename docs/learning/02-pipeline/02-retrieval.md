 # 检索链路

 > **定位**：理解一次 `retrieval/search` 请求从入口到返回 topK chunks 的完整路径
 > **前置**：[Core 服务架构](../04-architecture/01-core-service.md)、[算法总览](../03-algorithms/README.md)
 > **代码路径**：`internal/core/service/retriever/hybrid.go`（~719 行）

 ---

 ## 0. 检索全景

 ```
 Client
   │ POST /api/v1/retrieval/search {query, kb_type, top_k}
   ▼
 Handler  → retrievalH.Search
   │ scope 解析 (tenant_id + kb_id → storage_partition)
   ▼
 Hybrid.Search
   │ 1. 检索缓存 L1/L2（exact key 匹配）
   │    ├─ hit → 直接返回（跳过后续所有步骤）
   │    └─ miss → 继续
   │ 2. 意图分类（fact / concept / proper_noun / hybrid）
   │ 3. Query4D 提取（domain / scene / style / entities）
   │ 4. 4D 瀑布退避（Tier 1→5）
   │ 5. 三路并行召回：
   │    ├─ 全文 ILIKE + search_terms
   │    ├─ pgvector 余弦相似度
   │    └─ structured 向量（可选）
   │ 6. RRF 加权融合
   │ 7. 两阶段 Rerank（L1 Worker → L2 Gateway）
   │ 8. 写回缓存
   ▼
 Response {chunks[], query_semantic_4d, retrieval_trace, strategy}
 ```

 ---

 ## 1. 入口与缓存检查

 ```go
 func (h *Hybrid) Search(ctx context.Context, req model.SearchRequest) (*model.SearchResponse, error) {
     // Phase 1: 精确缓存（L1 Ristretto → L2 Redis exact key）
     cacheKey := buildExactCacheKey(req.Query, req.KBType, req.TenantID, req.KBID)
     if cached, hit := h.cache.GetExact(ctx, cacheKey); hit {
         return cached, nil  // cache hit → 直接返回
     }

     // Phase 2: 需要向量化
     queryVec, err := h.embedder.Embed(ctx, req.Query)

     // Phase 3: 语义缓存（L2 Redis 余弦相似 ≥ 0.92）
     if cached, hit := h.cache.GetSemantic(ctx, req.KBType, queryVec); hit {
         return cached, nil
     }

     // 缓存全部 miss → 执行检索
     return h.doSearch(ctx, req, queryVec)
 }
 ```

 **三级缓存检查**：

 | 阶段 | 匹配方式 | 命中条件 | 跳过 |
|------|----------|----------|------|
| Phase 1 | exact key（query + partition） | key 完全匹配 | 跳过 Embedding + 检索 + Rerank |
| Phase 2 | —（标记） | — | 需要计算 Embedding |
| Phase 3 | 语义相似（余弦 ≥ 0.92） | query_vec 与缓存中已有 query_vec 相近 | 跳过检索 + Rerank |

 ---

 ## 2. 意图分类

 代码路径：`internal/core/service/classifier/classifier.go`

 ```
 Query → ClassifyAndRoute(query)
   ├─ 规则路径（<5ms）：正则 + 关键词打分
   │   ├─ 疑问词检测（"什么""怎么""如何" → concept）
   │   ├─ 专名模式（《书名》、[A-Z][a-z]+ → proper_noun）
   │   ├─ 数字/单位模式（→ fact）
   │   └─ 取最高分意图
   └─ LLM 兜底（规则 confidence 低时触发）
   → RoutePlan {QueryIntent, FulltextWeight, VectorWeight}
 ```

 输出决定检索权重：

 | 意图 | fulltext_weight | vector_weight |
|------|----------------|---------------|
| fact | 0.6 | 0.4 |
| concept | 0.3 | 0.7 |
| proper_noun | 0.5 | 0.5 |
| hybrid | 0.45 | 0.55 |

 ---

 ## 3. Query4D + 4D 瀑布

 详见 [自研算法深度学习](../03-algorithms/02-custom-algorithms.md) §1 §2。

 简版流程：

 ```
 query → ExtractQuery4D
   → domain / scene / style / entities
   → sql_4d_where (用于过滤)
   → semantic_4d_fingerprint (用于缓存 key)

 4D Waterfall:
   Tier 1 (strict):   domain + entities + scene + style
   Tier 2 (_wf_core):  domain + entities
   Tier 3 (_wf_ent):   domain + shortest entity
   Tier 4 (_wf_dom):   domain only
   Tier 5 (_wf_soft):  无 SQL WHERE + BoostSemantic4D 软加分
 ```

 ---

 ## 4. 三路并行召回

 ```go
 // 并行执行三路召回
 var wg sync.WaitGroup
 var fulltextResults, vectorResults, structuredResults []model.ChunkResult

 wg.Add(3)
 go func() { fulltextResults = h.fulltextSearch(ctx, ...); wg.Done() }()
 go func() { vectorResults = h.vectorSearch(ctx, queryVec, ...); wg.Done() }()
 go func() { structuredResults = h.structuredSearch(ctx, queryVec, ...); wg.Done() }()
 wg.Wait()
 ```

 ### 4.1 全文召回

 SQL 示例：

 ```sql
 SELECT id, doc_id, search_text, ts_rank(...) as score
 FROM kb_chunks
 WHERE kb_type = $1
   AND index_ready = true
   AND (
     search_text ILIKE '%' || $2 || '%'           -- 子串匹配
     OR search_text ILIKE '%' || $3 || '%'        -- 分词匹配（AND 连接）
   )
 ORDER BY score DESC
 LIMIT 50;
 ```

 `search_text` 内容 = `title（重复 2 次）+ entities 拼接 + 前 2000 字正文`

 ### 4.2 向量召回

 ```sql
 SELECT id, doc_id, embedding <=> $1 AS distance
 FROM kb_chunks
 WHERE kb_type = $2
   AND index_ready = true
   -- 可选 4D WHERE 条件
 ORDER BY embedding <=> $1
 LIMIT 50;
 ```

 `<=>` 是 pgvector 的余弦距离操作符。距离越小越相似。

 ### 4.3 structured 向量召回（可选）

 仅当项目配置启用了 structured embedding 且有结构化内容的 chunk 时参与。

 ---

 ## 5. RRF 加权融合

 代码路径：`retriever/hybrid.go` → `rrfFuse`

 ### 5.1 标准 RRF 公式

 \[
 RRF(d) = \sum_{r \in R} \frac{1}{k + rank_r(d)}
 \]

 参数 `k`（`rrf_constant_k`）默认 60。

 ### 5.2 GrayDraft 加权版本

 \[
 RRF_{weighted}(d) = w_f \cdot \frac{1}{k + rank_f(d)} + w_v \cdot \frac{1}{k + rank_v(d)} + w_s \cdot \frac{1}{k + rank_s(d)}
 \]

 | 路 | 权重符号 | 默认值 |
|----|---------|--------|
| 全文（fulltext） | w_f | 0.45 |
| 向量（vector） | w_v | 0.40 |
| structured 向量 | w_s | 0.15 |

 权重由意图分类动态调整。

 ### 5.3 多 KB 分区策略

 当 `kb_types[]` 包含多个分区时：

 1. 每个分区独立执行三路召回 + RRF
 2. 各分区 top K 汇总 → 按分区优先级全局排序
 3. 主知识库权重 > 辅助知识库

 ---

 ## 6. 两阶段 Rerank

 ### 6.1 L1（Worker 轻量重排）

 ```
 RRF Top 50 candidates
   → POST Worker /rerank-l1
     body: {query, passages: [{title: "标题 | 实体 | 正文", ...}]}
   ← Top 20 候选
 ```

 ### 6.2 L2（Gateway 精排）

 ```
 L1 Top 20
   → POST Gateway /rerank
     body: {query, passages: [...]}
   ← 每个 passage 的 relevance_score (0-1) + 排序
 ```

 ### 6.3 Combined 模式

 如果 L1 和 L2 使用同一模型提供方 → 合并为单次调用，减少一次网络往返。

 ### 6.4 词法降级

 Gateway Rerank 不可用时（超时/限流/错误），自动降级到本地词法排序（F1 + bigram + 标题 bonus），保证链路不断。

 ---

 ## 7. 检索诊断 API

 ```bash
 curl -X POST http://localhost:8443/api/v1/retrieval/diagnose \
   -H 'Content-Type: application/json' \
   -d '{"query":"InternVL 怎么用","kb_type":"tutorial","top_k":5}'
 ```

 返回比 search 更丰富的调试信息：

 ```json
 {
   "query_semantic_4d": {"domain":"edu_tutorial","scene":"参数对照","entities":["InternVL"]},
   "retrieval_trace": {
     "waterfall_tier": 1,
     "waterfall_suffix": "",
     "entity_alignments": [...]
   },
   "strategy": "hybrid_rrf+rerank_l1_l2",
   "cache_hit": "",
   "gate_score": 0.72,
   "gate_threshold": 0.25,
   "retrieval_grade": "A",
   "chunks": [...]
 }
 ```

 ---

 ## 8. 检索结果写回缓存

 ```
 检索完成
   → L1 Ristretto: exact key → TTL 5min
   → L2 Redis: exact key → TTL 30min
   → L2 Redis: 语义缓存（query_vec + result）→ TTL 1h
 ```

 Key 包含 `catalog_version`——知识库更新后旧 key 自动失效。

 ---

 ## 9. 学习练习

 1. 追踪 `Search()`：画出从 handler 到 `rrfFuse` 的完整调用链。
 2. 手算 RRF：全文路 [A:1, B:2, C:3]、向量路 [B:1, C:2, A:10]，k=60，求 A/B/C 的融合分。
 3. 对比诊断：分别用 diagnose API 查 "InternVL 怎么用" 和 "RAG 是什么"，对比两者的 query_intent、waterfall_tier、gate_threshold。
 4. 为什么三路召回用 `sync.WaitGroup` 而不是 channel？这样设计的好处和风险是什么？
 5. 如果 L1 rerank 池设为 20（太小），会出现什么问题？

 ---

 ## 10. 相关文档

 - [算法总览](../03-algorithms/README.md) §2
 - [自研算法深度学习](../03-algorithms/02-custom-algorithms.md) §1 §2 §3 §4
 - [标准与改良算法深度学习](../03-algorithms/01-standard-and-improved.md) §2
 - [retrieval.md](../../api/retrieval.md)
 - [retrieval-and-chat.md](../../features/retrieval-and-chat.md)
 - [semantic-driven-retrieval.md](../../architecture/semantic-driven-retrieval.md)
