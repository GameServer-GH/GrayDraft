# GrayDraft 项目中的 AI 算法与作用总览（学习文档）

> 面向：希望从“能跑”走向“可解释、可调优、可扩展”的读者  
> 依据：`docs/architecture/系统逻辑总览.md`（以当前代码实现为准）

本文把 GrayDraft 里用到的 **AI/类 AI 算法**（含 LLM/RAG、向量检索、融合、重排、门控、缓存与多模态去重等）按业务链路拆开讲清楚：**它解决什么问题、在系统里怎么接、输入输出是什么、调参时会影响什么**。你可以把它当作阅读 `internal/core` / `internal/worker` / `internal/gateway` 相关代码前的“概念地图”。

> **自研算法深度学习**（逐模块拆解、判定树、公式、代码路径、自测题）：[GrayDraft-自研AI算法深度学习.md](02-custom-algorithms.md)

### 算法来源标记（阅读本文时看标题后缀）

| 标记 | 含义 | 判断标准 |
|------|------|----------|
| 🟢 **已有/标准** | 业界常见算法或基础设施能力 | 直接调用外部模型/API，或实现与教科书/开源方案基本一致，改动主要是参数与接入 |
| 🟡 **标准+改良** | 基于标准算法，但有明显工程改造 | 在 RRF、分块、重排、缓存等标准做法上叠加了 GrayDraft 特有的权重、格式、路由或组合策略 |
| 🔵 **自研** | GrayDraft 原创设计或规则体系 | 四维语义、瀑布退避、分级门控、Blueprint 进化等，属于本项目为解决知识库场景专门设计的逻辑 |

> 说明：LLM / Embedding / Rerank / VL 等**模型推理本身**均通过 Gateway 调用第三方，不属于“自研模型”；自研主要体现在**编排、打分、门控、标签体系与缓存键设计**上。

### 算法来源总览表

| 算法 / 能力 | 标记 | 代码落点（参考） |
|-------------|------|------------------|
| MD5 内容哈希去重 | 🟢 | `content_hash`、`answercache/key.go` |
| pHash 感知哈希 | 🟢 | `multimodal/phash.go` |
| pHash 三级校验链（颜色 + SSIM） | 🟡 | `multimodal/ssim.go` |
| 质量门禁（长度/敏感规则） | 🟢 | `worker/quality_gate/` |
| 固定窗口分块 + overlap | 🟢 | `worker/chunkers/semantic_chunker.py` |
| 段落感知 / 多模态事实表优先分块 | 🟡 | `semantic_chunker.py`（`chunk_text_by_sections`、`chunk_text_multimodal_consolidated`） |
| Embedding 向量化（Gateway 调用） | 🟢 | `embedder/client.go`、Gateway |
| pgvector 余弦相似度检索 | 🟢 | `chunk_repo.go` |
| structured 向量第三路召回 | 🟡 | `embedding_structured` + 三路 RRF |
| LLM 标题推断 / 拆分 / 分析纠错 | 🟢 | `title_llm.go`、`split_llm.go`、`analyzer.go`（模型外调，提示词与校验自研） |
| 智能导入置信度五检（Pass/Warning/Reject） | 🔵 | `smartimport/confidence_label.go` |
| 意图分类器（规则 + 可选 LLM） | 🟡 | `classifier/classifier.go` |
| Query4D 提取与对齐 | 🔵 | `retriever/query4d.go`、`taxonomyevolve/aligner.go` |
| 4D 瀑布退避（Tier 1→5） | 🔵 | `retriever/waterfall.go` |
| BoostSemantic4D 软加分 | 🔵 | `retriever/query4d.go` |
| 全文 ILIKE + 中文分词 fallback | 🟡 | `search_terms.go` |
| RRF 多路融合 | 🟡 | `retriever/hybrid.go`（标准 RRF + 加权三路） |
| 两阶段 Rerank（L1/L2） | 🟡 | `reranker/worker_client.go` + Worker |
| Gateway Rerank 模型 | 🟢 | `worker/rerank/gateway.py` |
| 词法 Rerank 降级（F1 + bigram + 标题 bonus） | 🟡 | `worker/rerank/scorer.py` |
| 检索分级 A/B/C/D | 🔵 | `confidence/gate.go` |
| 门控综合分公式 + 动态阈值 | 🔵 | `confidence/gate.go`、`confidence/table_hit.go` |
| 检索/答案缓存 L1/L2 | 🟡 | `cache/`、`answercache/`（标准缓存 + 版本/4D 键设计） |
| L2 语义相似缓存 | 🟡 | `cache/manager.go` |
| 答案一致性自检（无额外 LLM） | 🟡 | `confidence/answer.go` |
| RAG 流式生成 / chat_mode 路由 | 🟡 | `chat/service.go`（标准 RAG + 自研分级路由） |
| Blueprint 自发现 / Taxonomy Evolve | 🔵 | `taxonomyevolve/` |
| Embedding Aligner | 🔵 | `taxonomyevolve/aligner.go` |
| 影子回填（Shadow Backfill） | 🟡 | `taxonomyevolve/backfill.go` |
| 知识分 lifecycle（knowledge_score） | 🔵 | `smartimport/knowledge_score.go` |
| 分区校准器 Calibrator（P20/P60） | 🔵 | `confidence/calibrator.go` |

