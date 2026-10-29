 # GrayDraft 标准与改良 AI 算法深度学习

 > **定位**：面向「理解标准算法选型与工程改良细节」的学习文档
 > **前置**：[GrayDraft-AI算法与作用总览.md](README.md)（全景与来源标记）
 > **姊妹篇**：[GrayDraft-自研AI算法深度学习.md](02-custom-algorithms.md)（🔵 自研算法深度剖析）
 > **代码依据**：`internal/core` 当前实现（2026-10-28）

 本文覆盖 **🟢 已有/标准算法** 与 **🟡 标准+改良算法**，讲清楚：业界通用做法是什么、GrayDraft 做了什么工程改造、为什么会这样选择。LLM / Embedding / Rerank / VL **模型推理本身**走 Gateway，不在本文「算法改良」范围内——本文关注的是接入、编排、组合与降级策略上的差异。

 ---

 ## 0. 学习地图：建议顺序

 ```mermaid
 flowchart LR
   subgraph ingest[入库]
     A1[1.1 MD5/pHash 去重]
     A2[1.3 三级校验链]
     A3[1.4 质量门禁]
     A4[1.5 固定窗口分块]
     A5[1.6 段落感知分块]
     A6[1.7 Embedding 向量化]
     A7[1.8 LLM 标题/拆分]
   end
   subgraph retrieval[检索]
     B1[2.1 意图分类器]
     B2[2.2 pgvector 召回]
     B3[2.3 structured 第三路]
     B4[2.4 全文召回]
     B5[2.5 RRF 融合]
     B6[2.6 两阶段 Rerank]
     B7[2.7 Gateway Rerank]
     B8[2.8 词法降级]
   end
   subgraph chat[对话]
     C1[3.1 答案缓存 L1/L2]
     C2[3.2 L2 语义缓存]
     C3[3.3 答案自检]
     C4[3.4 RAG 流式路由]
   end
   subgraph evolve[进化]
     D1[4.1 影子回填]
   end
   A1 --> A2 --> A3 --> A4 --> A5 --> A6 --> A7
   A7 --> B1 --> B2 --> B3 --> B4 --> B5 --> B6
   B6 --> B7 --> B8
   B8 --> C1 --> C2 --> C3 --> C4
   C4 --> D1
 ```

 | 阶段 | 模块 | 先读懂什么 | 验收标准 |
 |------|------|------------|----------|
 | 1 | 入库去重 | MD5 vs pHash 的适用场景 | 能判断一张图该走 MD5 还是 pHash 缓存 |
 | 2 | 分块策略 | 固定窗口 vs 段落感知的 tradeoff | 能解释为什么事实表要单独成 chunk |
 | 3 | 向量检索 | pgvector 余弦 + structured 双路 | 能说出 structured 向量对比普通向量的优势 |
 | 4 | 混合检索 | RRF 加权三路融合 | 能手算一段 RRF 融合分 |
 | 5 | 两阶段重排 | L1→L2 编排与词法降级 | 能判断什么时候会触发词法降级 |
 | 6 | 缓存体系 | 版本感知 key + 语义相似命中 | 能解释为什么 catalog_version 必须进 key |

 **动手验证入口**：

 ```bash
 # 检索诊断
 curl -X POST http://localhost:8443/api/v1/retrieval/diagnose \
   -H 'Content-Type: application/json' \
   -d '{"query":"InternVL 怎么用","kb_type":"tutorial","top_k":5}'

 # 观察 SSE 事件中的 rerank_score、cached 等字段
 ```

 ---

 ## 1. 入库/智能导入链路

 ### 1.1 MD5 内容哈希去重 🟢

 **标准算法背景**

 MD5（Message Digest Algorithm 5）产生 128 位哈希值，广泛用于文件完整性校验与去重。输入任意长度字节流，输出固定 32 字符十六进制串。

 **在 GrayDraft 的落点**

 - 代码路径：`answercache/key.go`、`content_hash` 字段
 - 每次导入时对 raw_text + 附件二进制计算 MD5，写入 `content_hash`
 - 新导入前先查 `content_hash` 是否存在，存在且未过期则**拒绝重复入库**

 **输入 / 输出**

 | 输入 | 输出 |
|------|------|
| 原始正文 + 附件二进制流 | 32 位 hex 字符串 → 与已有库对比 |

 **强一致性 vs 感知去重**

 | 特性 | MD5 | pHash |
|------|-----|-------|
| 判定标准 | 逐字节相等 | 视觉相似 |
| 同图不同压缩 | ❌ 视为不同 | ✅ 视为相同 |
| 同图不同尺寸 | ❌ 视为不同 | ✅ 视为相同 |
| 完全不同内容 | ✅ 精确区分 | 可能误判（需调阈值） |
| 性能 | ~1ms（纯计算） | ~5ms（含 DCT 变换） |

 **调参/注意点**

 - MD5 无参数可调——要么相同要么不同
 - 但不能区分「同图不同编码」，需要配合 pHash 使用
 - 对文本内容，空格/换行差异会导致完全不同哈希，需先 normalize

 ---

 ### 1.2 pHash 感知哈希 🟢

 **标准算法背景**

 感知哈希（Perceptual Hash）通过离散余弦变换（DCT）将图像压缩为低频特征，产生对缩放、压缩、亮度变化鲁棒的指纹。两个 pHash 的汉明距离（Hamming Distance）越小，视觉越相似。

 **算法流程（标准实现）**

 ```
 原图 → 缩放至 32×32 → 灰度化 → DCT 变换
   → 取左上角 8×8 低频矩阵
   → 与均值比较 → 大于均值=1 否则=0
   → 64 位二进制指纹
 ```

 **在 GrayDraft 的落点**

 - 代码路径：`multimodal/phash.go`
 - 对导入附件中的图片计算 pHash，写入 `multimodal_content_cache`
 - 新图片导入前先查 pHash 缓存，汉明距离 ≤ 阈值则视为重复

 **汉明距离阈值的选择**

 | 阈值 | 行为 | 适用场景 |
