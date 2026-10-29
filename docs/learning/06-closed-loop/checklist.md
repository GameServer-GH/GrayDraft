# GrayDraft 学习闭环清单

上一篇：[测试即规格](../05-practice/tests-as-specs.md) · 下一篇：[自测题](self-quiz.md)

逐项勾选。无法解释的项不要死记，回到对应章节，结合代码和测试重新走一遍。

---

## A. 全局与边界

- [ ] 能用一分钟介绍 GrayDraft 的目标、输入、输出与使用方
- [ ] 能画出 Core、Worker、Gateway、BFF、PostgreSQL、Redis、NATS 的关系
- [ ] 能解释 `docs/learning`、`docs/architecture`、`docs/api` 的职责差异
- [ ] 知道 Agent v2 当前入口，避免把旧 v1 Chat 当成对外主流程

返回：[系统总览](../00-overview/system-overview.md) · [代码地图](../00-overview/code-map.md)

## B. Agent 基础

- [ ] 能解释 Agent Loop 的状态、动作、观察与停止条件
- [ ] 能解释 Tool Registry、Schema 与治理级别的职责
- [ ] 能区分工作状态、会话上下文与长期记忆
- [ ] 能说明 Answer Gate / Groundedness 为什么位于最终输出前

返回：[Agent 基础](../01-foundations/01-ai-agent-overview.md)

## C. 三条主链路

- [ ] 能从 UGC 请求说到 Worker `index_ready`
- [ ] 能从检索请求说到全文 / 向量召回、RRF、Rerank 与 Grade
- [ ] 能从创建 Agent Run 说到事件持久化与 SSE 回放
- [ ] 能解释 `knowledge.semantic` 如何把 Agent 与 Retriever 连接起来

返回：[入库](../02-pipeline/01-ingestion.md) · [检索](../02-pipeline/02-retrieval.md) · [Agent v2](../02-pipeline/03-agent-v2-sse.md)

## D. 算法与可靠性

- [ ] 能解释为什么 RRF 使用排名而不是直接相加异构分数
- [ ] 能区分召回、融合、重排、分级与门控
- [ ] 能说明 Query4D 和四维瀑布的作用
- [ ] 能解释低相关、无证据、冲突证据分别如何影响回答

返回：[算法总览](../03-algorithms/README.md) · [RRF](../03-algorithms/03-rrf.md) · [可靠性](../03-algorithms/05-reliability-governance.md)

## E. 工程架构

- [ ] 能说明租户、KB 与 storage partition 的隔离关系
- [ ] 能解释 L1 / L2 缓存与 catalog version 的关系
- [ ] 能说明 Gateway 与 Core 的职责边界
- [ ] 能说明 GrayDraft BFF 为何不能只是透明反代
- [ ] 能指出一次 Agent Run 应查看的日志、Trace、Metric 与事件

返回：[工程架构](../04-architecture/01-core-service.md)

## F. 实践

- [ ] 本地启动过开发栈并确认健康状态
- [ ] 完成一次入库、一次检索和一次 Agent v2 Run
- [ ] 验证过 SSE `after_seq` 回放
- [ ] 跟踪过一次 `knowledge.semantic` 到 Hybrid Retriever
- [ ] 独立运行过 Agent、Retriever 与 Worker 的最小测试集

返回：[动手实践](../05-practice/hands-on.md) · [测试即规格](../05-practice/tests-as-specs.md)

---

## 闭环动作

```text
清单未完成
  → 做 self-quiz 定位薄弱阶段
  → 回到对应文档
  → 跟一遍代码
  → 跑相关测试
  → 用自己的话重新画图或复述

清单完成
  → 选择一个故障场景做排障演练
  → 再进入 07-career 组织面试表达
```

继续：[自测题](self-quiz.md)。
