# AI Agent 面试问答大全

> **定位**：面向 AI Agent 应用研发 / RAG 平台 / AI 工程化岗位的高频问答文档。  
> **使用方式**：先读 [AI-Agent面试全局视角](02-interview-overview.md) 建立全局框架，再用本文逐题训练表达。
> **回答原则**：每个问题尽量按「概念定义 → 项目落点 → 工程权衡 → 风险治理」四步回答。

---

## 0. 面试答题总模板

### 0.1 四步答题法

```text
1. 先定义：这个概念是什么，解决什么问题。
2. 再落项目：GrayDraft / GrayDraft 里对应哪个模块、链路、技术栈。
3. 讲权衡：为什么这么设计，有什么替代方案。
4. 讲治理：如何处理成本、延迟、失败、安全、可观测。
```

### 0.2 一句话人设

> 我不是只会调 Prompt 的应用层开发，而是能把 Agent / RAG / Tool Use / Memory / Gateway / 缓存 / 降级 / 可观测做成生产系统的 AI 应用工程师。

---

# 一、基础概念篇

## Q1：什么是 AI Agent？

**回答**：

AI Agent 是以 LLM 为认知核心，结合 **规划 Planning、记忆 Memory、工具调用 Tool Use、环境反馈 Observation** 的系统。它不是单次 Prompt 问答，而是能根据任务目标和外部反馈循环决策：

```text
observe → think / plan → act / tool use → observe → final answer
```

在项目里，GrayDraft 更偏 Agent 的 **RAG 与知识治理执行层**，提供检索、入库、问答、模型网关等能力；GrayDraft 更偏 **产品 Agent 编排层**，通过 BFF 和业务 Skill 把 GrayDraft 能力接到教育场景业务里。

---

## Q2：Agent、Chatbot、Workflow、RAG 有什么区别？

**回答**：

| 类型 | 控制流 | 外部知识 | 工具调用 | 典型场景 |
|------|--------|----------|----------|----------|
| Chatbot | 多轮对话 | 可有可无 | 弱 | 客服、闲聊 |
| RAG | 固定检索 → 生成 | 强 | 通常固定 | 企业知识问答 |
| Workflow | 预定义流程 / DAG | 取决于节点 | 固定节点 | 审核、入库、审批 |
| Agent | 运行时动态决策 | 可接 RAG | 动态选择 | 多步任务、自动执行 |

GrayDraft Core 的 Agent v2（`knowledge.semantic` + AnswerGate）是 RAG 执行层；Smart Import 更像领域 Workflow；GrayDraft 的 14 Skill 编排更接近业务 Agent。

---

## Q3：AI Agent 的核心组成是什么？

**回答**：

常见拆法是五部分：

1. **LLM**：理解、推理、生成。
2. **Planning**：拆解任务、选择步骤。
3. **Memory**：保存短期上下文和长期知识。
4. **Tool Use**：调用搜索、数据库、业务 API、外部服务。
5. **Guardrails / Observability**：权限、安全、降级、日志、质量评估。

GrayDraft 重点做 Memory 的长期知识层和 RAG 执行层；GrayDraft 重点做 Tool Use 和业务上下文编排。

---

## Q4：为什么说 RAG 是 Agent 的长期记忆？

**回答**：

Agent 的 Memory 不只是对话上下文。长期知识通常不能全部塞进上下文窗口，所以需要外部存储和检索机制。RAG 通过文档库、向量库、检索和重排，把长期知识按需取回给模型，因此是 Agent 长期记忆的重要实现方式。

在 GrayDraft 里，文档、chunk、pgvector 向量、taxonomy、knowledge_score 共同组成长期知识记忆；GrayDraft 根据授权的 `kb_type` 选择知识空间。

---

## Q5：什么是 ReAct？

**回答**：

ReAct 是 Reasoning + Acting 的 Agent 范式，让模型交替执行：

```text
Thought → Action → Observation → Thought → ... → Final Answer
```

它的价值是把推理和工具调用结合起来。模型不是一次性回答，而是可以先思考、再调用工具、观察结果、继续推理。

项目中，GrayDraft Core 不是通用 ReAct 框架，但 GrayDraft 的 Skill Router 可以借鉴 ReAct：识别意图 → 选择 Skill → 得到 observation → 组织最终回答。

---

## Q6：什么是 Plan-and-Execute？和 ReAct 有什么区别？

**回答**：

Plan-and-Execute 是先生成完整计划，再逐步执行。ReAct 是边思考边行动。

| 模式 | 特点 | 适合场景 |
|------|------|----------|
| ReAct | 动态、灵活，每一步根据观察调整 | 短任务、工具少、需要解释 |
| Plan-and-Execute | 先规划再执行，结构更清晰 | 长任务、多步骤、可并行 |

如果是「帮用户查询某教程资料」这种短任务，ReAct 足够；如果是「分析全站内容并生成知识库迁移计划」，Plan-and-Execute 更合适。

---

## Q7：Agent 为什么难以稳定？

**回答**：

Agent 不稳定主要来自：

1. **规划不稳定**：模型可能拆错步骤。
2. **工具选择错误**：选错工具或参数错。
3. **外部环境失败**：API 超时、模型限流、数据库异常。
4. **上下文污染**：历史对话、工具结果太多导致模型误判。
5. **成本和延迟不可控**：循环步数过多。
6. **权限风险**：模型误调用敏感操作。

工程上要通过工具 schema、权限过滤、最大步数、超时、降级、trace_id、审计日志来治理。

---

## Q8：你怎么看「Agent = Prompt 工程」？

**回答**：

Prompt 很重要，但 Agent 远不止 Prompt。生产级 Agent 还需要：

- 知识库和检索质量。
- 工具 schema 和执行器。
- 权限、安全和审计。
- 缓存、降级和成本控制。
- 可观测、评估和反馈闭环。

我的项目重点就是把 Prompt 之外的工程部分补齐：GrayDraft 做 RAG、检索、网关、缓存、可观测；GrayDraft 做业务 Skill 和产品闭环。

