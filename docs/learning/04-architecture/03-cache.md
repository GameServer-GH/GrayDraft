 # 缓存体系

 > **定位**：理解 GrayDraft 的多级缓存架构、缓存键设计与失效策略
 > **代码路径**：`internal/core/service/cache/`、`internal/core/service/answercache/`

 ---

 ## 0. 缓存全景

 ```
 请求到达
   │
   ├── 检索缓存（retrieval cache）
   │   ├─ L1 Ristretto（进程内，<1ms）
   │   ├─ L2 Redis exact key（<5ms）
   │   └─ L2 Redis semantic（余弦 ≥ 0.92）
   │
   └── 答案缓存（answer cache）
       ├─ L1 Ristretto（进程内）
       └─ L2 Redis 版本感知
 ```

 **两种缓存的区别**：

 | | 检索缓存 | 答案缓存 |
|------|----------|----------|
| 缓存内容 | chunks[] + retrieval_trace | 完整 SSE 事件序列（meta + chunk + done） |
| 失效触发 | catalog_version bump | catalog_version bump |
| 命中效果 | 跳过检索 + Rerank | 跳过检索 + LLM 生成 |
| Grade D | 写入 | 不写入 |

 ---

 ## 1. L1 Ristretto（进程内）

 Ristretto 是 DGraph 出品的 Go 高性能缓存库，基于 TinyLFU 准入策略。

 **配置**：

 ```yaml
 cache:
   l1_max_items: 100000       # 最大条目数
   l1_max_mb: 256             # 最大内存（MB）
   l1_ttl_seconds: 300        # TTL 5 分钟
 ```

 **为什么用 Ristretto 而不是 sync.Map？**

 | | Ristretto | sync.Map |
|------|-----------|----------|
| 淘汰策略 | TinyLFU（基于访问频率） | 无自动淘汰 |
| 内存控制 | 有（max cost） | 无 |
| 并发性能 | 高（分片锁） | 中（读多写少优化） |
| GC 友好 | 是 | 是 |

 ---

 ## 2. L2 Redis（跨进程）

 ### 2.1 精确 key 缓存

 ```
 key = hash(query + kb_type + tenant_id + kb_id + catalog_version)
 value = JSON {chunks[], query_semantic_4d, retrieval_trace, ...}
 TTL = cache.redis_ttl_seconds（默认 1800s = 30min）
 ```

 ### 2.2 语义相似缓存

 不是精确 key 匹配，而是基于 Embedding 余弦相似度的近似匹配：

 ```
 查询流程:
   1. query → query_vec（Embedding）
   2. 查 Redis 中缓存的 query embeddings（按 kb_type 分区）
   3. 计算余弦相似度
   4. cos ≥ 0.92 → 返回缓存结果
   5. cos < 0.92 → miss
 ```

 | 阈值 | 行为 |
|------|------|
| 0.95 | 极其保守（≈精确匹配） |
| 0.92（默认） | 推荐：容忍少量表达差异 |
| 0.88 | 较激进：命中率高但可能误命中 |

 ### 2.3 版本感知失效

 ```
 catalog_version = 15
   → 所有 key 含 "v15"
   → 知识库更新 → catalog_version = 16
   → 旧 key "v15" 不再命中
   → 新查询生成 "v16" key → 走检索
   → 旧 key 由 Redis TTL 自然淘汰
 ```

 不需要全量 purge——利用 key 中嵌版本做到「写入即失效」。

 ---

 ## 3. 缓存键设计原则

 ```go
 func buildCacheKey(query string, partition string, version int, fingerprint string, grade string) string {
     raw := fmt.Sprintf("%s|%s|%d|%s|%s", query, partition, version, fingerprint, grade)
     return fmt.Sprintf("graydraft:cache:%x", sha256.Sum256([]byte(raw)))
 }
 ```

 | 组成部分 | 作用 | 不包含的后果 |
|----------|------|-------------|
| `query` | 基础去重 | 不同 query 共享缓存 |
| `partition`（kb_type） | 分区隔离 | 跨分区返回错误结果 |
| `catalog_version` | 版本感知 | 知识库更新后回放过期结果 |
| `semantic_4d_fingerprint` | 语义维度区分 | 同 query 不同场景返回同一结果 |
| `retrieval_grade` | 分级感知 | 低分级结果覆盖高分级 |

 ---

 ## 4. 缓存失效策略

 | 触发条件 | 动作 | 粒度 |
|----------|------|------|
| 单文档入库 | purge 该 partition 的检索 + 答案缓存 | partition 级 |
| catalog_version bump（Blueprint 变更） | 旧 key 自动失效 | 全量（通过 key 设计） |
| Redis TTL 到期 | 自然淘汰 | key 级 |
| 手动 `workspace-data/purge` | 全量清空该 partition 缓存 | partition 级 |
| 时效性 query | 自动跳过答案缓存 | query 级 |

 ---

 ## 5. 学习练习

 1. 对比 L1 Ristretto 和 L2 Redis：为什么答案缓存只命中时就跳过检索和 LLM，而检索缓存只跳过检索？
 2. 如果移除 catalog_version 进 key，什么场景下用户会看到过期的错误答案？
 3. 语义相似缓存阈值从 0.92 降到 0.85，会带来什么好处和风险？
 4. 设计一个实验验证「catalog_version bump 后旧 key 在多久后完全消失」。

 ---

 ## 6. 相关文档

 - [检索链路](../02-pipeline/02-retrieval.md)
 - [SSE RAG 对话](../02-pipeline/03-agent-v2-sse.md)
 - [标准与改良算法深度学习](../03-algorithms/01-standard-and-improved.md) §3.1 §3.2
 - [cache.md](../../api/cache.md)
 - [answer-cache.md](../../api/answer-cache.md)
