# 个人简历

## 基本信息

| 项目 | 内容 |
|------|------|
| 姓名 | [你的姓名] |
| 手机 | [手机号] |
| 邮箱 | [邮箱] |
| GitHub | [GitHub 链接] |
| 求职意向 | **AI 应用工程师 / AI Agent 工程师** |
| 到岗时间 | [可填写] |

---

## 个人总结

全栈 AI 工程师，具备 **Go + Python 双语言** 生产级后端与 **大模型 Agent / RAG** 系统从 0 到 1 的落地经验。独立设计并实现 **GrayDraft**：面向教程、论文和课件的资料工作台，覆盖混合检索、带出处的讲解和笔记沉淀。

熟悉 **PostgreSQL/pgvector、MySQL、Redis、NATS JetStream、Docker** 等分布式基础设施，具备多 Provider LLM 路由、语义缓存降本、异步入库流水线等工业级 AI 工程实践。擅长将 **RAG、Agent 编排、工具调用** 产品化落地；垂直领域（如 AI for Science）可作为能力迁移的**加分展示**，非当前主攻方向。

---

## 专业技能

### AI 应用与算法（核心）

| 技能 | 熟练度 | 项目佐证 |
|------|--------|----------|
| **Python**（FastAPI、asyncio、异步流水线） | 熟练 | GrayDraft Worker 入库/Rerank/QA 生成 |
| **PyTorch / 深度学习** | 了解～熟练 | [按你实际课程/项目填写；可写「李沐动手学深度学习 + 图像分类实践」] |
| **大模型应用**（Prompt Engineering、RAG、SSE 流式） | 熟练 | 教育检索 + 书生对话 + 流式讲解 |
| **AI Agent**（Planning、Memory、Tool Use） | 熟练 | `edu_workspace_v1` 五工具：词法、语义、阅读、扩展、笔记 |
| **模型微调**（LoRA 等） | 了解 | [若已实践请补充；否则写「了解 LoRA 原理，计划在领域语料上微调」] |
| **向量检索**（pgvector、RRF 融合、语义缓存） | 熟练 | GrayDraft 三级缓存 + 4D 瀑布检索 |

### 工程与架构（支撑）

| 技能 | 熟练度 | 项目佐证 |
|------|--------|----------|
| **Golang**（Gin、goroutine/channel、并发池） | 熟练 | GrayDraft Core/Gateway/BFF；GrayDraft 全栈后端 |
| **PostgreSQL / MySQL** | 熟练 | GrayDraft PG16+pgvector（22 次迁移）；GrayDraft MySQL 72+ 表 |
| **Redis**（缓存、ZSET 调度、语义相似缓存） | 熟练 | 两项目均有深度使用 |
| **消息队列 / 异步** | 熟练 | NATS JetStream（GrayDraft）；async_tasks + asyncpool（GrayDraft） |
| **Docker / Docker Compose** | 熟练 | 两项目 dev/prod/nas 三套编排 |
| **Kubernetes** | 了解 | [若未上线 K8s，写「了解 Pod/Service/Deployment，计划将 GrayDraft 迁移至 K8s」] |
| **微服务 / 分布式** | 熟练 | GrayDraft 9 服务 Monorepo；读写分离、熔断、DLQ |
| **可观测性** | 熟练 | Prometheus + Grafana、OpenTelemetry、全链路 trace_id |

### 领域与软技能（加分项，按需填写）

- 理解 RAG 在**垂直领域知识库**（行业文档、专业术语、多模态资料）中的工程挑战
- 关注 ArXiv 上 LLM Agent、RAG、Tool Use 相关论文
- [若面试岗位偏 AI for Science：可补充对计算生物学 / 药物研发的了解与学习意愿，勿虚构项目经验]
- [英语读写能力、论文阅读能力等，按实际情况填写]

---

## 项目经历

### 项目一：GrayDraft — Agent 知识库中台（v3.0）

**角色**：核心开发 / 架构设计  
**时间**：[起止年月]  
**技术栈**：Go 1.22 · Python 3.11 · FastAPI · PostgreSQL 16 + pgvector · Redis 7 · NATS JetStream · Gin · Protobuf/gRPC · Docker Compose · Prometheus/Grafana

