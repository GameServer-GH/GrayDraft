# 三级缓存

Core 检索在访问数据库前依次查询：

| 层级 | 实现 | 说明 |
|------|------|------|
| **L1** | Ristretto 进程内 | 精确 Key 命中，微秒级 |
| **L2** | Redis | 精确 Key + 向量语义相似（cosine ≥ threshold） |
| **L3** | 内存预测器 | 基于同 `kb_type` 上一查询的预取命中 |

## 版本感知 Key

```
MD5(normalize(query) | kb_type | top_k | catalog_version | latest_doc_updated_at | use_global_index | query_intent | semantic_4d_fingerprint)
```

`semantic_4d_fingerprint` 来自 `QuerySemantic4D.CacheFingerprint()`，仅在本次检索实际启用 4D SQL 预过滤且标签非空时写入；避免「同句问法、不同 4D 过滤」误命中旧缓存。

**L2 语义缓存**：`IndexKey` 同样包含 fingerprint，向量相似命中仅在同一 4D 分桶内比较。

`kb_catalog_version` 在 Worker 完成入库后递增，旧缓存自动失效。

## 配置（`config.dev.yaml`）

```yaml
cache:
  enabled: true
  redis_enabled: true
  l3_enabled: true
  similarity_threshold: 0.92
  ttl_seconds: 3600
```

## 跳过缓存

```json
{ "query": "...", "kb_type": "tutorial", "skip_cache": true }
```

## 响应字段

- `cache_hit`: `l1` | `l2_exact` | `l2_semantic` | `l3`
- `strategy`: 缓存命中时为 `cache_l1` 等；未命中时为 `hybrid_rrf+rerank_l1_l2` 等

对话完整答案缓存见 [`answer-cache.md`](answer-cache.md)。