---

## 0. 一句话全景（从链路看算法）

- **入库（Ingest）**：多模态预处理（VL/OCR/识图分流）→ 质量门禁 → 解析 → **语义分块** → **Embedding 向量化** →（可选）自动 QA 生成 → `index_ready=true`
- **检索（Retrieval）**：**意图分类** → **Query4D 语义标签提取** → **4D 瀑布退避** →（并行）全文召回 + 向量召回 + structured 向量路 → **RRF 融合** → **两阶段重排（rerank L1/L2）**
- **对话（Chat/RAG）**：答案缓存 L1/L2 →（miss）检索 → **检索分级 A/B/C/D（门控）** →（A/B/C 部分）LLM 流式生成 →（可选）**答案一致性自检** → 写回答案缓存
- **语义进化（Evolve）**：Blueprint 自发现 → Embedding 对齐（Aligner）→ 影子回填（backfill）驱动 Worker 批处理更新

---

## 1. 入库/智能导入链路的算法

### 1.1 多模态预处理与去重：MD5 / pHash 缓存 🟢🟡

> **来源**：MD5 / pHash 算法本身为 🟢 已有/标准；**三级校验链**（Hamming 前缀召回 → 颜色校验 → SSIM）与多图合并策略为 🟡 标准+改良。

**解决的问题**

- 图片/多媒体附件重复提交、重复解析会浪费成本（识图/LLM/VL 调用、存储、索引）。
- 多图导入时，需要把“附件信息”稳定地并入正文/扩展字段，避免同一张图在不同任务间反复处理。

**核心方法**

- **MD5**：对二进制内容做强一致性哈希，判断“完全相同文件”。
- **pHash（感知哈希）**：对图像做感知摘要，判断“视觉上几乎相同但二进制不同”的重复（例如压缩、尺寸变化）。

**在 GrayDraft 的落点**

- 文档中提到：`service/multimodal/` + `multimodal_content_cache`，用于 “MD5/pHash 缓存、多图合并表”。
- 结果会影响智能导入阶段的“媒体识图入正文”、以及 `extensions.multimodal` 等扩展信息的组织。

**输入 / 输出**

- 输入：附件二进制、（可选）用户给的短说明。
- 输出：缓存命中/写入结果（md5、phash、媒体解析产物引用），并入导入正文与 extensions。

**调参/注意点**

- pHash 的“相似阈值”决定误判率：阈值太松会把不同图当成同一张（错缓存）；太严则命中率低（成本上升）。
- MD5 只能判定“完全相同”，对“同图不同编码”无效；pHash 用于补齐这一块。

---

### 1.2 质量门禁（quality gate） 🟢

**解决的问题**

- 垃圾文本、极短内容、明显无效输入（或命中敏感规则）不应进入索引，否则会污染检索与 RAG。

**核心方法**

- 规则门禁：长度阈值、敏感/黑名单规则等（文档描述为 `quality_gate`）。

**输入 / 输出**

- 输入：原始正文/元数据。
- 输出：是否允许入库、失败原因、（可选）任务直接失败或进入 DLQ/人工处理链路。

**调参/注意点**

- 门禁过严会导致“明明有用但被拒”；过松会导致向量库被噪声淹没，后续重排/门控压力变大。

---

### 1.3 语义分块（chunking：size/overlap） 🟢🟡