**项目简介**

面向 Agent 场景的 **PaaS 级 RAG 中台**：接收 UGC 内容异步入库，经解析、分块、向量化后提供自适应混合检索与流式 RAG 对话；支持智能快速导入、多模态 VL 处理、工作区语义蓝图自动进化与 RAGOps 自愈闭环。已对接 GrayDraft Agent 产品与 GrayDraft 教育场景。

**核心职责与成果**

1. **微服务架构设计与落地**  
   设计 9 服务 Monorepo（Core / Worker / Gateway / Console / Ops / GrayDraft BFF 等），Core 经 NATS JetStream 异步解耦 Worker 入库流水线，支持 DLQ 重放与同步降级，完成 22 次 DB Schema 演进与 40+ HTTP API。

2. **自适应混合检索引擎**  
   实现意图分类 → 三级语义缓存（Ristretto L1 / Redis 精确+向量相似 L2 / L3 预测预取）→ 全文 ILIKE + pgvector 并行检索 → RRF 融合 + 双向量加权 → Query4D 五级瀑布退避 → L1/L2 两阶段 Rerank，显著降低 Embedding/LLM 调用成本。

3. **流式 RAG 对话与模型路由**  
   实现 Agent Runtime v2（`POST /api/v2/agent-runs` + SSE），通过 `knowledge.semantic` 复用混合检索；Gateway 统一路由 DashScope Embedding、DeepSeek LLM、火山方舟 VL 等多 Provider，含熔断、微批 Embedding、AnswerGate 与 `gate_score` 置信度门槛。

4. **智能导入与多模态处理**  
   构建规则 + LLM 混合导入流水线（MinHash 去重、TF-IDF 信息密度、骨架扫描、LLM 拆分 2.0），多模态 4.0 支持 VL 分流、pHash 近重复复用、1024 维双向量索引；Blueprint Agent 自动进化工作区 4D 语义 taxonomy。

5. **RAGOps 自愈与运维体系**  
   点踩 strike 触发 heal job（rechunk/demote），Ops 服务定时批量修复；Console JWT 管理面 + DLQ replay；Prometheus 10+ 指标与 Grafana 仪表盘。

**与目标岗位的关联**

- RAG 知识库工程可迁移至任意垂直领域（企业知识库、客服、科研文献等）
- Agent 编排（Blueprint、Query4D、工作区 Prompt 叠加）对应 **Planning、Memory、Tool Use**
- 多模态 VL 处理可扩展至图表、扫描件、截图等非纯文本场景

---

### 项目二：GrayDraft — 资料工作台

**角色**：Agent / RAG 工程  
**时间**：2026-10 起  
**技术栈**：Go · Python · PostgreSQL/pgvector · Redis · NATS JetStream · Vue 3 · 书生对话模型 · InternVL

**项目简介**

面向教程、论文、接口说明和课件的工作台。导入后按 `edu_workspace_v1` 多轮取证，给出带原文引用的讲解，并把结论写成可再检索的笔记。

**核心职责与成果**

1. **感知**  
   Markdown、网页摘录和课件截图经智能导入与 InternVL 进入 `tutorial` / `paper`。

2. **决策**  
   Planner 在词法、语义、相邻章节和实体边之间选择工具。证据不足时拒答或改写，不重复同一调用。

3. **执行**  
   回答必须落到原文切片。`note.draft` 经审批后入库，类型为 `note`。

**与目标岗位的关联**

- 查找、阅读和笔记写在同一条资料闭环里
- 工具调用、引用和拒答可以回放

---

## 教育背景

| 学校 | 专业 | 学历 | 时间 |
|------|------|------|------|
| [学校名称] | [计算机/生物信息/相关专业] | [本科/硕士] | [起止年月] |

**相关课程 / 自学**（按实际情况勾选填写）

- 机器学习（吴恩达）、深度学习（李沐《动手学深度学习》）
- 数据结构与算法
- [生物化学 / 分子生物学 / 化学信息学 等，如有]

---

## 学习与职业规划（面试可谈）

结合目标岗位学习路径，当前能力与规划如下：