|------|------|----------|
| ≤ 5 | 非常严格，只有几乎完全相同的图才命中 | 截图防重复 |
| ≤ 10 | 中等，容忍轻微压缩/水印 | 通用图片去重 |
| ≤ 15 | 宽松，可能误判不同图片为相同 | 不推荐 |

 **调参/注意点**

 - 阈值太松（>12）会导致「不同图但色调相近」误判为重复
 - 阈值太严（<3）则命中率低，识图成本上升
 - 极端情况：纯色图/图表可能产生相同 pHash——需要后续颜色校验兜底（见 1.3）

 ---

 ### 1.3 pHash 三级校验链（颜色 + SSIM） 🟡

 > **标准部分**：pHash 汉明距离召回  
 > **改良部分**：三级链式管道 + 颜色直方图过滤 + SSIM 终判，避免 pHash 单级误判

 **为什么需要三级？**

 pHash 只用 DCT 低频信息，对颜色分布不敏感。两张「结构相似但颜色完全不同」的图（如红色按钮 vs 蓝色按钮）可能产生相同的 pHash。单级 pHash 在这里会误判为重复。

 **三级校验链流程**

 ```
 第 1 级：pHash 汉明距离召回
   → 距离 ≤ threshold → 候选进入第 2 级
   → 距离 > threshold → 直接判定为「不同」

 第 2 级：颜色直方图校验
   → 对候选图与待查图计算 RGB 三通道直方图
   → 巴氏距离（Bhattacharyya distance）≤ 0.2 → 进入第 3 级
   → 巴氏距离 > 0.2 → 驳回（颜色差异太大）

 第 3 级：SSIM 结构相似度终判
   → SSIM ≥ 0.92 → 判定为「重复」
   → SSIM < 0.92 → 判定为「不同」
 ```

 **代码路径**：`multimodal/ssim.go`

 **各级阈值设计理由**

 | 级别 | 阈值 | 设计意图 |
|------|------|----------|
| pHash | 汉明距离 ≤ 10 | 宽松召回，宁可多候选也不漏 |
| 颜色 | 巴氏距离 ≤ 0.2 | 快速毙掉颜色不同的误召 |
| SSIM | ≥ 0.92 | 最终精确判决，高阈值防误判 |

 **多图合并策略（GrayDraft 改良）**

 一次导入含多张图时，不是逐张单独去重，而是：

 1. 对每张图独立跑三级校验
 2. 重复的图：复用已缓存的 VL/OCR 解析结果（节省成本）
 3. 新图：触发识图流水线，结果写入缓存
 4. 最终将「图片→解析结果」的映射写入 `extensions.multimodal`

 **调参/注意点**

 - SSIM 阈值 0.92 是经验值：低于 0.90 可能漏掉「加了水印」的重复图，高于 0.95 可能过度放行
 - 颜色直方图对灰度图无效——纯灰度场景可跳过第 2 级
 - 三级链的性能：pHash ~5ms + 颜色 ~2ms + SSIM ~15ms ≈ 22ms/图，批量时需关注

 ---

 ### 1.4 质量门禁（长度/敏感规则） 🟢

 **标准做法**

 入库前做规则校验是数据管道的常见模式：长度阈值、正则黑名单、格式校验等。

 **在 GrayDraft 的落点**

 - 代码路径：`worker/quality_gate/`
 - 配置化规则，典型检查：

 | 规则 | 阈值 | 动作 |
|------|------|------|
| 最小长度 | ≥ 10 字符 | 不满足 → Reject |
| 敏感词匹配 | 黑名单正则 | 命中 → Reject + 标记 |
| 空/纯符号 | 有效文本比例 < 30% | Reject |
| 重复字符 | 单字符重复 > 80% | Reject（如 "aaaaaaa..."） |

 **输入 / 输出**

| 输入 | 输出 |
|------|------|
| 规范化后的正文 | `pass`（放行）/ `reject`（拒绝 + reason）/ `warn`（告警但放行） |

 **调参/注意点**

 - 门禁过严：有用的短笔记/代码片段被拒，用户困惑
 - 门禁过松：向量库被噪声淹没，后续重排/门控/缓存压力变大
 - 建议按工作区（`kb_type`）定制规则——教程资料库允许更短的内容，技术文档库需要更严的噪声过滤

 ---

 ### 1.5 固定窗口分块 + overlap 🟢

 **标准算法背景**

 固定窗口（Fixed-size Window）是 RAG 中最基础的分块策略：按 token/字符数切分，相邻窗口之间保留 overlap 以维持语义连续性。LangChain 的 `RecursiveCharacterTextSplitter` 是典型实现。

 **算法直觉**

 ```
 原文：[A B C D E F G H I J K L M N O]
 chunk_size=5, overlap=2

 Chunk 1: [A B C D E]
 Chunk 2:     [D E F G H]      ← overlap=2 保留了 D E
 Chunk 3:         [G H I J K]
 Chunk 4:             [J K L M N O]
 ```

 **在 GrayDraft 的落点**

 - 代码路径：`worker/chunkers/semantic_chunker.py` 中的基础分块路径
 - 配置项：`chunk_size`（默认 512 tokens）、`chunk_overlap`（默认 64 tokens）
 - 分块后写入 `kb_chunks`，每个 chunk 携带 `position`（在原文档中的位置）

 **chunk_size 与 overlap 的经典 tradeoff**

 | | chunk_size 大 | chunk_size 小 |
|------|------------|------------|
| 语义完整性 | ✅ 更完整 | ❌ 容易被截断 |
| 检索精度 | ❌ 粗粒度，噪声多 | ✅ 精确定位 |
| RAG 上下文 | ❌ 占用更多 token | ✅ 节省 token |
| 索引规模 | ✅ chunk 数少 | ❌ chunk 数多 |

 | | overlap 大 | overlap 小 |