> **来源**：固定窗口 + overlap 为 🟢；段落边界感知、`##` 分段、多模态事实表优先单块策略为 🟡。

**解决的问题**

- LLM/RAG 的引用粒度需要“可读且可定位”；太大导致检索粗糙、注入上下文浪费；太小导致语义断裂、召回变差。

**核心方法**

- 按配置的 `size/overlap` 做切片（文档称 “语义分块”）。常见策略：
  - 固定窗口 + overlap（保留上下文连续性）
  - 基于段落/标题边界的分块（更自然）

**在 GrayDraft 的落点**

- Worker 流水线 `chunk_text`，切片写入 `kb_chunks`，并参与后续全文/向量检索与 RAG 注入。

**输入 / 输出**

- 输入：规范化后的正文（可含标题、实体、导入说明等）。
- 输出：`chunks[]`（每块含文本、位置、与文档的关系），后续会生成 embedding 与 `search_text`。

**调参/注意点**

- overlap 增大：召回更稳但重复注入变多、索引膨胀；overlap 太小：跨句信息丢失明显。
- chunk 粒度要与重排 passage 格式一致（见 2.6），否则重排学习不到稳定的语义结构。

---

### 1.4 Embedding 向量化（pgvector + structured embedding） 🟢🟡

> **来源**：Embedding 模型调用与 pgvector 余弦检索为 🟢；**双路向量**（`embedding` + `embedding_structured`）及第三路 RRF 融合为 🟡。

**解决的问题**

- 让“语义相近但字面不同”的内容可被召回；同时为混合检索与重排提供特征。

**核心方法**

- 通过 Gateway 调用 **Embedding 模型**，把每个 chunk 映射到向量空间。
- 文档中提到两路向量：
  - `embedding`：常规文本 embedding
  - `embedding_structured`：结构化向量（可作为第三路召回/加分）

**在 GrayDraft 的落点**

- `kb_chunks.embedding`（pgvector），检索时做余弦相似度；structured 向量可作为“第三路”。
- `index_ready=true` 是向量化完成的关键状态；检索 SQL 只查 `index_ready=true` 的切片。

**输入 / 输出**

- 输入：chunk 文本（可能带 `workspace_context` 前缀或上下文配置）。
- 输出：向量 + 与 chunk 的绑定关系，写回 Postgres。

**调参/注意点**

- embedding 模型/维度变化会引入“新旧向量不可比”，需要重建或做版本隔离。
- structured embedding 更适合表格/字段化内容；如果数据域偏结构化，它的权重/bonus 会显著影响门控与分级（见 3.2）。

---

### 1.5 智能导入：标题推断（LLM）与标题校验 🟢🟡

> **来源**：LLM 推理为 🟢（Gateway 外调）；标题校验规则（拒绝 JSON/导入意图句等）为 🟡 工程改良。

**解决的问题**

- 用户给的 raw_text 往往是“聊天式/意图句/碎片化笔记”，直接作为标题会让文档管理、引用展示、检索质量变差。

**核心方法**

- `InferTitleLLM`：用 LLM 提取/生成标题。
- `title_validate`：拒绝不合规标题（例如把 JSON、导入意图句当标题）。

**输入 / 输出**

- 输入：预处理后的正文（含多模态合并结果）、用户短说明等。
- 输出：标题字符串（或回退策略），并进入后续拆分/分析/去重链路。

**调参/注意点**

- 标题是一类“强特征”：它同时影响全文召回、重排 passage 结构、以及检索分级中的 `TitleHitBonus`（见 3.2）。

---

### 1.6 智能导入：拆分（split：single / multi_segment + line_range） 🟡

> **来源**：拆分思路常见，但 `line_range`、多段 `Analyze`、`plan_segments` 编排为 GrayDraft 自研流水线（🟡）。

**解决的问题**

- 一次导入可能包含多条笔记/多段不同主题内容；拆分能提升每条文档的语义纯度，改善召回与引用可信度。

**核心方法**

- 规则拆分 +（可选）LLM 拆分：输出多段 `items[]`，并携带 `line_range` 等定位信息。

**输入 / 输出**

- 输入：raw_text（可能已合并多模态内容）。
- 输出：单段或多段的导入预览、各段的置信度、以及后续可提交的任务列表。

**调参/注意点**