---

# 二、项目总览篇

## Q9：请介绍一下你的 AI Agent 项目。

**回答**：

我主要有两个相关项目：

1. **GrayDraft**：面向 Agent 场景的知识库中台，负责 Smart Import、混合检索、SSE RAG、模型网关、语义进化和 RAGOps。
2. **GrayDraft**：教育场景 + AI 智能化平台，把 GrayDraft 接入教育场景业务，通过 BFF 和 `edu_workspace_v1` 做带引用的讲解与笔记沉淀。

整体闭环是：导入教程或课件 → 检索取证 → 带引用讲解 → 笔记审批后再入库。

---

## Q10：GrayDraft 的定位是什么？

**回答**：

GrayDraft 是 Agent 知识库中台，不是单纯文档问答 Demo。它的定位是为上层 Agent 提供可复用的长期知识能力，包括：

- 智能入库：解析、拆分、质量门禁、向量化。
- 混合检索：全文 + 向量 + structured 向量 + RRF + Rerank。
- 流式 RAG：SSE、citations、答案自检、缓存。
- 模型网关：多 Provider 路由、熔断、重试、流式代理。
- 语义进化：Blueprint、Aligner、影子回填。
- 运维治理：DLQ、trace_id、Prometheus、Feedback。

---

## Q11：GrayDraft 和 GrayDraft 是什么关系？

**回答**：

GrayDraft 是业务产品层，GrayDraft 是 AI 知识中台层。

- GrayDraft 负责教程、课件、论文摘录和笔记。
- GrayDraft 负责把这些业务内容沉淀成可检索、可问答的知识。
- GrayDraft 通过 BFF / Skill 调用 GrayDraft 的 Smart Import、Retrieval、Agent v2 对话等能力。

比如一份书生教程导入后进入 `tutorial`；学员提问时，Agent 只在授权分区里检索，并引用原文。

---

## Q12：你在项目中承担什么角色？

**回答**：

我承担核心开发和架构设计角色，主要负责：

- GrayDraft 5 服务 Monorepo 的架构设计。
- Smart Import 入库流水线。
- 混合检索、Query4D、4D 瀑布、A/B/C/D 分级门控。
- SSE RAG 对话和 Gateway 模型路由。
- GrayDraft 与 GrayDraft 的 BFF / Skill 集成。
- Docker 部署、DB Schema 迭代、缓存、可观测和降级。

回答时可以强调：自己不是只做某个接口，而是从数据入库、检索、生成、前端体验到部署运维都有参与。

---

## Q13：GrayDraft 为什么拆成 Core / Worker / Gateway / Console / Ops？

**回答**：

这是为了职责分离和稳定性：

| 服务 | 职责 |
|------|------|
| Core | 对外 API、检索、对话、入库控制 |
| Worker | 异步任务、分块、Embedding、多模态、回填 |
| Gateway | 统一模型调用、多 Provider、熔断、流式代理 |
| Console | 管理和可视化 |
| Ops | 运维、DLQ、观测、数据治理 |

如果全部放在一个服务里，模型调用、异步任务、API 请求会互相影响。拆分后，耗时入库可以异步处理，模型故障可以在 Gateway 层降级，Core 保持主链路稳定。

---

## Q14：项目里哪些是自研，哪些是接入？

**回答**：

我会明确区分：

| 能力 | 定位 |
|------|------|
| LLM / Embedding / Rerank / VLM | 外部模型接入 |
| pgvector | 标准向量检索组件 |
| RRF | 经典算法，项目中做工程组合 |
| Query4D | 自研语义约束与检索编排 |
| 4D 瀑布退避 | 自研召回策略 |
| A/B/C/D 检索分级 | 自研 evidence 门控规则 |
| Smart Import 置信度五检 | 自研入库质量规则 |
| Blueprint 语义进化 | 自研 taxonomy 编排 |

推荐话术：

> 模型推理本身不是我自研，我的价值在于把标准模型能力组合成可解释、可降级、可观测的 Agent 知识治理系统。

---

# 三、RAG 基础篇

## Q15：什么是 RAG？

**回答**：

RAG 是 Retrieval-Augmented Generation，检索增强生成。它先从外部知识库检索相关证据，再把证据组装到 Prompt 中，让 LLM 基于证据生成答案。

标准链路是：

```text
文档解析 → 分块 → Embedding → 向量库 / 索引 → 检索 → Rerank → Prompt → 生成 → 引用 / 反馈
```

它解决模型知识过期、领域知识不足、幻觉不可控的问题。

---

## Q16：RAG 为什么不能完全解决幻觉？

**回答**：

因为 RAG 只能提供证据，但最终生成仍由 LLM 完成，幻觉可能来自：

1. 检索没召回正确证据。
2. 召回了错误或低质量证据。
3. Prompt 没有强约束模型基于证据回答。
4. 模型过度发挥或错误归纳。
5. 多轮上下文污染。

所以 GrayDraft 不只做 RAG，还做 Smart Import 质量门禁、混合检索、Rerank、A/B/C/D 分级、citations、答案自检和反馈闭环。

---

## Q17：标准 RAG 和生产级 RAG 有什么区别？

**回答**：

标准 RAG 常见是：切块、向量化、向量检索、拼 Prompt、生成。生产级 RAG 还要考虑：

- 多格式文档解析和智能入库。
- 去重、质量门禁、版本管理。
- 混合检索和重排。
- 检索失败降级。
- 流式输出和答案缓存。
- 权限、租户、工作区隔离。
- 观测指标、trace、反馈闭环。
- 模型网关和成本控制。

GrayDraft 的价值就是把这些生产问题补齐。

---

## Q18：RAG 的分块策略怎么设计？

**回答**：

分块要在语义完整性和检索粒度之间平衡。

常见策略：

1. 固定窗口：简单，但可能切断语义。
2. 标题 / 段落感知：更符合文档结构。
3. 表格 / 多模态优先：避免把事实表切坏。
4. overlap：保留上下文，但会增加存储和召回重复。
5. Smart Import 多段拆分：先识别结构，再入库。