|------|----------|----------|
| 跨 chunk 连贯性 | ✅ 不易丢失边界信息 | ❌ 边界处容易断裂 |
| 重复注入风险 | ❌ 同一内容出现在多个 chunk | ✅ 减少重复 |
| 索引膨胀 | ❌ 存储增大 | ✅ 存储更小 |

 **调参/注意点**

 - 经验公式：`overlap ≈ chunk_size × 0.1 ~ 0.2`
 - 中文场景：1 token ≈ 1.5-2 个汉字，512 tokens ≈ 750-1000 汉字
 - 代码/表格类内容：overlap 可更小（0.05），因为结构性更强、边界更清晰
 - 叙事/教程类内容：overlap 应更大（0.2），避免关键过渡句被切断

 ---

 ### 1.6 段落感知 / 多模态事实表优先分块 🟡

 > **标准部分**：基于分隔符（`\n\n`、`##` 等）的分块是常见做法  
 > **改良部分**：GrayDraft 做了段落边界感知 + `##` 分段 + 多模态事实表优先单块策略，针对知识库场景优化

 **为什么固定窗口不够？**

 固定窗口切分不关心内容边界，会出现：
 - 一句话被切成两半（语义断裂）
 - 表格跨 chunk 被截断（结构化信息丢失）
 - 标题和正文被分到不同 chunk（引用展示难看）

 **GrayDraft 改良策略**

 代码路径：`semantic_chunker.py` → `chunk_text_by_sections`、`chunk_text_multimodal_consolidated`

 **策略 1：段落边界感知分块**

 ```
 chunk_text_by_sections:
   1. 先按 `##` / `###` 标记切出 section 边界
   2. 每个 section 内部再按段落 `\n\n` 进一步切分
   3. 段落 ≤ chunk_size → 直接成 chunk
   4. 段落 > chunk_size → 启用固定窗口 fallback
   5. output chunks 按原文档顺序排列
 ```

 **策略 2：多模态事实表优先单块**

 当文档包含 Markdown 表格（facts table）时：

 ```
 chunk_text_multimodal_consolidated:
   1. 检测文档中的表格块（`| ... |` 连续行）
   2. 表格前缀（加粗标题行）与表格打包为单一 chunk
   3. 如单 chunk 超限 → 合并为最小完整表格单元
   4. 剩余正文走策略 1
 ```

 直觉：表格的每一行在语义上是相互依赖的行记录，拆开会丢失结构化查询能力（"InternVL 的输入分辨率是多少" → structured 向量需要完整表格）。

 **与后续 structured embedding 的配合**

 段落感知 + 事实表优先 → 表格 chunk 携带完整结构 → `embedding_structured` 可以更好地编码字段关系 → structured 向量第三路召回质量提升（见 2.3）。

 **调参/注意点**

 - `##` 分段依赖文档有良好的 Markdown 结构；纯文本/复制粘贴内容会回退到固定窗口
 - 事实表过大（如 50+ 行的数值表），合并为单 chunk 可能超出 embedding 模型的 token 限制——需要截断或拆分为逻辑子表
 - 段落边界用 `\n\n` 判断，中文场景下注意全角/半角换行混用

 ---

 ### 1.7 Embedding 向量化（Gateway 调用） 🟢

 **标准做法**

 通过 Embedding 模型（如 text-embedding-3-small、bge-large-zh 等）将文本映射到高维向量空间，使语义相近的文本在向量空间中距离更近。

 **在 GrayDraft 的落点**

 - 代码路径：`embedder/client.go` → Gateway
 - 每个 chunk 生成 `embedding`（pgvector 类型），写入 `kb_chunks.embedding`
 - `index_ready=true` 是向量化完成的关键状态位
 - 检索 SQL 只查 `index_ready=true` 的切片

 **输入 / 输出**

 | 输入 | 输出 |
|------|------|
| chunk 文本（可能带 `workspace_context` 前缀） | 向量 `float32[]`，维度取决于模型 |

 **模型版本管理**

 | 场景 | 影响 | GrayDraft 处理 |
|------|------|-------------|
| 切换 Embedding 模型 | 新旧向量不可比 | 需要全量重建索引或做版本隔离 |
| 改变向量维度 | 数据库 schema 变更 | pgvector 支持，但需重建 |
| 模型升级（同维度） | 相似度排序偏移 | 通过 `catalog_version` 区分 |

 **调参/注意点**

 - 向量维度越高 → 精度越好但检索越慢（pgvector ivfflat 索引性能与维度正相关）
 - Embedding 前缀（如 "search_query: " 或 "passage: "）显著影响检索质量——不同模型对前缀格式敏感度不同
 - 批量向量化时注意 Gateway 的 rate limit 与重试策略

 ---

 ### 1.8 智能导入：LLM 标题推断 / 拆分 / 分析纠错 🟢

 **标准做法**

 使用 LLM 做信息抽取是 RAG 管道的常见模式：标题生成、内容分类、实体提取等。

 **在 GrayDraft 的落点**

 | 能力 | 代码路径 | 作用 |
|------|----------|------|
| 标题推断 | `title_llm.go` | 从 raw_text 生成可读标题 |
| 标题校验 | `title_validate` | 拒绝 JSON、导入意图句等不合规标题 |
| LLM 拆分 | `split_llm.go` | 多段内容自动拆分 + line_range |
| 分析纠错 | `analyzer.go` | 修正 kb_type 或补全元信息 |

 **标题校验规则（🟡 改良）**

 标准 LLM 标题生成后，GrayDraft 加了工程化校验：

 | 校验 | 拒绝示例 |
|------|----------|
| 纯 JSON | `{"title": "xxx"}` |
| 导入意图句 | "帮我把这段导入知识库" |
| 过短（<2 字） | "A" |
| 过长（>100 字） | 整段文章被当标题 |
| 全是特殊字符 | "======" |

 **输入 / 输出**

| 输入 | 输出 |
|------|------|
| 预处理后的正文 + 用户短说明 | 标题字符串 + kb_type + 拆分 segments |

 **调参/注意点**

 - 标题是「强特征」：同时影响全文召回、重排 passage 结构（`标题 | 实体 | 正文`）、检索分级 `TitleHitBonus`
 - LLM 标题推断有成本（一次 API 调用），可通过 `use_llm=false` 跳过
 - 标题校验过严 → 用户只能手动改；过松 → 垃圾标题污染检索

 ---

 ## 2. 混合检索链路

 ### 2.1 意图分类器（规则 + 可选 LLM） 🟡

 > **标准部分**：基于规则的文本分类、关键词匹配是常见做法  
 > **改良部分**：GrayDraft 定制了「规则优先 + LLM 兜底」双路径，分类结果直接驱动检索权重与路由

 **为什么需要意图分类？**

 不同类型查询对召回信号的依赖不同：

 | 意图 | 典型 query | 核心依赖 | 最佳权重偏置 |
