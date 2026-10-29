# GrayDraft 术语表

上一篇：[代码地图](code-map.md) · 下一篇：[完整知识地图](learning-map.md)

---

| 术语 | 在 GrayDraft 中的含义 |
|------|----------------------|
| Tenant | 租户隔离边界 |
| KB | Knowledge Base，租户下的知识库 |
| Storage Partition | 由作用域解析得到的物理或逻辑存储分区 |
| UGC | 用户知识内容及其通知、任务和入库管理 |
| Chunk | 文档经过解析与分块后的最小检索单元 |
| `index_ready` | 文档已完成可检索索引的状态 |
| Embedding | 把文本映射为向量，用于语义相似度检索 |
| Hybrid Retrieval | 组合全文、向量与结构化条件的混合检索 |
| Query4D | GrayDraft 的四维查询语义提取与约束表达 |
| RRF | Reciprocal Rank Fusion，按多个结果列表中的名次融合，不直接混合异构原始分数 |
| Rerank | 对召回候选做更精细的相关性重排 |
| Grade A/B/C/D | 对检索或回答质量进行分级的治理信号 |
| Gate | 在继续执行、输出或拒答前进行阈值与规则判断 |
| Agent Run | 一次可持久化、可追踪、可取消和可回放的 Agent 执行 |
| Agent Loop | 根据当前状态反复规划、调用工具、观察结果，直到满足停止条件 |
| Tool Registry | 工具声明、查找与治理入口 |
| `knowledge.semantic` | Agent Runtime 调用 GrayDraft 混合检索的知识工具 |
| Memory | 为 Agent 提供工作状态、会话信息或长期语义信息的记忆机制 |
| SSE | Server-Sent Events，GrayDraft 用于 Run 事件回放与实时推送 |
| `after_seq` | SSE 断线恢复时指定从哪个持久化事件序号之后继续 |
| BFF | Backend for Frontend，为 GrayDraft 等产品侧做作用域映射、字段注入与协议适配 |
| RAGOps | 围绕检索增强生成的评估、诊断和修复能力 |
| JetStream | NATS 的持久消息能力，用于异步任务和 Agent Run 调度 |
| Groundedness | 回答是否由检索证据充分支撑 |
| Catalog Version | 知识目录版本，用于缓存失效和一致性判断 |

遇到术语与代码不一致时，以当前类型、配置和测试中的命名为准。

继续：[AI Agent 核心机制全景图](../01-foundations/01-ai-agent-overview.md)。
