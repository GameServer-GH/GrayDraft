 # 问答可靠性治理

 > **定位**：理解检索分级、门控综合分、答案自检与 Feedback 闭环
 > **代码路径**：`internal/core/service/confidence/`

 ---

 ## 0. 可靠性全景

 ```
 检索返回 chunks
   │
   ├── 1. 检索分级 A/B/C/D（grounding + 实体 + 质量）
   │     └─ 决定 Prompt 注入强度
   │
   ├── 2. 门控综合分（Rerank + Vector + TitleHit + TableBonus）
   │     └─ 按 query_intent 动态阈值判定「有用/无用」
   │
   ├── 3. 答案自检（token overlap + 专名一致性 + 过度自信）
   │     └─ 输出 answer_confidence
   │
   └── 4. Feedback 闭环（点赞/点踩 → 调整 quality_score）
         └─ 影响后续检索排序
 ```

 ---

 ## 1. 检索分级 A/B/C/D

 代码：`confidence/gate.go` → `EvaluateRetrievalGrade`

 ### 判定树

 ```
 chunks 为空 → D（no_chunks）

 高质量 grounding？（Quality ≥ 0.72 + Title/Entity 命中 OR Quality ≥ 0.82 + 关键词）
   ├─ 是 + 实体 Jaccard ≥ 0.3 → A
   └─ 是 + 无实体匹配 → B

 直接证据？（top3：标题命中 / 表格关键词 / quality≥0.65 + 正文关键词）
   → B

 标题命中 OR topQuality ≥ 0.50
   → B

 Final vs 动态阈值
   → C（above 或 below 阈值）
 ```

 ### 分级含义

 | Grade | 含义 | 注入策略 | 缓存写入 |
|-------|------|----------|----------|
| A | 高可信（实体 + 高质量 chunk） | 强注入 5-8 chunks | ✓ |
| B | 中可信（直接证据 / 标题命中） | 正常注入 3-5 chunks | ✓ |
| C | 低可信（稀疏参考） | 稀疏注入 1-2 chunks | 可选 |
| D | 无参考（chunks 为空） | 固定话术/通用 LLM | ✗ |

 ---

 ## 2. 门控综合分

 代码：`confidence/gate.go` → `CalculateFinalConfidenceWithIntent`

 ### 公式

 \[
 Final = 0.6 \times maxRerank + 0.3 \times maxVector + TitleHitBonus(0.15) + StructuredTableBonus(0.10)
 \]

 | 项 | 值 | 来源 |
|----|-----|------|
| maxRerank | top chunk 的 rerank 分数 | L2 Gateway 重排 |
| maxVector | top chunk 的向量分数 | pgvector 余弦 |
| TitleHitBonus | 0.15 | 任一 chunk 标题命中 query 词 |
| StructuredTableBonus | 0.10 | query 词命中 Markdown 表格 |

 ### 动态阈值

 代码：`confidence/threshold.go`

 | query_intent | 基础阈值 | 设计意图 |
|--------------|----------|----------|
| fact | 0.25 | 事实类宁缺毋滥 |
| concept | 0.15 | 概念类允许弱证据 |
| default | 0.20 | — |

 **动态调整**（降低阈值 = 更容易判「有用」）：

 | 条件 | 调整 |
|------|------|
| topQuality ≥ 0.80 | −0.05 |
| topQuality ≥ 0.65 | −0.03 |
| TitleMatched | −0.02 |
| EntityMatched | −0.02 |

 最终阈值夹在 [0.12, 0.32]。

 ---

 ## 3. 答案一致性自检

 代码：`confidence/answer.go` → `EvaluateAnswerConfidence`

 不额外调用 LLM，使用启发式规则：

 | 检测 | 方法 | 权重 |
|------|------|------|
| Token 重叠率 | answer tokens ∩ top chunk tokens / answer tokens | 0.5 |
| 专名/数字一致性 | answer 中专名 ⊆ chunks 中专名；数字偏差 <10% | 0.3 |
| 过度自信惩罚 | answer 中「一定」「肯定」「绝对」比例 | 0.2（扣分） |

 **局限性**：

| 场景 | 表现 |
|------|------|
| LLM 用同义词改写 chunk | token overlap 低 → 误判为不一致 |
| LLM 扩展常识 | 扩展部分无对应 token → 降低 overlap |
| 推荐类答案 | 无法判断是否基于引用 |

 ---

 ## 4. Feedback 闭环

 ```bash
 curl -X POST http://localhost:8443/api/v1/chat/feedback \
   -d '{"doc_id":"...", "helpful": true}'
 ```

 **影响链路**：

 ```
 点踩 → quality_score -= 0.1
   → 检索排序中该文档降权
   → 后续检索更不可能排到 top
   → 连续多次点踩 → 可能触发文档淘汰
 ```

 ---

 ## 5. 学习练习

 1. 给定一个检索结果（rerank=0.85, vector=0.72, titleMatched, 无 tableHit），计算 Final 分数。
 2. fact 类 query、topQuality=0.85、TitleMatched：动态阈值是多少？Final 需要多高才通过？
 3. 一个答案 rating=0.82 但 entity_number_match=0.2，answer_confidence 会是多少？这合理吗？
 4. 设计一个场景：分级从 B 降到 C，但用户看起来答案没问题。说明为什么会这样。

 ---

 ## 6. 相关文档

 - [检索分级与门控综合分](../03-algorithms/04-retrieval-grading-and-gating.md)（专题深挖）
 - [自研算法深度学习 §3 §4](../03-algorithms/02-custom-algorithms.md)
 - [agent-import-chat-reliability.md](../../architecture/agent-import-chat-reliability.md)
 - [agent-v2.md](../../api/agent-v2.md)
 - [chat.md](../../api/chat.md)（已下线，历史参考）