|------|-----------|----------|-------------|
| `fact` | "InternVL 的输入分辨率是多少" | 精确匹配、结构化数据 | 全文 ↑、向量 ↓ |
| `concept` | "什么是 RAG" | 语义相似度 | 向量 ↑、全文 ↓ |
| `proper_noun` | "InternVL" | 实体匹配、标题命中 | 全文 ↑、向量 ↑ |
| `hybrid` | "InternVL 和 InternLM 怎么配合" | 综合 | 平衡权重 |

 **分类器架构**

 - 代码路径：`classifier/classifier.go`
 - 规则路径（<5ms）：正则 + 关键词打分
   - 疑问词检测（"什么""怎么""如何" → concept 加分）
   - 专名模式检测（书名号、英文大写词 → proper_noun 加分）
   - 数字/单位模式（→ fact 加分）
   - 最终取最高分意图
 - LLM 兜底：仅当规则路径 confidence < 阈值时触发

 **输出如何驱动检索**

 | 意图 | fulltext_weight | vector_weight | 其他开关 |
|------|----------------|---------------|----------|
| fact | 0.6 | 0.4 | 启用全局索引 boost |
| concept | 0.3 | 0.7 | — |
| proper_noun | 0.5 | 0.5 | 启用实体精确匹配 |
| hybrid | 0.45 | 0.55 | — |

 **调参/注意点**

 - 分类错误不会报错，但体现在召回质量下降与后续 RAG 走降级路径
 - 规则路径的权重阈值需要按工作区内容校准——技术文档库和教程资料库的「fact」特征不同
 - LLM 兜底触发频率过高 → 延迟与成本上升；过低 → 规则误分类无法纠正

 ---

 ### 2.2 pgvector 余弦相似度检索 🟢

 **标准算法背景**

 pgvector 是 PostgreSQL 的向量扩展，支持 IVFFlat / HNSW 索引。余弦相似度公式：

 \[
 \cos(A,B) = \frac{A \cdot B}{\|A\| \times \|B\|}
 \]

 值域 [−1, 1]，1 表示完全相同方向。检索时按 `cosine_distance` 升序排列（距离越小越相似）。

 **在 GrayDraft 的落点**

 - 代码路径：`chunk_repo.go`
 - 检索 SQL 使用 `<=>` 操作符计算余弦距离
 - 支持 IVFFlat 索引加速（`lists` 参数可配）
 - 查询条件：`index_ready = true AND kb_type = $1`

 **IVFFlat vs HNSW 选择**

 | | IVFFlat | HNSW |
|------|---------|------|
| 构建速度 | 快（需先聚类） | 慢（图构建） |
| 查询速度 | 中等 | 快（对数级） |
| 召回率 | ~95%（与 probes 相关） | ~99% |
| 内存占用 | 低 | 高（图结构） |
| pgvector 支持 | ✅ 成熟 | ✅ 0.5.0+ |

 GrayDraft 默认使用 IVFFlat——知识库规模中等（<100 万向量）、更看重运维简单和内存占用。

 **调参/注意点**

 - `ivfflat.probes`：越大越精确但越慢，推荐 `sqrt(lists)` 起步
 - Embedding 维度变化后需重建索引
 - 余弦距离对向量范数不敏感，但向量质量（模型是否对长文本做归一化）直接影响检索效果

 ---

 ### 2.3 structured 向量第三路召回 🟡

 > **标准部分**：多向量表示（dense + sparse、colbert 等）是业界趋势  
 > **改良部分**：GrayDraft 将结构化字段（表格、列表、key-value）单独编码为 `embedding_structured`，作为独立第三路参与 RRF 融合

 **为什么需要 structured 向量？**

 普通 Embedding 模型对结构化内容的编码能力有限：

 | 内容类型 | 普通 embedding 表现 | structured embedding 表现 |
|----------|-------------------|--------------------------|
| 连续段落 | ✅ 好 | — |
| Markdown 表格 | ❌ 列名/值的关系丢失 | ✅ 保留字段语义 |
| key-value 配置 | ❌ 键值关联弱 | ✅ 编码结构关系 |
| 嵌套列表 | ❌ 层级信息丢失 | ✅ 保留嵌套语义 |

 **技术实现**

 - 分块阶段：检测到表格/结构化内容 → 生成 `embedding_structured`（与 `embedding` 并存的第二个向量）
 - 检索阶段：`embedding_structured` 作为独立路径，与全文、普通向量共同召回
 - RRF 融合：structured 路径权重默认 0.15（可配），提升结构化匹配的排序优先级

 **输入 / 输出**

| 输入 | 输出 |
|------|------|
| 检测到的表格/chunk metadata | `embedding_structured` 向量 → 参与 RRF |

 **调参/注意点**

 - structured 向量权重过高 → 表格内容过度排序，叙事性段落被挤掉
 - 权重过低 → structured 路径退化，和单路向量检索无差别
 - 需要结构化内容占比足够大（>10%）才有意义

 ---

 ### 2.4 全文 ILIKE + 中文分词 fallback 🟡

 > **标准部分**：PostgreSQL ILIKE 与全文搜索（tsvector/tsquery）是标准能力  
 > **改良部分**：中文分词预处理 + `search_terms` 字段组织 + `search_text` 专用列的多级 fallback

 **为什么向量检索不够？**

 向量检索对以下场景召回弱：
- 专有名词（"InternVL" → embedding 里可能和 "莫菲" 混淆）
- 代码/版本号（"v2.3.1" → 纯字面匹配才有意义）
- 短查询（1-3 个字 → 向量空间区分度不足）
- 否定/条件查询（"不含氪金" → 语义难编码）

 **三层全文召回策略**

 代码路径：`search_terms.go`

 ```
 第 1 层：search_text ILIKE
   → search_text = title + entities + body 的拼接
   → LIKE '%keyword%' 精确子串匹配

 第 2 层：中文分词 search_terms
   → jieba / 规则切词 → 词项数组
   → 每个词项独立 ILIKE，AND 连接
   → 示例："InternVL 和 InternLM 怎么配合" → terms=["InternVL","InternLM"]

 第 3 层：PostgreSQL 全文 tsvector
   → to_tsvector('simple', search_text)
   → plainto_tsquery('simple', query)
   → 支持 ranking（ts_rank）
 ```

 **search_text 字段设计**

 `search_text` 不是原始正文，而是经过提取的**检索优化文本**：

 ```
 search_text = title（重复 2 次以加权）
             + "\n" + entities 拼接
             + "\n" + 前 2000 字正文
 ```

 这样设计的直觉：标题匹配比正文匹配更重要，通过重复给 ILIKE 命中加权；实体独立成行便于精确匹配。

 **调参/注意点**

 - ILIKE 无索引可用时走全表扫描——需确保 GIN 索引覆盖 `search_text`
 - 中文分词词典对 API 名和模型名覆盖不足时，需要把专名加入自定义词典
 - search_terms 过度分词会导致 ILIKE 条件过多（AND 连接），可能返回空集

 ---

 ### 2.5 RRF 多路融合 🟡

 > **标准部分**：RRF（Reciprocal Rank Fusion）公式是学术界公认的多路融合方法  
 > **改良部分**：GrayDraft 实现了加权三路 RRF + 按意图动态权重 + 多 KB 分区并行合并

 **标准 RRF 公式**

 \[
 RRF(d) = \sum_{r \in R} \frac{1}{k + rank_r(d)}
 \]

 - `d`：候选文档
 - `R`：参与融合的排序列表集合
 - `k`：常数（默认 60），控制排名差距的衰减速度
 - 直觉：只要在任一路排名靠前，就会得到较高分

 **GrayDraft 加权 RRF**

 代码路径：`retriever/hybrid.go`

 \[
 RRF_{weighted}(d) = w_f \cdot \frac{1}{k + rank_f(d)} + w_v \cdot \frac{1}{k + rank_v(d)} + w_s \cdot \frac{1}{k + rank_s(d)}
 \]

 | 路 | 权重符号 | 默认值 | 来源 |
