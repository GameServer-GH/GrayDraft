# 完成主线后的下一步

上一篇：[自测题](self-quiz.md) · 返回：[Learning 首页](../README.md)

完成 00–06 后，不建议继续无差别扩充阅读量。按目标选择一条深化路线。

## 路线 A：Agent Runtime

- 精读 `internal/core/service/agent/`
- 为 Loop、Tool Governance、Memory、Answer Gate 各补一张状态图
- 选一个停止条件或治理边界，补充测试并解释设计取舍

## 路线 B：RAG 与检索

- 精读 Hybrid Retriever、Query4D、RRF、Rerank 与 Grade
- 用固定问题集比较召回、重排和门控前后的结果
- 学习 `tests/agent-eval/golden.json` 与回归评估方法

## 路线 C：知识工程

- 精读 Smart Import、Worker Pipeline、Multimodal 与 Taxonomy Evolve
- 跟踪一份复杂文档的分块、标签、Embedding 与索引结果
- 设计一个质量门禁或去重边界用例

## 路线 D：生产工程

- 精读缓存、Gateway、可观测性、部署和 RAGOps
- 演练模型超时、Redis 不可用、低质量检索、SSE 断线等场景
- 建立“用户现象 → Metric / Trace / Log → 代码入口”的排障表

## 路线 E：求职表达

先完成一个可运行实验和一份代码笔记，再进入 [07-career](../07-career/01-interview-roadmap.md)。项目表达应围绕真实问题、方案、取舍、验证与结果，不要只罗列技术名词。

---

建议每次深化都留下三个产物：

1. 一张机制或时序图；
2. 一组可重复执行的命令 / 测试；
3. 一段不依赖文档也能讲清的项目说明。