GrayDraft 的思路是先 Smart Import 做结构理解和质量判断，再由 Worker 做分块和向量化。

---

## Q19：chunk size 怎么选？

**回答**：

chunk 太小，语义不完整，模型拿到碎片难以回答；chunk 太大，检索不精准，也会浪费上下文窗口。

一般要根据文档类型调参：

- FAQ / 短知识：小 chunk。
- 教程 / 说明文：中等 chunk，保留段落。
- 表格 / 配置：尽量结构化，不要硬切。

工程上可以通过召回准确率、Rerank 分数、answer_confidence、人工评测集来调优。

---

## Q20：Embedding 在 RAG 中的作用是什么？

**回答**：

Embedding 把文本转成向量，让系统能按语义相似度检索，而不是只靠关键词匹配。它适合同义表达、概念性问题、跨语言或模糊查询。

但 Embedding 对专名、数字、版本号、短实体不一定稳定，所以 GrayDraft 不是只用向量，而是结合全文检索、structured 向量和 Query4D 实体约束。

---

## Q21：为什么需要 Rerank？

**回答**：

初召回通常追求 recall，会取比较多候选，但排序不一定准。Rerank 用更强的相关性判断模型或规则，对候选进行精排，让最终注入 Prompt 的证据更可靠。

GrayDraft 使用两阶段 Rerank：先轻量 L1 筛选，再 L2 Gateway 精排；如果 Gateway 不可用，降级到词法排序，保证链路不断。

---

## Q22：RAG 中 citations 有什么价值？

**回答**：

citations 是答案引用的证据来源，价值有三点：

1. **可信度**：用户知道答案依据哪里来。
2. **可追溯**：排查错误时能定位到 chunk / 文档。
3. **反馈治理**：引用次数可以进入 knowledge_score，帮助评估知识价值。

在 GrayDraft 里，citations 和 retrieval_trace、answer_confidence 一起构成可解释 RAG 的基础。

---

# 四、检索算法篇

## Q23：为什么不用单纯向量检索？

**回答**：

单纯向量检索有几个问题：

- 对专名、版本号、数字、道具名不稳定。
- 容易召回语义相似但实体错误的内容。
- 无法很好表达结构化过滤条件。
- 分数不一定可解释。

所以 GrayDraft 使用混合检索：全文召回处理关键词和实体，pgvector 处理语义相似，structured 向量处理结构化语义，再用 RRF 和 Rerank 融合。

---

## Q24：混合检索怎么做？

**回答**：

GrayDraft 的混合检索大致是：

```text
query → 意图分类 → Query4D → 全文召回 + pgvector 召回 + structured 召回 → RRF 融合 → 4D 瀑布退避 → Rerank → A/B/C/D 分级
```

这样既能处理精确实体，也能处理语义相似，还能通过分级门控决定是否注入证据。

---

## Q25：什么是 RRF？为什么用它？

**回答**：

RRF 是 Reciprocal Rank Fusion，用 rank 名次融合多个召回结果，而不是直接相加原始分数。公式直觉是：

```text
score(doc) = Σ 1 / (k + rank_i(doc))
```

它的好处是不同检索路的分数不需要同量纲。全文检索、向量检索、structured 向量的分数范围不同，直接相加不合理，用 RRF 更稳。

---

## Q26：Query4D 是什么？

**回答**：

Query4D 是 GrayDraft 的四维语义提取机制，从用户 query 中提取：

- `domain`：领域。
- `scene`：场景。
- `style`：风格。
- `entities`：实体。

它的作用是把模糊 query 转成结构化语义约束，驱动 SQL 过滤、软加分和检索 trace。比如教程问题里，entities 可以是模型名、API 名和参数名。

---

## Q27：4D 瀑布退避解决什么问题？

**回答**：

如果 4D 过滤太严格，可能零召回；如果完全不做过滤，又可能召回很多噪声。4D 瀑布退避就是从严格到宽松逐级放宽：

1. Tier 1：domain + entities + scene + style 全量硬过滤。
2. Tier 2-4：逐步放宽 scene / style / entities。
3. Tier 5：不做硬过滤，改用 4D 对齐软加分。

这样既利用结构化语义，又保证最终不会轻易零召回。

---

## Q28：什么是 A/B/C/D 检索分级？

**回答**：

A/B/C/D 分级是对检索 evidence 质量的门控：

| 等级 | 含义 | 策略 |
|------|------|------|
| A | 高质量 grounding，实体匹配强 | 强注入，正常回答 |
| B | 有直接证据或标题命中 | 正常 RAG |
| C | 证据较弱但可用 | 稀疏注入，提示不确定 |
| D | 无可靠参考 | 降级通用 LLM 或固定话术 |

它比简单 useful / not useful 更细，可以根据 evidence 质量动态决定回答策略。

---

## Q29：Calibrator 有什么作用？

**回答**：

不同知识库、不同 `kb_type` 的分数分布可能不一样。如果全局用固定阈值，某些工作区会过严，某些会过松。Calibrator 通过历史 quality_score 分布学习 P20 / P60 等分位，用分区校准替代硬编码阈值。

这体现了生产级 RAG 的一个重点：阈值不是拍脑袋，而要结合数据分布调整。

---

## Q30：如何评估检索质量？

**回答**：

可以从离线和在线两类评估：

离线：

- Recall@K：正确证据是否在 Top K。
- MRR / NDCG：排序质量。
- Rerank 命中率。
- 标注集人工评测。

在线：

- citations 点击 / 引用。
- 用户点赞点踩。
- answer_confidence。
- 搜索后追问率。
- fallback 比例。
- 无结果率。

GrayDraft 可以结合 retrieval_trace、citations、knowledge_score、Feedback 做质量闭环。

---

# 五、Smart Import 与入库篇

## Q31：为什么需要 Smart Import？

**回答**：

普通入库是文档直接切块向量化，但这样容易把重复、低质量、结构混乱的内容放进知识库。RAG 的质量很大程度取决于入库质量，所以 GrayDraft 做 Smart Import：先预览和分析，再决定是否提交入库。