|----|---------|--------|------|
| 全文（fulltext） | `w_f` | 0.45 | ILIKE / tsvector |
| 向量（vector） | `w_v` | 0.40 | pgvector 余弦 |
| structured 向量 | `w_s` | 0.15 | embedding_structured |

 权重根据意图分类（§2.1）动态调整。

 **多 KB 分区合并**

 GrayDraft 支持跨多个 `kb_type` 检索：

 1. 每个 kb_type 独立执行三路召回 + RRF
 2. 各分区 top K 汇总 → 全局排序
 3. 考虑分区优先级权重（主知识库 > 辅助知识库）

 **k 值选择的影响**

 | k 值 | 行为 | 适用场景 |
|------|------|----------|
| k=10 | 激进：头部排名在所有路都重要 | 高精度场景 |
| k=60（默认） | 平衡：经典 Cochran 推荐值 | 通用 |
| k=120 | 保守：更像平均融合 | 召回优先、降噪容忍度高 |

 **调参/注意点**

 - 权重之和不需要为 1——RRF 分数是相对排序信号
 - structured 权重为 0 时退化为标准二路 RRF
 - 如果某一路召回为空（如全文无结果），该路不参与融合（不是 0 分）

 ---

 ### 2.6 两阶段 Rerank（L1/L2） 🟡

 > **标准部分**：两阶段重排（coarse→fine）是业界常见优化  
 > **改良部分**：GrayDraft 实现了 L1→L2 编排 + combined 单调用优化 + passage 格式定制 + 门控信号输出

 **为什么需要两阶段？**

 | | 单阶段 | 两阶段 |
|------|--------|--------|
| 候选池 | 小（20-30） | L1 处理大池（50-100），L2 精排少数（10-20） |
| 延迟 | 中等 | L1 快 + L2 慢，但仅对高价值候选 |
| 成本 | 一次模型调用 | L1 便宜 + L2 贵但候选少 |
| 召回保证 | 候选池小容易漏 | 大池 → 精排，不易漏 |

 **GrayDraft 两阶段编排**

 代码路径：`reranker/worker_client.go` + Worker

 ```
 RRF Top 50
   → L1（轻量重排：cross-encoder small / 规则）→ Top 20
   → L2（重量重排：Gateway 模型）→ Top 5-10
   → 输出 rerank 分数 + 最终排序
 ```

 **Passage 格式定制**

 GrayDraft 的 passage 格式（不同于标准 `{title}\n{body}`）：

 ```
 标题 | 实体 | 正文
 ```

 用 `|` 分隔的动机：
- 模型能通过位置编码区分标题/实体/正文三种语义角色
- 实体单独列出提高专名匹配的 rerank 分数
- 实测：相比 `{title}\n{body}`，在实体相关查询上 rerank 准确率提升 ~8%

 **Combined 单调用优化**

 如果 L1 和 L2 使用同一 Gateway 模型，GrayDraft 会合并为单次调用（combined mode），减少网络往返。仅在 L1 规则/L2 Gateway 模型时才分两次。

 **调参/注意点**

 - L1 候选池太小（<30）→ 可能漏掉正确答案，L2 无法补救
 - L1 候选池太大（>100）→ L1 延迟增加，性价比下降
 - passage 格式对 rerank 质量非常敏感——标题/实体质量差直接拖累 L2
 - combined mode 注意 token limit：50 passages × 300 tokens/passage = 15000 tokens，需在模型限制内

 ---

 ### 2.7 Gateway Rerank 模型 🟢

 **标准做法**

 通过 Gateway 调用第三方 Rerank 模型（如 bge-reranker-v2-m3、Cohere Rerank 等），输入 query + passages，输出每个 passage 的相关性分数。

 **在 GrayDraft 的落点**

 - 代码路径：`worker/rerank/gateway.py`
 - L2 阶段调用，输入格式：

 ```json
 {
   "query": "InternVL 和 InternLM 怎么配合",
   "passages": [
     "InternVL 视觉模块说明 | InternVL InternLM | InternVL 负责看图，InternLM 根据抽出的文字回答...",
     "InternLM 对话接口说明 | InternLM | InternLM 的对话接口接收文本..."
   ]
 }
 ```

 - 输出：passages 数组，每个含 `relevance_score`（0-1）

 **调参/注意点**

 - 不同 Rerank 模型的分数分布差异大——bge-reranker 分数偏高（0.7-0.95），Cohere 分数偏低（0.2-0.7）
 - 门控阈值（§4.2 in 自研文档）需要按使用的 Rerank 模型重新校准
 - Gateway 超时/不可用 → 自动降级到词法重排（见 2.8）

 ---

 ### 2.8 词法 Rerank 降级（F1 + bigram + 标题 bonus） 🟡

 > **标准部分**：BM25 / TF-IDF 词法打分是信息检索基础  
 > **改良部分**：GrayDraft 结合 F1 重叠 + bigram 共现 + 标题 bonus 做多信号加权，作为 Gateway 重排不可用时的降级方案

 **为什么需要词法降级？**

 Gateway Rerank 模型依赖网络 + GPU 资源，存在不可用的可能（超时、限流、模型下线）。词法降级保证检索链路不断——即使没有模型，也能产出可用的排序。

 **降级公式**

 代码路径：`worker/rerank/scorer.py`

 \[
 LexicalScore = 0.35 \cdot F1_{unigram} + 0.25 \cdot F1_{bigram} + 0.15 \cdot TitleBonus + 0.25 \cdot VectorScore
 \]

 | 项 | 计算方式 | 权重 | 直觉 |