> 完整 12 周路径见 [../05-practice/learning-roadmap-ai-agent.md](../05-practice/learning-roadmap-ai-agent.md) · BFF 桥接层见 [bff-proxy-capabilities.md](../04-architecture/06-bff-proxy-capabilities.md)

| 阶段 | 岗位要求 | 当前状态 | 下一步 |
|------|----------|----------|--------|
| **第一阶段** | Python/Go 精通、算法、PyTorch 基础 | Go/Python 生产实践已具备；算法持续刷题 | 深化 PyTorch / 微调实践（按需） |
| **第二阶段** | LLM/RAG/Agent、服务化部署 | **已落地** GrayDraft RAG + GrayDraft Agent | 完成 roadmap 1–2 项进阶；整理 Demo 作品集 |
| **第三阶段** | K8s、分布式、垂直领域（加分） | 微服务/Docker**已具备**；K8s 了解中 | K8s 部署 GrayDraft；**可选**垂直领域小 Demo |

### 短期目标项目（建议面试前完成）

1. **必做**：能演示 GrayDraft 全链路（Smart Import → 检索 → 流式 RAG）+ GrayDraft Agent 问答  
2. **选做**：完成 [roadmap](../../roadmap.md) 中 1 项 P1/P2 任务（如检索可视化、答案缓存语义命中）
3. **加分**：选一个垂直领域做 1–2 周小 Demo（药物研发 / 法务 / 客服等均可）

---

## 附录：岗位学习路径对照

> 完整 GrayDraft 驱动学习路径见 [../05-practice/learning-roadmap-ai-agent.md](../05-practice/learning-roadmap-ai-agent.md)。
> 以下为目标岗位（**AI Agent / AI 应用开发**）的核心要求摘要，供面试准备参考。

### 岗位要求核心提炼

| 层次 | 要求 |
|------|------|
| **AI 应用与 Agent**（核心） | 精通 Python 及后端框架，熟悉大模型应用；有 RAG、Agent、Tool Use 落地经验 |
| **工程与架构**（支撑） | 掌握 Golang、PostgreSQL/MySQL、Redis、Docker；有分布式、微服务经验 |
| **垂直领域**（加分项） | 能将通用 AI 能力迁移到具体行业；AI for Science / 药物研发等**非必需** |

### 分阶段学习路径

**第一阶段：打好 AI 与工程的地基（3–6 个月）**

- Python 高级特性、asyncio；Golang 并发模型（goroutine、channel）
- 算法与数据结构：数组、链表、哈希表、树、图；排序、搜索、动态规划
- AI 理论基础：机器学习、深度学习；PyTorch 张量操作、自动求导、模型训练

**第二阶段：专攻 AI 工程与 Agent 开发（6–9 个月）**

- 大模型应用：Prompt Engineering、RAG、LoRA 微调
- AI Agent：Planning、Memory、Tool Use；LangChain / AutoGen 等框架
- 后端服务化：Flask / FastAPI RESTful API；Docker 容器化部署

**第三阶段：深入架构与可选领域（长期积累）**

- Kubernetes 容器编排；微服务架构、gRPC
- PostgreSQL / MySQL 事务与索引优化；Redis 缓存与消息队列
- **可选**：垂直领域 Demo（AI for Science、行业知识库等）

### 补充建议

- **以项目驱动学习**：每阶段完成可演示的 Agent / RAG 功能，而非空读理论
- **关注前沿**：定期阅读 ArXiv 上 LLM、AI Agent、RAG 论文
- **准备面试**：准备 2–3 个「从问题到落地」的故事（检索降本、Agent 工具链、流式 RAG 排障）

---

## 撰写说明

1. **占位符**：`[你的姓名]`、`[起止年月]` 等请替换为真实信息。
2. **诚实原则**：未做过的技能（如 K8s 生产、LoRA 微调）标为「了解」，避免面试被深挖。
3. **项目排序**：GrayDraft 放第一（与岗位 AI Agent/RAG 最契合）；GrayDraft 体现全栈与产品化。
4. **量化数据**：72+ 表、14 Skill、9 微服务、22 次迁移、三级缓存等均来自代码库，可放心使用。
5. **垂直领域**：主岗表述聚焦 **Agent / RAG 工程**；若 JD 含 AI for Science，再补充迁移意愿与可选 Demo，勿虚构行业经验。