它解决的是「垃圾进，垃圾出」的问题。

---

## Q32：Smart Import 的流程是什么？

**回答**：

大致流程是：

```text
输入内容 / 附件 → 结构解析 → 标题推断 → 规则 + LLM 拆分 → kb_type 分析 → 去重 → 置信度五检 → 预览 / Warning / Reject → 提交 → JetStream → Worker → 分块 / Embedding / 写库
```

低置信内容不会直接入库，而是返回预览和原因，避免污染知识库。

---

## Q33：置信度五检包括什么？

**回答**：

可以概括为五类可解释规则：

1. 结构骨架：是否有标题或清晰层级。
2. 实体密度：是否有足够信息量。
3. 噪声重复：ngram 重复率是否过高。
4. 标题校验：标题和正文是否匹配。
5. 实体溯源：抽取出的实体是否能在正文追溯。

它不是黑盒分数，而是可解释门禁，方便前端展示和人工确认。

---

## Q34：content_hash 去重有什么意义？

**回答**：

content_hash 用于精确去重，避免同一内容重复入库。意义包括：

- 保证幂等。
- 降低向量化和存储成本。
- 避免重复 chunk 干扰检索排序。
- 便于版本管理和更新。

在异步入库系统里，幂等尤其重要，因为任务可能重试。

---

## Q35：pHash 近重复复用解决什么问题？

**回答**：

pHash 是感知哈希，主要用于图片近重复检测。多模态内容如果每次都调用 VLM / OCR，成本很高。pHash 可以判断图片是否和已有图片高度相似，如果命中，就复用已有识别结果，减少模型调用。

这是多模态 RAG 的典型降本手段。

---

## Q36：为什么入库要用 JetStream 异步？

**回答**：

入库包含解析、分块、Embedding、多模态、数据库写入，耗时长且依赖外部模型。如果同步处理，容易接口超时，也不方便失败重试。

JetStream 的价值：

- 削峰填谷。
- Worker 异步消费。
- 失败重试。
- DLQ 死信队列。
- 任务可重放。
- Core 和 Worker 解耦。

---

## Q37：DLQ 怎么用？

**回答**：

DLQ 是 Dead Letter Queue。任务多次失败后进入 DLQ，运维人员可以查看失败原因，例如模型超时、数据格式错误、数据库约束错误。修复问题后，可以重放任务。

DLQ 的关键是保留足够上下文：task_id、trace_id、payload、错误类型、失败阶段、重试次数。

---

# 六、SSE RAG 与对话篇

## Q38：为什么用 SSE 做流式对话？

**回答**：

LLM 生成是典型服务端单向持续输出，SSE 比 WebSocket 更简单：

- 基于 HTTP，代理友好。
- 前端 EventSource / fetch stream 接入简单。
- 天然适合 token chunk。
- 对断线、错误事件处理清晰。

WebSocket 更适合双向高频实时通信；SSE 更适合 RAG 对话这种请求后服务端持续输出。

---

## Q39：SSE RAG 的事件应该怎么设计？

**回答**：

不能只返回 token chunk，还要返回过程信息。GrayDraft 可以设计：

| 事件 | 作用 |
|------|------|
| retrieval_grade | 告诉前端证据质量 |
| thinking | 展示正在检索 / 生成 |
| meta | 返回 citations、trace、模式等元信息 |
| chunk | 模型生成内容 |
| done | 完成，带 answer_confidence |
| error | 错误或降级原因 |

这样前端能展示进度，后端也保留可观测信息。

---

## Q40：答案缓存怎么做？

**回答**：

答案缓存可以按 query、workspace、chat_mode、prompt 版本、知识库版本等构造 key。命中后不需要重新检索和生成，而是直接回放 SSE 事件：meta → chunk → done。

关键注意：

- 缓存必须感知知识库版本，否则知识更新后可能返回旧答案。
- 对个性化 / 权限相关问题要谨慎缓存。
- 可以区分精确缓存和语义相似缓存。

---

## Q41：hybrid 和 kb_only 有什么区别？

**回答**：

- `kb_only`：要求答案严格基于知识库，证据不足时应拒答或提示不足。
- `hybrid`：优先用知识库，但证据不足时可以结合模型通识或降级回答。

在教程资料这类需要依据社区内容的问题，可以更偏 `kb_only`；在开放建议类问题，可以用 `hybrid`。

---

## Q42：Gateway 不可用时对话怎么办？

**回答**：

要按能力分级降级：

- Chat LLM 不可用：走 `search_only_fallback`，把检索证据整理成简明答案。
- Rerank 不可用：降级本地词法排序。
- Embedding 不可用：新入库任务重试或进 DLQ；已有索引仍可检索。
- 单 Provider 不可用：Gateway 切换其他 Provider 或熔断。

核心原则是：模型不可用不应让整个业务完全不可用。

---

## Q43：如何做答案自检？

**回答**：

答案自检可以从几方面做：

1. 答案是否覆盖用户问题。
2. 答案中的关键实体是否出现在 citations。
3. 是否出现证据中没有的数字、结论或承诺。
4. 检索分级是否足够支撑回答。
5. 输出 answer_confidence 供 UI 展示和反馈。

更高级可以用 LLM-as-judge，但生产上要注意成本和延迟。

---

# 七、Tool Use / Skill 篇

## Q44：Tool Use 的完整链路是什么？

**回答**：

Tool Use 通常包括：

```text
注册工具 schema → 模型选择 tool_call → 宿主执行工具 → 返回 observation → 模型继续推理或输出最终答案
```

工具 schema 要包含 name、description、parameters、返回格式、错误语义。执行工具时必须在服务端做权限和参数校验，不能完全相信模型。

---

## Q45：Skill 和 Tool 有什么区别？

**回答**：

Tool 更像一个可调用函数或 API；Skill 更像一个能力包，包含工具、流程、领域知识、触发条件和输出规范。

在 GrayDraft 里，检索和 `note.draft` 都是工具。教育助手按 `edu_workspace_v1` 选择词法、语义、阅读、扩展或笔记。

