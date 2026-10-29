# GrayDraft Learning — 从使用到实现的学习闭环

> 目标：从「知道 GrayDraft 是什么」走到「能独立跑通入库、检索和 Agent v2，能沿代码解释关键算法、架构取舍与可靠性保障」。
>
> 技术事实以当前代码、[`docs/architecture/`](../architecture/README.md) 和 [`docs/api/`](../api/) 为准；本目录负责学习顺序、概念解释、代码入口、练习和自检。

本目录参考 MolMind Learning 的组织方式，按 **全局认知 → 基础机制 → 核心流水线 → 算法 → 工程架构 → 实践 → 自检** 编排。不要一开始就逐篇阅读所有算法或面试题，先走通主链路，再按薄弱点回看。

---

## 推荐学习路径

```text
00 全局认知：项目目标、系统边界、代码地图、术语
        ↓
01 Agent 基础：Loop、Tool、Memory、关键模块
        ↓
02 核心流水线：入库 → 检索 → Agent v2 + SSE
        ↓
03 算法与可靠性：召回、RRF、Rerank、分级、门控
        ↓
04 工程架构：Core、数据、缓存、Gateway、BFF、可观测、部署
        ↓
05 动手实践：启动、入库、检索、Agent Run、读测试
        ↓
06 自检闭环：清单 → 自测 → 回到薄弱章节再走一圈
        ↓
07 表达与复盘：把入库、检索和对话讲清楚（不属于核心必修）
```

| 阶段 | 建议用时 | 学完应能回答 |
|------|----------|--------------|
| [00-overview](00-overview/system-overview.md) | 0.5–1 天 | GrayDraft 解决什么问题？五类服务怎样协作？三条主链路是什么？ |
| [01-foundations](01-foundations/01-ai-agent-overview.md) | 1–2 天 | Loop、Tool、Memory 如何共同组成 Agent Runtime？ |
| [02-pipeline](02-pipeline/01-ingestion.md) | 3–5 天 | 一份知识如何入库、被检索，并通过 Agent v2 形成流式回答？ |
| [03-algorithms](03-algorithms/README.md) | 3–5 天 | Query4D、混合召回、RRF、Rerank、Grade 与门控各解决什么问题？ |
| [04-architecture](04-architecture/01-core-service.md) | 3–5 天 | 数据、缓存、模型网关、BFF、观测与部署如何支撑主链路？ |
| [05-practice](05-practice/hands-on.md) | 2–3 天 | 能否独立启动系统并完成一次入库 → 检索 → Agent Run？ |
| [06-closed-loop](06-closed-loop/checklist.md) | 0.5–1 天 | 能否不看笔记画图、读码、排障，并用测试验证理解？ |

完整走一轮约 **2–3 周**。如果每天只有 1–2 小时，可按 **4 周**完成；先保证顺序，不追求一次记住所有细节。

---

## 逐篇阅读顺序

### 00 · 全局认知

1. [系统总览](00-overview/system-overview.md)
2. [代码地图](00-overview/code-map.md)
3. [术语表](00-overview/glossary.md)
4. [完整知识地图](00-overview/learning-map.md)（需要查全量专题时使用）

### 01 · Agent 基础

1. [AI Agent 核心机制全景图](01-foundations/01-ai-agent-overview.md)
2. [Agent Loop](01-foundations/02-agent-loop.md)
3. [Tool Use](01-foundations/03-tool-use.md)
4. [Memory](01-foundations/04-memory.md)
5. [AI 关键模块](01-foundations/05-key-modules.md)

这一阶段先理解职责和状态流，不要求背 API 或实现细节。

### 02 · 核心流水线

1. [入库流水线](02-pipeline/01-ingestion.md)
2. [检索链路](02-pipeline/02-retrieval.md)
3. [Agent v2 对话与 SSE](02-pipeline/03-agent-v2-sse.md)
4. [智能导入](02-pipeline/04-smart-import.md)
5. [语义进化](02-pipeline/05-semantic-evolution.md)

前三篇是必修主链路；智能导入与语义进化属于进阶流水线。

### 03 · 算法与可靠性

1. [AI 算法总览](03-algorithms/README.md)
2. [标准与改良算法](03-algorithms/01-standard-and-improved.md)
3. [自研算法](03-algorithms/02-custom-algorithms.md)
4. [RRF 专题](03-algorithms/03-rrf.md)
5. [检索分级与门控](03-algorithms/04-retrieval-grading-and-gating.md)
6. [问答可靠性治理](03-algorithms/05-reliability-governance.md)

阅读时始终回答三个问题：输入输出是什么、失败时如何降级、哪组测试钉住了行为。

### 04 · 工程架构

1. [Core 服务架构](04-architecture/01-core-service.md)
2. [数据模型](04-architecture/02-data-model.md)
3. [缓存体系](04-architecture/03-cache.md)
4. [模型 Gateway](04-architecture/04-model-gateway.md)
5. [BFF 基础](04-architecture/05-bff.md)
6. [GrayDraft BFF 代理能力](04-architecture/06-bff-proxy-capabilities.md)
7. [可观测性](04-architecture/07-observability.md)
8. [部署运维](04-architecture/08-deployment-ops.md)

### 05 · 动手实践

1. [动手跑通主链路](05-practice/hands-on.md)
2. [测试即规格](05-practice/tests-as-specs.md)
3. [AI Agent / 应用开发进阶路线](05-practice/learning-roadmap-ai-agent.md)

### 06 · 自检闭环

1. [复习清单](06-closed-loop/checklist.md)
2. [自测题](06-closed-loop/self-quiz.md)
3. [下一步](06-closed-loop/next-steps.md)

### 07 · 表达与复盘（可选）

1. [面试与通用能力路线](07-career/01-interview-roadmap.md)
2. [AI Agent 面试全局视角](07-career/02-interview-overview.md)
3. [AI Agent 面试问答](07-career/03-interview-questions.md)
4. [简历素材](07-career/04-resume.md)

---

## 学习方法

1. **先图后码**：先看系统总览和代码地图，再进入具体实现。
2. **围绕一条请求读代码**：不要按目录逐文件阅读；分别跟踪入库、检索、Agent Run 三条请求。
3. **测试即规格**：每学完一个专题，找到对应 `_test.go` 或 `test_*.py`，用断言确认真实行为。
4. **文档与代码冲突时以当前实现为准**：尤其注意 Agent v2 已取代旧的 `/api/v1/chat/stream` 对外入口。
5. **每阶段都要有产出**：画一张图、写一段复述、跑一组测试，三者至少完成两项。

开始：[系统总览](00-overview/system-overview.md)。