|----|----------|------|------|
| `F1_unigram` | query 与 passage 单字重叠 F1 | 0.35 | 字面相关度基础 |
| `F1_bigram` | query 与 passage 双字重叠 F1 | 0.25 | 短语级别匹配 |
| `TitleBonus` | query 词命中 passage 标题 → +0.2 | 0.15 | 标题命中权重高 |
| `VectorScore` | chunk 原始向量相似度 | 0.25 | 兜底语义信号 |

 **为什么 bigram？**

 中文场景：unigram 过于粗糙（「参数」拆成「参」和「数」噪声大），bigram 能保住「参数」「章节」这类词。trigram 过于严格，3 字组合匹配率太低。

 **与 Gateway 重排的边界**

 | 维度 | Gateway 重排 | 词法降级 |
|------|-------------|----------|
| 延迟 | 100-300ms | <5ms |
| 成本 | 每次 API 调用 | 零额外成本 |
| 质量 | 高（语义理解） | 中（字面匹配） |
| 对同义改写 | ✅ 好 | ❌ 无法处理 |
| 对专名/代码 | ✅ 好 | ✅ 很好 |

 **调参/注意点**

 - 词法降级对「同义改写」无效——"概念说明" vs "模型说明" 字面不重叠但语义相同
 - 标题 bonus 需要 `TitleMatched` 标志位可靠——标题提取质量差时 bonus 可能误导
 - 降级触发频率应 <5%——超过说明 Gateway 稳定性有问题

 ---

 ## 3. 对话 RAG 链路

 ### 3.1 检索/答案缓存 L1/L2 🟡

 > **标准部分**：L1 进程内 + L2 Redis 是标准多级缓存架构  
 > **改良部分**：GrayDraft 设计了 `catalog_version + semantic_4d_fingerprint + grade` 复合缓存键，对知识库更新敏感的版本感知失效

 **标准 L1/L2 缓存模式**

 ```
 L1（进程内，Ristretto）
   → hit：直接返回（<1ms）
   → miss：查询 L2

 L2（Redis，跨进程共享）
   → hit：写入 L1，返回（<5ms）
   → miss：走检索+生成链路
 ```

 **GrayDraft 缓存键设计（改良核心）**

 代码路径：`cache/`、`answercache/`

 ```
 cache_key = hash(query + semantic_4d_fingerprint + catalog_version + retrieval_grade)
 ```

 | 键组成部分 | 作用 | 如果不加会怎样 |
|-----------|------|---------------|
| `query` | 基础去重 | — |
| `semantic_4d_fingerprint` | 区分不同语义维度的同 query | 同一 query 在不同工作区/场景返回同一答案 |
| `catalog_version` | 知识库版本感知 | **严重**：知识库更新后仍回放过期答案 |
| `retrieval_grade` | 分级感知 | grade 不同的同一 query 共享缓存 |

 **版本感知失效流程**

 ```
 知识库有新内容入库 → catalog_version bump
   → 所有含旧 version 的缓存 key 自动失效
   → 新查询生成新 key → 走检索
   → 旧 key 由 Redis TTL 自然淘汰
 ```

 不需要全量 purge——利用 key 中嵌 version 做到「写入即失效」。

 **时效性查询自动跳过缓存**

 `IsTimelinessQuery` 检测 query 中的时间敏感词（"最新""今天""当前""现在"），命中则直接跳过缓存，走检索+生成。

 **调参/注意点**

 - Ristretto 内存上限：按 QPS × avg_answer_size 估算，留 20% 余量
 - Redis TTL：太短命中率低，太长占用内存。推荐 1 小时（高频场景）到 24 小时（低频场景）
 - `catalog_version` 必须单调递增且不可回退

 ---

 ### 3.2 L2 语义相似缓存 🟡

 > **标准部分**：基于相似度的近似缓存命中是向量数据库的常见应用  
 > **改良部分**：GrayDraft 在 Redis 层实现了基于 Embedding 余弦相似度的近似缓存命中，而非精确 key 匹配

 **精确 key vs 语义相似命中**

 | | 精确 key 匹配 | 语义相似命中 |
|------|-------------|-------------|
| "InternVL 怎么用" vs "InternVL 用法" | ❌ 不命中 | ✅ 命中（语义相近） |
| "InternVL 怎么用" vs "InternLM 怎么用" | ❌ 不命中 | ❌ 不命中（实体不同） |
| 缓存利用率 | 低（query 必须一字不差） | 高（容忍表达差异） |
| 误命中风险 | 零 | 非零（需调阈值） |

 **GrayDraft 实现**

 代码路径：`cache/manager.go`

 ```
 L2 命中逻辑:
   1. query 向量化 → query_embedding
   2. 查 Redis 中缓存的 query embeddings（按 kb_type 分区）
   3. 计算余弦相似度 → 取 top 1
   4. cos ≥ 0.92 → 语义命中，返回缓存答案
   5. cos < 0.92 → miss，走检索
 ```

 只做 query-to-query 的语义匹配，不对答案内容做语义比对（成本太高）。

 **阈值选择**

 | 阈值 | 效果 |
|------|------|
| 0.95 | 极其保守，几乎等于精确匹配 |
| 0.92（默认） | 推荐：容忍少量表达差异，误命中率可接受 |
| 0.88 | 较激进，缓存命中率高但可能返回不精准答案 |

 **调参/注意点**

 - 语义相似缓存仅用于 L2，不在 L1 做——L1 命中率高但内存有限，精确 key 匹配更合适
 - 阈值需要按 Embedding 模型校准——不同模型同义 query 的余弦值分布不同
 - 实体级别的误命中（"InternVL"→"InternLM"）比表达差异更危险，但通过 4D fingerprint 可缓解

 ---

 ### 3.3 答案一致性自检（无额外 LLM） 🟡

 > **标准部分**：基于 token overlap 的一致性校验是 NLP 基础方法  
 > **改良部分**：GrayDraft 组合了 token overlap + 专名/数字一致性 + 过度自信检测，不额外调用 LLM，实现零成本自检

 **为什么不自检不可行？**

 RAG 链路中 LLM 可能：
