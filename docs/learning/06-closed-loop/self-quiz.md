# GrayDraft 自测题

上一篇：[复习清单](checklist.md) · 下一篇：[下一步](next-steps.md)

先不看文档回答。每题能讲清“机制 + 代码入口 + 一个测试或观察方法”才算掌握。

---

## 1. 全局

1. GrayDraft 与普通向量数据库或单一 RAG 接口的区别是什么？
2. Core、Worker、Gateway、BFF 为什么要分开？
3. PostgreSQL、Redis、NATS 分别保存或传递什么？

## 2. 入库与检索

4. 一份文档从 `ugc/notify` 到 `index_ready` 经历哪些阶段？
5. 为什么检索要同时使用全文、向量和结构化约束？
6. RRF 为什么不直接相加 BM25 与向量相似度？
7. Rerank 与 RRF 各解决什么问题？交换顺序会有什么影响？
8. Query4D 和四维瀑布怎样改变候选集合？

## 3. Agent v2

9. 为什么创建 Run 与订阅 SSE 被设计为两个请求？
10. Agent Loop 如何避免无限循环、成本失控或重复工具调用？
11. `knowledge.semantic` 的 Tool Result 如何影响下一步和最终回答？
12. `after_seq` 如何支持断线恢复？事件为什么需要持久化？
13. Memory 与普通 Prompt 上下文有什么区别？

## 4. 可靠性与工程

14. 检索 Grade、Groundedness、Answer Gate 分别在哪一层起作用？
15. 缓存为什么要感知 catalog version？
16. 模型 Gateway 失败时，Core 应该知道哪些信息，哪些细节应由 Gateway 隔离？
17. GrayDraft BFF 做了哪些产品语义到 GrayDraft 语义的转换？
18. 遇到“有 SSE 连接但没有最终回答”时，你会按什么顺序排查？

## 5. 实践题

19. 从任意一条 Agent v2 路由开始，定位到 Runtime、Tool、Retriever 与 Answer Gate。
20. 找一个测试说明 Agent 的停止条件，找另一个测试说明低证据回答如何处理。
21. 设计一个不删除数据的故障实验，说明预期日志、状态和用户结果。

---

## 评分方式

| 水平 | 标准 | 下一步 |
|------|------|--------|
| 0 | 说不出或只会背名词 | 回到对应阶段的总览文档 |
| 1 | 能解释概念 | 补代码入口 |
| 2 | 能解释概念和代码入口 | 补测试或运行观察 |
| 3 | 能讲清机制、代码、验证和失败处理 | 进入下一阶段或做项目表达 |

每题不必一次达到 3 分。优先把第 4–16 题补到 2 分以上，再做求职材料。

继续：[下一步](next-steps.md)。