---

## Q46：GrayDraft 的 14 Skill 怎么讲？

**回答**：

不需要逐个背，按类型讲：

1. 上下文：当前学员和授权分区。
2. 资料查询：教程、课件、论文摘录、笔记。
3. 业务查询类：任务、订单、支付等。
4. 外部能力类：天气、提醒。
5. 知识问答类：调用 GrayDraft RAG。

重点是说明每个 Skill 都有触发条件、参数 schema、权限边界、失败降级和 observation 格式。

---

## Q47：如何避免模型误调用工具？

**回答**：

可以做多层控制：

- 工具描述写清楚适用范围。
- RouterModel 先做意图分类。
- 只暴露当前用户有权限的工具。
- 参数用 JSON Schema 限制。
- 执行前服务端二次校验。
- 敏感操作需要用户确认。
- 工具失败返回结构化错误，避免模型胡编。

---

## Q48：Tool Use 如何处理权限？

**回答**：

权限不能交给模型决定，必须由业务服务控制。做法是：

1. 根据用户身份生成可用工具列表。
2. 工具执行时带 user_id / tenant / workspace。
3. 后端 API 再做权限校验。
4. 返回结果做脱敏。
5. 敏感操作写审计日志。

比如订单查询 Skill 只能查当前用户订单，不能让模型传任意 user_id。

---

## Q49：工具调用失败怎么办？

**回答**：

工具失败时不要直接让模型猜。应该返回结构化 observation：

```json
{
  "ok": false,
  "error_code": "TIMEOUT",
  "message": "订单服务暂时不可用",
  "retryable": true
}
```

然后 Agent 可以选择重试、换工具、降级回复或提示用户稍后再试。

---

# 八、Memory 篇

## Q50：Agent Memory 怎么分类？

**回答**：

可以分三类：

1. **短期记忆**：当前会话上下文、中间步骤、工具 observation。
2. **长期记忆**：文档库、向量库、用户历史偏好、知识库。
3. **会话状态**：授权分区、当前问题和已批准笔记。

GrayDraft 主要做长期知识记忆；GrayDraft 负责短期 session 和业务状态。

---

## Q51：短期记忆怎么控制上下文长度？

**回答**：

方法包括：

- 保留最近 N 轮。
- 对历史对话做摘要。
- 只保留关键工具结果。
- 对 observation 做结构化压缩。
- 用检索方式召回历史记忆。
- 设置最大 token 和最大步数。

不要把所有历史无脑塞进 Prompt，否则成本高、噪声大、容易污染判断。

---

## Q52：长期记忆如何更新？

**回答**：

长期记忆更新要有治理流程：

1. 内容进入 Smart Import。
2. 做去重和质量门禁。
3. 分块、向量化、写入文档库。
4. 更新索引版本和缓存失效。
5. 根据用户反馈调整 knowledge_score。
6. 需要时做 taxonomy 进化和影子回填。

不能让模型随意把任何对话写入长期记忆，否则会污染知识库。

---

## Q53：Memory 和 RAG 的区别是什么？

**回答**：

RAG 是长期记忆的一种实现，但 Memory 更广。Memory 包括会话上下文、用户偏好、业务状态、中间工具结果和长期知识。RAG 主要解决「从外部知识库取回相关证据」的问题。

所以可以说：

> RAG 是 Agent 长期知识记忆的核心技术之一，但 Agent Memory 不等于 RAG。

---

# 九、模型网关与多 Provider 篇

## Q54：为什么要做模型网关 Gateway？

**回答**：

Gateway 的作用是隔离业务系统和模型供应商。它统一处理：

- Chat / Embedding / Rerank / VLM 调用协议。
- Provider 鉴权和模型名称映射。
- 超时、重试、熔断。
- 流式 SSE 代理。
- 多 Provider 路由。
- 审计和成本统计。

没有 Gateway，Core 和 Worker 会直接耦合不同模型厂商，后期维护成本很高。

---

## Q55：多 Provider 路由怎么设计？

**回答**：

可以按多种策略：

- 按任务类型：Chat、Embedding、Rerank 分别走不同模型。
- 按质量等级：fast / pro 模型分层。
- 按成本：简单问题走便宜模型，复杂问题走强模型。
- 按可用性：主 Provider 失败后切备 Provider。
- 按租户配置：不同 workspace 使用不同模型。

路由策略最好配置化，而不是写死在业务代码里。

---

## Q56：什么是熔断？模型调用为什么需要熔断？

**回答**：

熔断是当某个 Provider 连续失败或超时时，短时间内停止继续调用它，避免请求堆积和雪崩。

模型调用依赖网络和外部服务，可能出现限流、超时、服务不可用。如果不熔断，所有请求都会卡住，影响 Core 和 Worker。Gateway 层熔断后，可以快速失败、切换 Provider 或触发降级。

---

## Q57：Embedding 微批有什么价值？

**回答**：

Embedding 通常可以批量调用。微批把短时间内的多条文本合并成一次请求，价值是：

- 降低网络请求次数。
- 提高吞吐。
- 减少 Provider 限流风险。
- 降低单位成本。

但 batch 太大也会增加等待时间，所以要在吞吐和延迟之间平衡。

---

# 十、缓存与性能篇

## Q58：GrayDraft 缓存体系怎么设计？

**回答**：

可以分三层：

1. **L1 Ristretto**：进程内缓存，速度快，适合热点检索结果。
2. **L2 Redis**：跨进程缓存，适合集群共享。
3. **答案缓存 / 语义缓存**：缓存完整回答或相似 query 结果。

缓存 key 要包含 workspace、query、模式、知识库版本、Prompt 版本等，避免脏读。

---

## Q59：语义缓存和精确缓存有什么区别？

**回答**：

- 精确缓存：query 完全一致才命中，准确但命中率低。
- 语义缓存：query 语义相似也命中，命中率高但有误命中风险。

语义缓存要设置相似度阈值，并结合 workspace、权限和知识库版本，不能跨业务上下文乱命中。