- 拆分过度会制造碎片文档、增加索引规模；拆分不足会造成“一个文档多主题”，Query4D 与 4D 瀑布更难命中。

---

### 1.7 智能导入：分析（规则 + 可选 LLM 修正 kb_type） 🟡

**解决的问题**

- 需要把内容落到正确的知识库分区（kb_type / storage_partition），否则后续检索会“查不到/查不准”。

**核心方法**

- 规则分析为主，LLM 作为纠错/补充：修正 `kb_type` 或补全元信息。

**输入 / 输出**

- 输入：标题 + 正文 +（可选）工作区背景（context_profile / workspace_context）。
- 输出：目标分区、结构化扩展字段、以及用于置信度计算的特征。

---

### 1.8 智能导入：去重（content_hash / target_doc_id）与版本 🟢

**解决的问题**

- 同一内容重复导入导致索引污染；同标题内容更新需要可控的“更新/新增”语义。

**核心方法**

- `content_hash`：内容级哈希判断 unchanged。
- `target_doc_id`：允许强制覆盖更新。
- `kb_version`：同标题内容变化时版本递增。

---

### 1.9 置信度门控（smart import confidence） 🔵

> **来源**：`EvaluateConfidence` 五检（结构/实体密度/噪声/标题/实体溯源）+ Pass/Warning/Reject 三级标签，为 GrayDraft 自研可解释门控，非通用 ML 分类器。

**解决的问题**

- 让系统对“我是否理解了用户要导入什么”做可解释的量化判断，避免错误入库。

**核心方法**

- 置信度 `confidence` 与阈值 `min_confidence` 比较；低于阈值时 **仍返回预览**，但不允许 submit（文档：422 仍带预览）。

**输入 / 输出**

- 输入：拆分/分析/去重/多模态处理后的综合特征。
- 输出：`import_allowed`（布尔）+ `confidence`（数值）+ 预览内容。

---

## 2. 混合检索链路的算法

### 2.1 意图分类器（classifier：fact / concept / proper_noun / hybrid） 🟡

> **来源**：基于正则/启发式规则打分（<5ms），低置信时可选 LLM 兜底；规则与权重路由为 GrayDraft 定制，非通用 NLP 分类模型。

**解决的问题**

- 不同类型查询对召回信号的依赖不同：
  - 事实/定义类（fact/concept）往往需要“语义相近 + 证据片段”
  - 专有名词（proper_noun）更依赖标题/实体/精确匹配
  - 混合型（hybrid）需要在 recall 与 precision 间动态权衡

**核心方法**

- 轻量分类器（可由规则/模型实现）产出 `query_intent` 与权重。

**在 GrayDraft 的落点**

- 文档：分类器会调整 `fulltext_weight / vector_weight`，并可启用全局索引 boost。

**输入 / 输出**

- 输入：query 文本（可结合工作区背景）。
- 输出：`query_intent` + 权重系数 +（可选）路由开关。

**调参/注意点**

- 分类错了不会“立刻报错”，而是体现在召回质量下降、分级变差、RAG 走降级路径。

---

### 2.2 Query4D：domain / scene / style / entities 提取与对齐 🔵

> **来源**：四维语义标签体系、LLM-first + 规则补洞、实体 sanitize/别名对齐，为 GrayDraft 核心自研设计。

**解决的问题**

- 在知识库/工作区规模变大时，单纯相似度会召回跨域内容；需要更强的“语义约束与可解释过滤/加权”。

**核心方法**

- 从 query 提取 4 维标签：
  - **domain**：领域（如“运维/产品/研发/法律”）
  - **scene**：场景（如“排障/方案/面试/文档写作”）
  - **style**：表达风格（如“要点/教程/代码/规范”）
  - **entities**：实体（项目名、人名、模块名、概念名）
- 实体会做 sanitize、别名对齐，降低噪声。
- 提取方式：规则为主，必要时可用 LLM 辅助。

**在 GrayDraft 的落点**

- `query_semantic_4d` 会出现在检索与对话 SSE `meta/done` 中，用于调试与可观测。

**输入 / 输出**

- 输入：query +（可选）workspace_context/context_profile。
- 输出：4D 标签对象 + `semantic_4d_fingerprint`（用于缓存 key 与失效策略）。

**调参/注意点**

- entities 过于激进会让 4D 过滤“误杀”；过于宽松则失去约束意义，系统回到纯相似度召回。