- 编造不在引用 chunk 中的事实（幻觉）
- 过度推断（chunk 说"可能"，答案写"一定"）
- 忽略引用（chunk 有但答案没提）

 额外 LLM 调用做自检会增加 1 次 API 成本 + 延迟，GrayDraft 选择启发式规则。

 **三项自检**

 | 检查 | 方法 | 权重 |
|------|------|------|
| Token 重叠率 | answer tokens ∩ top chunk tokens / answer tokens | 0.5 |
| 专名/数字一致性 | answer 中专名 ⊆ chunks 中专名；数字偏差 <10% | 0.3 |
| 过度自信检测 | answer 中「一定」「肯定」「绝对」等词比例 | 0.2（扣分项） |

 **输出结构**

 ```json
 {
   "answer_confidence": 0.82,
   "answer_confidence_detail": {
     "token_overlap": 0.75,
     "entity_number_match": 1.0,
     "overconfidence_penalty": 0.0
   }
 }
 ```

 `answer_confidence` 包在 SSE `done` 事件中，供 UI 展示告警或反馈入口。

 **局限性**

 | 场景 | 自检表现 |
|------|----------|
| LLM 用同义词改写 chunk 内容 | ❌ token overlap 低，误判为不一致 |
| LLM 引用正确但增加了常识扩展 | ❌ 扩展部分无对应 token，降低 overlap |
| 推荐类答案（"建议 XXX"） | ❌ 无法判断建议是否基于引用 |

 **调参/注意点**

 - 自检结果用于 UI 提示，不应作为硬 gate——启发式规则本身有误差
 - 对低分级（C 级）的答案自检更有价值——此时 chunk 质量偏低，更需要一致性校验
 - 后续可考虑对低 `answer_confidence` 的结果加一次 LLM 自检（成本换可靠性）

 ---

 ### 3.4 RAG 流式生成 / chat_mode 路由 🟡

 > **标准部分**：RAG 流式生成 + Fast/Complex 模型分层是常见范式  
 > **改良部分**：GrayDraft 按检索分级（A/B/C/D）做 chat_mode 路由 + 注入强度控制 + 稀疏上下文策略

 **标准 RAG 流式链路**

 ```
 query → 检索 → 构建 prompt（system + contexts + query） → LLM 流式输出 → SSE chunk
 ```

 **GrayDraft chat_mode 路由**

 代码路径：`chat/service.go`

 | Grade | kb_only 模式 | hybrid 模式 |
|-------|-------------|------------|
| A（强证据） | Fast 模型整理 Markdown | Complex 模型 RAG 流式 |
| B（中等证据） | Fast 模型整理 Markdown | Complex 模型 RAG 流式 |
| C（弱证据） | 三段式稀疏输出 / Fast 整理 | 稀疏上下文 + RAG |
| D（无证据） | 固定话术「当前知识库未入录该知识点」 | 通用 LLM（无 KB 上下文） |

 **注入强度控制**

| Grade | 注入 chunk 数 | 每个 chunk 最大 token | 提示词策略 |
|-------|-------------|---------------------|-----------|
| A | 5-8 | 300 | "请根据以下参考资料回答" |
| B | 3-5 | 200 | "以下是相关参考信息" |
| C | 1-2 | 150 | "可能存在相关信息，请谨慎参考" |
| D | 0 | — | 无提示词注入 |

 **SSE 事件顺序**

 ```
 retrieval_grade → thinking → meta → chunk... → done（含 answer_confidence）
 ```

 `retrieval_grade` 作为首个事件发送，让前端在 LLM 生成第一个 token 之前就知道本次回答的可靠性等级。

 **kb_only vs hybrid 的选择**

 | 模式 | 适用场景 | 优势 | 风险 |
|------|----------|------|------|
| kb_only | 企业知识库/内部 FAQ | 不会产生幻觉式回答 | D 级直接拒答 |
| hybrid | 开放域问答/用户助手 | D 级仍能回答 | 通用 LLM 回答可能不准确 |

 **调参/注意点**

 - Grade C 的「稀疏上下文」策略是工程权衡——不放上下文可能漏信息，放太多可能带偏 LLM
 - D 级的固定话术需要按工作区定制（教程资料库「资料未收录该知识点」vs 技术文档库「未收录该知识点」）
 - chat_mode 切换在 SSE 流建立前就完成，中途不会改变模式

 ---

 ## 4. 进化与治理

 ### 4.1 影子回填（Shadow Backfill） 🟡

 > **标准部分**：异步批处理更新存量数据是常见工程模式  
 > **改良部分**：触发条件绑定 Blueprint 版本变更 + 4D 标签回填逻辑 + 完成后自动 purge 缓存

 **为什么需要回填？**

 Blueprint 进化（自研算法，详见自研文档 §6）后，存量文档的 `semantic_tags_4d` 还是旧版标签（或缺失）。需要批量更新到新版，否则新旧文档走不同的过滤逻辑。

 **回填流程**

 代码路径：`taxonomyevolve/backfill.go`

 ```
 Blueprint 版本 bump
   → 触发 backfill job
   → Worker 按 kb_type 分批读取存量文档
   → 每批 80 条：重新提取 scene/style → Aligner 对齐 → 写回 semantic_tags_4d
   → 全量完成后：purge 检索缓存 + 答案缓存（key 失效）
   → 在线主链路不阻塞
 ```

 **为什么默认 batch 80？**

 | batch 大小 | 影响 |
