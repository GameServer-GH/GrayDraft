# 架构决策记录 (ADR)

| ADR | 标题 | 状态 |
|-----|------|------|
| ADR-001 | 单仓库多服务（Monorepo） | 已采纳 |
| ADR-002 | Go 编排 + Python AI Worker 异构架构 | 已采纳 |
| ADR-003 | PostgreSQL 16 + pgvector 分区 + 冷热表空间 | 已采纳 |
| ADR-004 | 三级语义缓存（Ristretto + Redis 版本感知） | 已采纳 |
| ADR-005 | NATS JetStream 异步 UGC 流水线 | 已采纳 |
| ADR-006 | 非常用端口映射策略 | 已采纳 |
| ADR-007 | Core Prometheus RAGOps 指标 | 已采纳 |
| ADR-008 | Console BFF + Admin API（运维 MVP） | 已采纳 |
| ADR-009 | JetStream DLQ Admin API + Console 页 | 已采纳 |

| 文档 | 说明 |
|------|------|
| [**设计理念**](设计理念.md) | AI 为操作主体；笔记、降噪与同类关卡不走逐条人审 |
| [**系统逻辑总览**](系统逻辑总览.md) | 当前实现的端到端逻辑；**§0 含 BFF 五项代理 + Core 模块 Mermaid 图** |
| [**教育场景 Agent 实现说明**](Agent流程技术实现方案.md) | `edu_workspace_v1` 的 Loop、工具、记忆与评测 |
| [**性能优化 1.0 方案**](performance-optimization-v1.md) | 导入与问答性能优化的 1.0 落地路线 |
| [**智能语义驱动检索方案**](semantic-driven-retrieval-scheme.md) | 语义 Blueprint / 瀑布 / 进化：已实现逻辑与待办 |
| [**赛事联调驱动升级优化**](upgrade-optimization-contest-2026-10-29.md) | SCP vs 论文差异技术分析 + 热路径收窄与验收目标 |
| [语义检索代码对照](semantic-driven-retrieval.md) | Roadmap 勾选与配置速查 |
| [PROJECT_STRUCTURE.md](../PROJECT_STRUCTURE.md) | 目录与分层速查 |
| [deployment.md](../deployment.md) | Compose、Nginx、环境变量 |
| [roadmap.md](../roadmap.md) | 里程碑与 backlog |