---

### 2.3 4D 瀑布（Tier 1→5）与软过滤（BoostSemantic4D） 🔵

> **来源**：Tier 1→5 SQL 退避 + Tier5 软加分 + 乐观并发（strict/soft 并行）均为 GrayDraft 自研检索策略。

**解决的问题**

- 4D 约束能提升精度，但遇到标签不全/噪声时会召回为零；需要一种“退避”机制保证鲁棒性。

**核心方法**

- **Tier 1→5 瀑布退避**：从最严格过滤逐步放宽到软加分（文档：Tier 5 `_wf_soft` 仅软加分）。
- 通过 `retrieval_trace` 记录命中的 tier、suffix、对齐方式等，便于诊断。

**输入 / 输出**

- 输入：Query4D 标签 + 召回候选。
- 输出：每一 tier 的策略与结果，最终 top candidates。

**调参/注意点**

- “严格 tier”决定 precision；“软 tier”决定 recall 与稳定性。调参时要结合 rerank 是否足够强。

---

### 2.4 全文召回（search_text + ILIKE fallback） 🟡

> **来源**：PostgreSQL ILIKE / 全文为 🟢；中文 `search_terms` 分词 fallback 与 `search_text` 字段组织为 🟡。

**解决的问题**

- 中文场景、专有名词、代码符号、版本号等往往更适合字面匹配；纯向量召回会漏掉。

**核心方法**

- `search_text`（标题+实体+正文）上做全文/ILIKE 召回，作为混合检索的一条信号通路。

**输入 / 输出**

- 输入：query 词项（可能会做 search_terms 处理）。
- 输出：全文候选集合与匹配得分/排序信号。

---

### 2.5 向量召回（pgvector 余弦）与 structured 向量第三路 🟢🟡

> **来源**：pgvector 余弦为 🟢；structured 向量作为独立第三路参与 RRF 为 🟡。

**解决的问题**

- 解决同义改写、表达差异导致的召回缺失；同时通过 structured 向量更好覆盖表格/字段化信息。

**核心方法**

- `pgvector` 余弦相似度召回。
- structured 向量作为第三路（可选）提升特定数据域召回。

---

### 2.6 RRF 融合（Reciprocal Rank Fusion） 🟡

> **来源**：RRF 公式为业界标准（🟢）；GrayDraft 做了**按意图加权的加权 RRF**（全文/向量/structured 三路权重可调）及多 KB 分区合并，属 🟡。

**解决的问题**

- 全文与向量各有长短板：一个擅长字面，一个擅长语义；需要一种“稳健融合”避免单路失效。

**核心方法**

- **RRF**：对多路排序结果按倒数排名加权求和：
  - 直觉：只要某个结果在任一路排得靠前，就会得到较高融合分；不会被另一条路“完全淹没”。
  - 参数：`rrf_constant_k` 控制“排名差距的衰减速度”。

**输入 / 输出**

- 输入：多路召回的排名列表（全文、向量、structured 等）。
- 输出：融合后的候选排名。

**调参/注意点**

- `k` 越大：更像“平均”，更保守；`k` 越小：更偏向头部，可能更激进。

---

### 2.7 两阶段重排（rerank L1/L2） 🟡

> **来源**：两阶段重排是 RAG 常见范式（🟢 概念）；本项目实现了 L1→L2 编排、combined 单调用优化、Gateway 模型 + 词法降级双路径，以及 `标题 | 实体 | 正文` passage 格式，属 🟡。

**解决的问题**

- 召回阶段强调“别漏掉”；重排阶段强调“把最相关/最可信的放前面”，并为门控/分级提供更可靠的质量信号。

**核心方法**

- **L1 rerank**：便宜/快的重排，把候选缩小到更小集合。
- **L2 rerank**：更强/更贵的重排，得到最终排序与分数。
- passage 格式：`标题 | 实体 | 正文`（文档明确写出），让模型能利用结构化提示。

**输入 / 输出**

- 输入：query + 候选 passages。
- 输出：rerank 分数、最终 top_k、以及可写入 `retrieval_trace`/门控分数的特征。

**调参/注意点**

- rerank 池大小影响成本与效果：池太小可能“召回进来但没来得及重排”；太大成本高。
- passage 组织方式对 rerank 质量非常敏感；标题/实体质量差会直接拖累 rerank。