---

## Q60：缓存失效怎么做？

**回答**：

常见策略：

- TTL 过期。
- 知识库版本号变化时失效。
- 文档更新 / 删除时 purge 相关 key。
- Prompt 版本变化时失效。
- workspace 级别批量失效。

RAG 缓存一定要版本感知，否则知识更新后用户还会拿到旧答案。

---

## Q61：如何优化 RAG 延迟？

**回答**：

可以分阶段优化：

- 检索前：缓存 query 分析结果。
- 检索中：全文和向量并行，限制 top_k。
- Rerank：两阶段重排，必要时跳过重排。
- 生成前：答案缓存命中直接回放。
- 生成中：SSE 流式降低感知延迟。
- 模型层：fast / pro 路由，超时降级。
- 入库侧：Embedding 微批，异步处理。

---

# 十一、可观测与评估篇

## Q62：AI Agent 系统应该观测哪些指标？

**回答**：

至少包括：

| 类型 | 指标 |
|------|------|
| 延迟 | 检索耗时、Rerank 耗时、首 token 时间、总生成时间 |
| 质量 | 检索分级、answer_confidence、citations 数量、反馈分 |
| 成本 | token 数、模型调用次数、Embedding 批量大小 |
| 稳定性 | 错误率、超时率、熔断次数、DLQ 数量 |
| 缓存 | L1 / L2 / 答案缓存命中率 |
| 业务 | 提问次数、追问率、点赞点踩、转化 |

---

## Q63：trace_id 有什么作用？

**回答**：

trace_id 用于串联一次请求的完整链路：

```text
前端 → BFF → Core → Retriever → Gateway → Worker / DB → SSE 输出
```

当回答错误或延迟高时，可以通过 trace_id 找到具体卡在哪一步，是检索没命中、Rerank 失败、Gateway 超时，还是缓存失效。

---

## Q64：如何做 Agent 质量评估？

**回答**：

可以结合自动评估和人工反馈：

1. 构建固定问题集和标准答案。
2. 评估检索 Recall@K 和 citations 正确率。
3. 用 LLM-as-judge 初筛答案相关性和 groundedness。
4. 人工抽样检查高风险问题。
5. 收集线上点赞点踩和追问。
6. 把反馈反哺 knowledge_score 和检索调参。

生产中不能只看模型输出是否流畅，更要看 evidence 是否正确。

---

## Q65：线上出现错误答案怎么排查？

**回答**：

按链路排查：

1. 看用户 query 和 workspace 是否正确。
2. 看 Query4D 和意图分类是否正确。
3. 看检索 top chunks 是否包含正确证据。
4. 看 RRF / Rerank 是否把正确证据排上来。
5. 看 A/B/C/D 分级是否误判。
6. 看 Prompt 是否正确约束模型。
7. 看模型是否编造。
8. 看 citations 和 answer_confidence。

如果证据没召回，是检索问题；如果证据正确但答案错，是生成或 Prompt 问题。

---

# 十二、安全与权限篇

## Q66：AI Agent 的安全风险有哪些？

**回答**：

主要包括：

- Prompt injection。
- 工具越权调用。
- 敏感数据泄露。
- 用户输入恶意内容。
- 模型编造操作结果。
- 跨租户 / 跨 workspace 数据泄露。
- 长期记忆污染。

治理方式包括权限过滤、服务端鉴权、工具白名单、输出脱敏、操作确认、审计日志和知识入库门禁。

---

## Q67：怎么防 Prompt Injection？

**回答**：

Prompt Injection 不能只靠 Prompt 防，必须多层治理：

1. 检索内容和用户指令分区放入 Prompt。
2. 明确系统指令优先级。
3. 对工具调用做服务端权限校验。
4. 不允许知识库内容覆盖系统指令。
5. 高风险操作二次确认。
6. 对外部文档做入库清洗和标记。

关键是：模型看到恶意文本没关系，不能让恶意文本获得系统权限。

---

## Q68：多租户 / workspace 隔离怎么做？

**回答**：

隔离要贯穿：

- API 请求带 tenant / workspace。
- 数据库查询强制过滤 workspace。
- 向量检索带 namespace / partition。
- 缓存 key 包含 workspace。
- Prompt 只注入当前 workspace 的 context。
- 工具列表按权限生成。
- 日志和审计带 scope。

GrayDraft 用 `tutorial` / `paper` / `note` 隔离资料，就是一种场景分区。

---

# 十三、架构设计篇

## Q69：设计一个企业知识库 Agent，你怎么做？

**回答结构**：

1. **入口层**：Web / IM / API。
2. **Agent Router**：识别意图，是问知识、查业务数据，还是执行操作。
3. **RAG 中台**：文档解析、分块、向量、混合检索、Rerank。
4. **Tool 层**：CRM、订单、工单、数据库等 API。
5. **模型网关**：多 Provider、熔断、成本控制。
6. **Memory**：会话上下文、长期知识、用户偏好。
7. **安全**：权限、审计、脱敏、Prompt injection 防护。
8. **评估**：离线测试集、线上反馈、trace。

可以类比 GrayDraft + GrayDraft 的分层。

---

## Q70：设计一个带出处的资料工作台，你怎么做？

**回答**：

我会按 GrayDraft 的方式设计：

```text
学员提问 → 确认授权分区 → Planner 选择教育工具 → 检索或写笔记 → SSE 返回讲解和引用
```

核心点：

- 概念和参数问题绑定 `tutorial`。
- 订单 / 任务问题必须鉴权。
- 文章审核后自动 Smart Import 入库。
- 结果带 citations 和 confidence。
- 模型不可用时降级到 search_only 或业务提示。

---

## Q71：如果让你把 GrayDraft 做成 SaaS，你要补什么？

**回答**：

需要补：

- 多租户隔离和计费。
- API Key 和权限管理。
- 配额、限流、成本统计。
- workspace 管理和数据导入 UI。
- 模型 Provider 配置化。
- 评测集和 RAGOps 面板。
- 数据导出、删除、合规能力。
- SLA 监控和告警。

