# GrayDraft 实施路线图与下一步建议

本文档汇总 Monorepo 分阶段建设进度与**后续优先级建议**。当前实现逻辑见 [系统逻辑总览](architecture/系统逻辑总览.md) 与 [功能实现](README.md#功能实现)。

---

当前优先建立正确性基线。离线纠错已完成固定样例与两条新答案的定向验收，下一项是独立留出题验证。线上旁路与尾延迟优化暂缓。[52 题复跑与旁路决策](architecture/correctness-rebaseline-shadow-decision-2026-11-04.md)

## 一、已完成能力（里程碑）

| 阶段 | 能力 | 关键文档 / 代码 |
|------|------|-----------------|
| 1 | 项目骨架、Monorepo、CI/Compose | `PROJECT_STRUCTURE.md`、`development.md` |
| 2 | Core 混合检索 + Postgres/pgvector | `api/retrieval.md` |
| 3 | Worker 解析/分块/门禁/QA/入库 | `api/worker.md` |
| 4 | Gateway Embedding + Worker 重排 | `api/gateway.md` |
| 5 | Core UGC + NATS 异步 | `api/ugc.md` |
| 6 | 三级检索缓存（L1/L2/L3） | `api/cache.md` |
| 7 | Gateway LLM + Agent v2 Run SSE | `api/agent-v2.md` |
| 8 | 查询分类器（规则版） | `api/classifier.md` |
| 9 | JetStream 持久化 UGC | `api/jetstream.md` |
| 10 | 对话答案缓存 | `api/answer-cache.md` |
| 11 | RAGOps 指标与自愈 | `api/ragops.md` |
| 12 | Console 运维 MVP | `api/console.md` |
| 13 | DLQ 运维 API + Console 页 | `api/dlq.md` |
| 14 | 模型路由（Embed/Rerank/LLM 可配置） | `api/models.md` |
| 15 | 智能导入 + 拆分 2.0 | `features/smart-import.md` |
| 16 | 多模态 4.0（VL/OCR、缓存、双向量） | `features/multimodal.md` |
| 17 | GrayDraft Agent（BFF + 工作区 Prompt） | `integrations/graydraft.md` |
| 18 | tenant_id + kb_id（PaaS Phase A） | `api/tenants.md` |
| 19 | 百炼 `text-embedding-v4` @ 1024（O-13） | `api/embedding.md` |
| 20 | 工作区 Agent Runtime（`edu_workspace_v1`） | `api/agent-v2.md`、`architecture/Agent流程技术实现方案.md` |

架构决策索引见 [`architecture/README.md`](architecture/README.md)（ADR-001～009）。

---

## 二、下一步建议（按优先级）

### P0 — 安全与可上线基础

| 序号 | 项 | 说明 |
|------|-----|------|
| 1 | **Console / Admin 鉴权** | ✅ O-03：JWT + Console 登录页 |
| 2 | **生产 NATS 高可用** | 3 节点集群、`replicas: 3`、备份策略 |
| 3 | **DLQ 自动告警** | ✅ O-03：Prometheus `alerts.yml` |

### P1 — 管理面板与产品化

| 序号 | 项 | 说明 |
|------|-----|------|
| 4 | **知识库 MVP** | 工作空间、kb CRUD、Key 占位（Console DB + Core） |
| 5 | **向量/分块管理** | Admin chunks 列表、筛选 index_ready / 质量分 |
| 6 | **Agent 测试台增强** | 测试历史、检索链路可视化 |

### P2 — 可观测与成本

| 序号 | 项 | 说明 |
|------|-----|------|
| 7 | **Worker / Gateway 指标** | 入库耗时、embed/LLM P95 |
| 8 | **答案缓存语义命中** | query embedding 相似答案 |
| 9 | **API 调用统计** | 按 Key / kb_type 聚合 |

### P3 — 模型与质量

| 序号 | 项 | 说明 |
|------|-----|------|
| 10 | **分类模型化** | 暂缓：赛事联调显示规则分类器已够用；优先热路径收窄（见 [upgrade-optimization-contest-2026-10-29](architecture/upgrade-optimization-contest-2026-10-29.md)），保留 `RoutePlan` |
| 11 | **批量回归 / RAGAS** | 用例集与多配置对比 |
| 12 | **多模态差距项** | 2048 维、OSS 原图、Chat 媒体引用、Console 运营台（见 `features/multimodal.md` §6） |

### P4 — 长期能力

| 序号 | 项 | 说明 |
|------|-----|------|
| 13 | 知识库版本切换与 Key 绑定 | |
| 14 | 对话式协作导入 | |
| 15 | 一键修复建议 + 补知识 | |
| 16 | 平台管理员全局视图与计费 | |

---

## 三、优化 backlog（O-xx）

| 编号 | 项 | 状态 | 说明 |
|------|-----|------|------|
| O-01 | QA 上下文锚点 | ✅ | Worker 读 `context_enhancement` / `subject` / `parent_title` |
| O-02 | 入库 trace_id | ✅ | BFF → Core → NATS → Worker |
| O-03 | Console JWT + DLQ 告警 | ✅ | |
| O-04 | NATS HA | 待做 | 生产 JetStream 集群 |
| O-05 | 语义去重建议 | 待做 | smart-import 预览相似文档 |
| O-06 | 可交互切割预览 | 待做 | `segments_override` |
| O-07 | 跨库检索 MVP | 待做 | `kb_types[]` |
| O-08 | 管理面板深化 | 待做 | 对齐 P1-4～6 |
| O-09 | 意图分类模型化 | 待做 | |
| O-10 | 答案忠实度 confidence | 待做 | NLI / LLM 验证 |
| O-11 | NATS 步骤化流水线 | 待做 | 拆 Embedding / QA 消费者 |
| O-12 | 指标与用量统计 | 待做 | |
| O-13 | 百炼 Embedding | ✅ | `api/embedding.md` |
| O-14 | tenant + kb_id | ✅ Phase A | Phase B/C 多 kb、废弃纯 kb_type 路径 |
| O-15 | OpenTelemetry | 待做 | 全链路 span |
| O-16 | 查询预算 / 成本 | 待做 | |
| O-17 | 全局物化索引 | ✅ | `api/ragops.md` |
| O-18 | 冷热分层 | ✅ | `storage_tier` |
| O-19 | RAGOps 自愈 | ✅ | strike + heal jobs |

**推荐 Sprint**：A = O-01～04（质量与排障）→ B = O-05～08（导入体验）→ C = O-09～13（算法）→ D = O-14 Phase B（平台化）。

---

## 四、依赖与风险

| 风险 | 缓解 |
|------|------|
| Admin API 暴露 | JWT + Nginx 限制 `/api/v1/admin` |
| JetStream 单节点 | P0-2 集群与副本 |
| 规则分类 / 1024 维与生产差距 | 测试台验证；升级向量需 re-embed |
| Console 与完整运营台差距 | 按 P1 分模块交付 |

---

## 五、相关文档

| 文档 | 用途 |
|------|------|
| [系统逻辑总览](architecture/系统逻辑总览.md) | 端到端实现 |
| [features/](features/) | 智能导入、多模态、检索对话 |
| [development.md](development.md) | 本地启动 |
| [deployment.md](deployment.md) | 部署与环境变量 |

---

*完成某项后可在第一节标记「已完成」并注明版本。*