---

## 3. 对话 RAG 链路的算法（门控、分级、缓存、自检）

### 3.1 检索分级 A/B/C/D（EvaluateRetrievalGrade） 🔵

> **来源**：A/B/C/D 四级分级、实体 Jaccard 匹配、高质量 grounding 判定等，为 GrayDraft 自研问答可靠性体系。

**解决的问题**

- “有没有参考”不是二元：参考可能强/中/弱/无。分级能让系统做更细粒度的路由：
  - **A/B**：可以强注入，走高质量 RAG
  - **C**：稀疏注入，尽量利用 KB 但避免幻觉式强引用
  - **D**：无参考，走通用 LLM 或固定拒答话术（kb_only 模式）

**核心方法**

- 根据 top chunk 的质量、实体匹配、标题命中、structured bonus 等综合特征给出等级与原因码。

**输出（面向 UI/调试）**

- `retrieval_grade`（A/B/C/D）、`coverage_score`、`grade_reason`、`chunks_summary` 等，作为 SSE 首个事件输出（文档：`retrieval_grade → thinking → meta → chunk…`）。

---

### 3.2 门控综合分（Final Score）与动态阈值 🔵

> **来源**：`Final = Rerank×0.6 + Vector×0.3 + TitleHitBonus + StructuredTableBonus` 及按 fact/concept 动态阈值，为 GrayDraft 自研打分公式。

**解决的问题**

- 不同意图对“可用证据”的要求不同；同样的相似度分数，在 fact 查询下可能不够可靠，但在 concept 查询下可接受。

**核心方法（文档给出的简化公式）**

\[
Final = Rerank×0.6 + Vector×0.3 + TitleHitBonus(0.15) + StructuredTableBonus
\]

- 并按 `query_intent` 采用动态阈值（示例：fact 0.25 / concept 0.15 / default 0.20）。

**调参/注意点**

- 提高阈值：更谨慎，减少“拿低质 chunk 强行 RAG”的情况，但更容易降级到通用 LLM。
- 降低阈值：更激进，能多用 KB，但需要依赖 answer self-check / 逐句标注等治理手段兜底。

---

### 3.3 答案缓存（Answer Cache L1/L2）与版本感知失效 🟡

> **来源**：L1 Ristretto + L2 Redis 为 🟢 标准分层缓存；**键设计**（catalog 版本 + 4D fingerprint + grade）与 L2 语义相似命中为 🟡。

**解决的问题**

- 高频重复问题（尤其是 Agent 面板调试、常见知识点）不应每次都走检索 + 生成，降低延迟与成本。
- 但缓存必须对“知识库更新”敏感，否则会回放过期答案。

**核心方法**

- Key 绑定：
  - `kb_catalog_version`（库版本，单调递增）
  - `semantic_4d_fingerprint`（语义指纹）
- 分层缓存：
  - L1（进程内，比如 Ristretto）
  - L2（Redis，且“版本感知”）
- 特例：时效性 query（`IsTimelinessQuery`）自动跳过答案缓存。

**输入 / 输出**

- 输入：query + 语义指纹 + catalog 版本等。
- 输出：命中则回放 `meta → chunk → done`；miss 则进入检索/生成。

**调参/注意点**

- cache key 太粗会误命中（答非所问）；太细会命中率很低（失去意义）。
- 强制把“库版本”纳入 key 是避免陈旧答案的关键。

---

### 3.4 答案一致性自检（Answer Self-Check / EvaluateAnswerConfidence） 🟡

> **来源**：基于 token overlap + 专名/数字一致性 + 过度自信检测的启发式，**不额外调用 LLM**；算法思路常见，规则组合为 GrayDraft 定制。

**解决的问题**

- RAG 仍可能出现“回答与引用不一致”“过度推断”；需要一个可量化的自检指标辅助 UI 告警、反馈闭环与路由决策。

**核心方法**

- 对“答案与引用 chunk 的一致性”做评估，输出 `answer_confidence` 与明细 `answer_confidence_detail`。

**输入 / 输出**

- 输入：最终答案文本 + 引用 chunks/sources。
- 输出：0–1 的 `answer_confidence`，用于 `done` 事件与可观测/反馈。

**调参/注意点**

- 自检开启会增加成本与延迟；通常用于对可靠性要求高的场景，或对低分级结果做重点治理。