当前 GrayDraft 已经有中台基础，但 SaaS 化还要强化租户、计费、权限和运营面。

---

## Q72：如果召回质量不好，你会怎么优化？

**回答**：

按顺序排查：

1. 入库质量：chunk 是否合理，内容是否缺失。
2. Query 理解：意图分类和 Query4D 是否错。
3. 召回策略：全文、向量、structured 哪一路失效。
4. 融合排序：RRF 权重、Rerank 是否合理。
5. 阈值门控：A/B/C/D 是否过严。
6. Prompt：是否正确使用证据。
7. 数据反馈：根据失败 query 建评测集持续优化。

不要一上来就换模型，很多问题其实是数据和检索链路问题。

---

## Q73：如果用户觉得回答慢，你怎么优化？

**回答**：

先通过 trace_id 定位慢在哪：

- 检索慢：并行召回、索引优化、减少 top_k。
- Rerank 慢：两阶段筛选、阈值跳过、缓存。
- LLM 慢：fast 模型、流式输出、限制 max_tokens。
- Embedding 慢：异步和微批。
- 重复问题：答案缓存和语义缓存。

同时用 SSE 降低感知延迟，让用户尽早看到进度和首 token。

---

# 十四、工程实现篇

## Q74：Go 和 Python 在项目中如何分工？

**回答**：

Go 适合做高并发 API、服务治理、数据库访问和业务控制；Python 适合做模型周边、文本处理、Embedding / Rerank / 多模态处理。

GrayDraft 中可以用 Go 做 Core / Gateway 主链路，用 Python Worker 处理 AI 相关异步任务，通过 JetStream 解耦。这样既利用 Go 的工程稳定性，也利用 Python AI 生态。

---

## Q75：为什么选 PostgreSQL + pgvector？

**回答**：

pgvector 的优势是把结构化数据、文档元数据和向量放在同一个 PostgreSQL 体系里，适合中小规模到中等规模的知识库，运维简单，事务和 SQL 能力强。

如果未来规模更大，可以考虑专用向量库，但早期用 pgvector 能降低架构复杂度，并方便和业务字段做联合过滤。

---

## Q76：Redis 在项目里做什么？

**回答**：

Redis 可以做：

- L2 缓存。
- 语义缓存索引或元数据。
- 会话状态。
- 限流计数。
- 业务实时状态。
- GrayDraft 中的排序、离线推送、临时状态。

它和 Ristretto 的区别是 Redis 跨进程共享，Ristretto 是进程内高速缓存。

---

## Q77：前端如何展示 Agent 过程？

**回答**：

前端不应该只等最终答案，而要展示过程状态：

- 正在理解问题。
- 正在检索知识库。
- 检索到多少条证据。
- 正在生成答案。
- 显示 citations。
- 显示低置信提示。
- 失败时展示降级原因。

GrayDraft 的 SSE 事件设计支持这种体验；GrayDraft 前端可以根据事件逐步更新 UI。

---

# 十五、行为面与项目复盘篇

## Q78：项目中最难的问题是什么？

**回答示例**：

最难的是把 RAG 从 Demo 做成稳定系统。Demo 只要能问答即可，但生产中会遇到低质量入库、检索召回不准、模型超时、成本高、缓存脏读、前端等待无反馈等问题。

我解决思路是分层治理：入库前 Smart Import 质量门禁；检索中混合召回和 Rerank；生成前分级门控；模型层 Gateway 熔断降级；体验层 SSE；运维层 trace_id、Prometheus、DLQ。

---

## Q79：你做过哪些取舍？

**回答示例**：

一个典型取舍是 pgvector vs 专用向量库。专用向量库在大规模场景性能更强，但项目初期需要快速落地，且要和文档元数据、workspace、kb_type 做复杂过滤，所以选择 PostgreSQL + pgvector，减少系统数量和运维复杂度。

另一个取舍是 SSE vs WebSocket。LLM 对话是服务端单向流，SSE 足够简单；实时业务状态才用 WebSocket。

---

## Q80：如果重做这个项目，你会优化什么？

**回答示例**：

我会优先加强三块：

1. **评测体系**：建立固定 query set，自动评估 Recall@K、groundedness、answer correctness。
2. **RAGOps 面板**：可视化每次请求的 Query4D、召回、Rerank、Prompt、SSE 事件。
3. **Tool Use 标准化**：把 GrayDraft Skill 的 schema、权限、错误格式抽成统一协议，便于扩展更多业务工具。

---

## Q81：你如何学习 AI Agent？

**回答**：

我的学习方式是项目驱动：先通过 GrayDraft 跑通 RAG、检索、模型网关、SSE，再对照 Agent Loop、Tool Use、Memory、MCP / Skills 等通用概念补理论。每学一个概念，我都会问它在项目里对应什么模块、解决什么工程问题、有什么风险。

这种方式的优势是不会停留在概念层，而是能落到 API、队列、缓存、数据库和前端体验。

---

# 十六、薪资和岗位匹配篇

## Q82：你为什么适合 AI Agent 应用研发岗位？

**回答**：

因为我既有 AI 应用链路经验，也有全栈工程落地能力。AI 侧我做过 RAG、混合检索、Tool Use、Memory、模型网关、SSE 流式对话；工程侧我能处理 Go 后端、Python Worker、Vue 前端、PostgreSQL / Redis、JetStream、Docker 部署和可观测。

AI Agent 应用研发需要的不是只会模型调用，而是把模型能力接入真实业务并保证稳定运行，这正好和我的项目经验匹配。

---

## Q83：你和算法工程师有什么区别？

**回答**：

算法工程师更偏模型训练、微调、算法研究和评估；我更偏 AI 应用工程化和平台落地。我的重点是如何把 LLM、Embedding、Rerank 等模型能力接到业务系统里，并处理检索质量、工具调用、权限、缓存、降级、部署、观测等问题。

我能和算法工程师协作，把模型能力产品化、工程化、稳定化。

---

## Q84：你和普通后端工程师有什么区别？

**回答**：