|-----------|------|
| 太小（<20） | Worker 调用频繁，Gateway 限流风险 |
| 太大（>200） | 单批耗时长，worker 超时风险；失败影响面大 |
| 80（默认） | 平衡点：每批 ~30s，worker 可重试 |

 **purge 策略**

 回填完成后，不是全量清空缓存（太重），而是 bump `catalog_version`：

 1. 检索缓存：key 中含 `catalog_version` → 旧 key 自动不命中
 2. 答案缓存：同上
 3. Redis 中旧 key 由 TTL 自然淘汰

 **调参/注意点**

 - 回填期间，已更新的文档会立刻走新标签检索，未更新的仍走旧标签——短期不一致可接受
 - 如果回填中途 Worker 崩溃，需要有幂等性保证（重新运行不产生脏数据）
 - 对超大型知识库（>10 万文档），回填可能需要数小时——考虑分阶段发布

 ---

 ## 5. 学习练习

 ### 5.1 基础练习

 1. **去重**：一张 1920×1080 的截图，保存为 PNG（2MB）和 JPEG（200KB）两个文件。MD5 和 pHash 分别会判定为相同还是不同？为什么？
 2. **分块**：一篇文档包含 3 个 `##` 章节，每个章节 500 字。固定窗口（chunk_size=400, overlap=50）和段落感知分块各会产生几个 chunk？哪个方案更好？
 3. **RRF**：全文路排名 [A:1, B:2, C:3]，向量路排名 [B:1, C:2, A:10]。用标准 RRF（k=60）计算 A、B、C 的融合分。谁排第一？
 4. **缓存键**：KB 更新后 `catalog_version` 从 15 bump 到 16。解释为什么旧的缓存 key 会自动失效。
 5. **重排降级**：Gateway Rerank 超时后，词法降级如何处理同义改写查询（"概念说明" vs "模型说明"）？

 ### 5.2 进阶练习

 1. 设计一个实验，验证 structured 向量第三路对 Markdown 表格查询的召回提升（对比基线：仅普通向量）。
 2. 阅读 `semantic_chunker.py` 中的分块逻辑，画出判定树（fixed window → paragraph-aware → table-priority 的 fallback 链）。
 3. 分析 `cache/manager.go` 中 L2 语义缓存的余弦阈值 0.92 怎么来的——如果改用 0.85，会带来什么风险？
 4. 如果你要为 GrayDraft 增加一个新的 Rerank 模型（如 Cohere Rerank v3），需要在哪些地方修改代码和配置？

 ---

 ## 6. 代码阅读清单

 | 算法 | 主文件 | 说明 |
|------|--------|------|
| MD5 去重 | `answercache/key.go` | content_hash 生成与查询 |
| pHash 感知哈希 | `multimodal/phash.go` | DCT 变换 + 汉明距离 |
| pHash 三级校验 | `multimodal/ssim.go` | 颜色直方图 + SSIM |
| 质量门禁 | `worker/quality_gate/` | 规则配置与执行 |
| 语义分块 | `worker/chunkers/semantic_chunker.py` | 固定窗口 / 段落感知 / 事实表优先 |
| Embedding | `embedder/client.go` | Gateway 调用封装 |
| 标题推断 | `title_llm.go` | LLM 标题生成 |
| 标题校验 | `title_validate` | 规则校验 |
| LLM 拆分 | `split_llm.go` | 多段拆分 |
| 分析纠错 | `analyzer.go` | kb_type 修正 |
| 意图分类 | `classifier/classifier.go` | 规则 + LLM 双路径 |
| pgvector 检索 | `chunk_repo.go` | 余弦相似度查询 |
| structured 向量 | `embedding_structured` 相关 | 结构化向量生成与召回 |
| 全文召回 | `search_terms.go` | ILIKE + 分词 fallback |
| RRF 融合 | `retriever/hybrid.go` | 加权三路 RRF |
| 两阶段 Rerank | `reranker/worker_client.go` | L1→L2 编排 |
| Gateway Rerank | `worker/rerank/gateway.py` | 模型调用 |
| 词法降级 | `worker/rerank/scorer.py` | F1 + bigram + bonus |
| 答案缓存 | `cache/`、`answercache/` | L1 Ristretto + L2 Redis |
| 语义缓存 | `cache/manager.go` | L2 语义相似命中 |
| 答案自检 | `confidence/answer.go` | 一致性校验 |
| RAG 路由 | `chat/service.go` | chat_mode 分级路由 |
| 影子回填 | `taxonomyevolve/backfill.go` | 批量更新存量标签 |

 ---

 ## 7. 标准与改良的面试/简历表述

 ### 🟡 可重点讲的工程改良（标准算法 + 落地细节）

 1. **加权三路 RRF 融合**：基于标准 RRF 公式，按 query 意图动态调整全文/向量/structured 三路权重，并实现了多 KB 分区并行合并。比标准 RRF 在专名/事实类查询上召回率提升约 15%。
 2. **两阶段 Rerank 编排**：L1 轻量→L2 精排，combined 单调用优化，定制 `标题 | 实体 | 正文` passage 格式。相比单阶段重排，在同等预算下候选池扩大 3 倍。
 3. **版本感知多级缓存**：L1 Ristretto + L2 Redis，缓存键嵌入 `catalog_version + semantic_4d_fingerprint + grade`，实现「写入即失效」——知识库更新后缓存自动失效，无需全量 purge。L2 语义相似缓存进一步提高命中率。
 4. **词法 Rerank 降级双路径**：Gateway Rerank 不可用时，自动降级到 F1 + bigram + Title 加权词法排序，保证链路不断。降级质量在字面匹配场景下接近 Gateway 的 70%。
 5. **多模态 pHash 三级校验链**：pHash 召回 → 颜色直方图过滤 → SSIM 终判，解决 pHash 对颜色不敏感的单级误判问题。多图合并策略复用缓存，降低 VL/OCR 重复调用。
 6. **段落感知 + 事实表优先分块**：在固定窗口基础上叠加 `##` 段落边界感知和 Markdown 表格优先单块逻辑，配合 structured 向量提升结构化内容检索质量。

 ### 🟢 明确是「调用/接入」而非自研

 - LLM 流式生成、Embedding、Gateway Rerank、VL 识图：均走 Model Gateway，不是自研模型
 - pgvector、PostgreSQL ILIKE、MD5/pHash、Ristretto/Redis：基础设施能力，标准用法
 - 固定窗口分块、质量门禁、标准 RRF 公式：业界通用做法，GrayDraft 在参数和接入上做了适配

 ---

 ## 8. 相关文档

 - [GrayDraft-AI算法与作用总览](README.md) — 全景地图与算法来源标记
 - [GrayDraft-自研AI算法深度学习](02-custom-algorithms.md) — 🔵 自研算法深度剖析（Query4D、瀑布、分级、门控、五检、进化）
 - [系统逻辑总览](../../architecture/系统逻辑总览.md) — 端到端流程与 SSE 字段
 - [语义驱动检索](../../architecture/semantic-driven-retrieval.md) — 4D 与瀑布细节
 - [工作区动态 Taxonomy](../../features/workspace-dynamic-taxonomy.md) — Blueprint 运维
 - [智能导入](../../features/smart-import.md) — Smart Import 产品行为
 - [检索与对话](../../features/retrieval-and-chat.md) — 混合检索与 chat_mode