---

## 4. 检索/对话治理与进化相关算法

### 4.1 Blueprint 自发现（Taxonomy Evolve） 🔵

> **来源**：工作区驱动的 Blueprint 生成、Delta/Merge/Birth 进化流程，为 GrayDraft 自研语义体系。

**解决的问题**

- 随着内容规模与主题增长，固定 taxonomy/标签会逐渐不适配；需要自动发现新概念/新结构并演化。

**核心方法**

- 自动进化 Blueprint（文档描述为 “自发现、Delta/Merge/Birth” 一类流程），为 4D 标签与检索约束提供更贴合的数据结构。

---

### 4.2 Embedding Aligner（向量对齐） 🔵

> **来源**：将自由文本 scene/style 映射到 Blueprint 标准标签（字符串相似度 + embedding 最近邻），为 GrayDraft 自研对齐器。

**解决的问题**

- embedding 模型升级、数据分布漂移会让向量空间的“相似度语义”发生变化，影响召回稳定性与分数可比性。

**核心方法**

- Aligner 用于做对齐/校正（实现细节以 `service/taxonomyevolve/` 为准），目标是让检索行为更稳定、可控。

---

### 4.3 影子回填（Shadow Backfill） 🟡

> **来源**：Blueprint 版本 bump 后异步 Worker 批处理更新存量切片，属常见工程模式，但触发条件与 4D 标签回填逻辑为 GrayDraft 定制。

**解决的问题**

- 进化/对齐后的新特征（新标签、新 embedding 方案）需要逐步应用到存量数据；在线全量重建风险与成本高。

**核心方法**

- 通过 Worker 批处理进行“影子回填”：不打断主链路，逐步把新策略结果写回或生成增量索引。

---

## 5. 学习建议：按顺序读什么、怎么验证理解

- **先读链路**：入库（1.x）→ 检索（2.x）→ 对话（3.x）→ 进化（4.x）。
- **用诊断接口对照概念**：文档提到 `/api/v1/retrieval/diagnose` 会返回 Query4D、waterfall、gate/grade、top chunks；最适合拿来“看算法是否按你想的在工作”。
- **抓住三类分数**：
  - 检索相似度（vector/fulltext/rrf）
  - 重排分（rerank）
  - 门控/分级/置信度（confidence / retrieval_grade / answer_confidence）

---

## 6. 术语速查（与文档/接口字段对齐）

- **Query4D**：`domain/scene/style/entities` 四维语义标签
- **4D waterfall**：Tier 1→5 的退避策略，最终可能变为软加分
- **RRF**：召回融合算法（全文 + 向量 + structured）
- **rerank L1/L2**：两阶段重排，提高最终排序可靠性
- **retrieval_grade**：检索分级 A/B/C/D（SSE 首个事件）
- **confidence**：检索置信度（用于门控与路由）
- **answer_confidence**：答案一致性自检置信度（可选）

---

## 7. 自研 vs 已有：面试/简历可怎么说

### 🔵 可重点讲的自研能力（差异化）

1. **Query4D + 4D 瀑布**：用四维语义标签约束检索，Tier 退避保证“查得准”与“查得到”的平衡。
2. **检索分级 A/B/C/D**：替代二元 gate，驱动 RAG 注入强度、`chat_mode` 路由与答案缓存策略。
3. **智能导入置信度五检**：可解释的 Pass/Warning/Reject，而非黑盒分数。
4. **Taxonomy Evolve**：工作区 Blueprint 自发现 + Embedding Aligner + 影子回填，让标签体系随内容进化。
5. **门控综合分公式**：融合 rerank/vector/标题/结构化表格多信号，按 query 意图动态阈值。

### 🟡 可讲的工程改良（标准算法 + 落地细节）

- 加权三路 RRF、structured 向量第三路、多 KB 分区并行合并
- 两阶段 rerank 编排 + Gateway/词法双路径降级
- 版本感知 + 4D fingerprint 的答案/检索缓存
- 多模态 pHash 三级校验、事实表优先分块策略

### 🟢 明确是“调用/接入”而非自研

- LLM 流式生成、Embedding、Gateway Rerank、VL 识图（均走 Model Gateway）
- pgvector、PostgreSQL ILIKE、MD5/pHash、Ristretto/Redis 缓存基础设施