普通后端更关注 CRUD、业务流程和系统稳定性；AI Agent 后端还要理解模型调用、Prompt、RAG、向量检索、Tool Use、Memory、流式输出和模型不确定性。

我的优势是既有后端工程能力，也理解 AI 链路的特殊问题，比如 hallucination、grounding、Rerank、语义缓存、模型熔断和 Agent 评估。

---

# 十七、追问压测篇

## Q85：如果面试官说「你这个只是 RAG，不是 Agent」，怎么回应？

**回答**：

可以承认并澄清边界：

> 如果只看早期 v1 Chat，GrayDraft 确实更像 RAG 管线；但 **当前 Core 已具备 Agent Runtime v2**（Run + Loop + Memory + Tool Governance）。完整业务 Agent 仍是 GrayDraft 产品层的 Skill Router + 业务工具 + GrayDraft 知识能力组合。我的设计是把通用 Agent 内核与知识治理中台化，产品层只负责意图与用户上下文。

这样回答比硬说自己是 Agent 更专业。

---

## Q86：如果面试官问「没有 LangChain / LangGraph 经验怎么办」？

**回答**：

可以回答：

> 我了解 LangChain / LangGraph 的核心思想：工具抽象、链式调用、状态图和 Agent Loop。我的项目没有强依赖这些框架，而是根据业务自建了 RAG、Gateway、Skill 和工作流能力。框架可以提高开发速度，但生产系统仍然要处理权限、缓存、降级、观测和业务集成。我能快速上手框架，因为底层概念和工程问题是相通的。

---

## Q87：如果面试官问「你的 Query4D 是不是过度设计」？

**回答**：

可以说：

> 如果是通用小型 FAQ，Query4D 可能过重。但在书生教程里，问题经常包含模型名、API 名、参数和章节。单纯向量检索容易召回语义相似但实体错误的内容，所以需要结构化语义约束。为了避免过严，我又做了 4D 瀑布退避和软加分，不是简单硬过滤。

---

## Q88：如果面试官问「RRF 是不是很普通，亮点在哪里」？

**回答**：

可以说：

> RRF 本身是标准算法，我不会把它说成自研。项目亮点在于如何把 RRF 放进完整检索链路：它融合全文、向量和 structured 三路召回；上游有 Query4D 语义约束，下游有 4D 瀑布、Rerank 和 A/B/C/D 分级。生产价值不是单个算法有多新，而是组合后解决了专名、召回、排序、门控和降级问题。

---

## Q89：如果面试官问「为什么不用直接微调模型」？

**回答**：

RAG 和微调解决的问题不同：

- RAG 适合频繁变化的事实知识、企业文档、需要引用来源的场景。
- 微调适合稳定风格、格式、特定任务能力。

教程、课件和笔记会不断变化，需要可追溯 citations，所以优先用 RAG。未来如果有稳定的回答风格或分类任务，可以考虑微调小模型做 Router 或分类器。

---

## Q90：如果面试官问「怎么证明你的系统有效」？

**回答**：

可以从四类证据证明：

1. **功能闭环**：资料能入库，学员能按资料提问，答案带引用。
2. **质量指标**：Recall@K、Rerank 命中、A/B/C 分布、answer_confidence。
3. **稳定指标**：错误率、超时率、DLQ、fallback 比例。
4. **业务指标**：提问量、点赞点踩、追问率、内容消费转化。

面试时要说明自己知道如何评估，而不是只说「感觉效果不错」。

---

# 十八、反问面试官

## Q91：可以反问哪些问题？

**推荐反问**：

1. 你们的 Agent 更偏通用助手，还是垂直业务流程自动化？
2. 当前最大瓶颈是检索质量、工具调用、成本，还是稳定性？
3. RAG 是否已有评测集和线上反馈闭环？
4. Tool Use 是否接入真实业务 API？权限和审计怎么做？
5. 模型调用是否有统一 Gateway、多 Provider 和熔断？
6. Memory 是只做会话上下文，还是有长期向量记忆？
7. 团队希望我优先补平台工程、业务集成、前端体验，还是检索优化？

---

# 十九、速背卡片

## Q92：一分钟总结你的 AI Agent 能力

**回答**：

> 我的 AI Agent 能力可以分三层：第一是原理层，包括 Agent Loop、Tool Use、Memory、RAG 和模型路由；第二是落地层，GrayDraft 做导入、混合检索、带引用讲解和笔记；第三是工程层，包括缓存、队列、拒答、权限和可观测。

## Q93：三句话讲 GrayDraft

**回答**：

1. GrayDraft 是面向 Agent 的 RAG 与知识治理中台。
2. 它覆盖 Smart Import 入库、混合检索、SSE RAG、Gateway 模型路由、语义进化和 RAGOps。
3. 它的价值是让上层业务 Agent 获得高质量、可解释、可降级、可观测的长期知识能力。

## Q94：三句话讲 GrayDraft AI

**回答**：

1. GrayDraft 是教育场景入口，提供教程、课件、论文摘录和笔记。
2. 它通过 BFF 和 教育五工具 把 GrayDraft RAG 与社区业务 API 编排成业务 Agent。
3. 它形成导入、带引用讲解和笔记再检索的闭环。

## Q95：三句话讲你的自研亮点

**回答**：

1. 模型不是自研，自研重点在编排、门控和工程化治理。
2. Query4D、4D 瀑布、A/B/C/D 分级、Smart Import 五检和 Blueprint 语义进化是项目的核心自研规则与编排。
3. 这些机制解决的是 RAG 生产环境里的召回、幻觉、低质量入库、降级和可解释问题。

---

## 20. 复习顺序

1. 先背 Q1-Q14，确保项目和 Agent 基础说得清。
2. 再背 Q15-Q43，覆盖 RAG、检索、入库、SSE。
3. 然后背 Q44-Q68，覆盖 Tool、Memory、Gateway、缓存、安全。
4. 面试前重点练 Q69-Q90，训练系统设计和追问压测。
5. 最后用 Q92-Q95 做电梯陈述。
