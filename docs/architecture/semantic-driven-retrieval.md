> **基线日期**：2026-10-28  
> **完整方案（已实现逻辑 + 未实现清单）** → [semantic-driven-retrieval-scheme.md](./semantic-driven-retrieval-scheme.md)

## 一、工作区语义蓝图

| 能力 | 实现 |
|------|------|
| Auto-Taxonomy | `taxonomyevolve/blueprint.go` → `workspace_4d_taxonomy` |
| Embedding Aligner | `taxonomyevolve/aligner.go`（默认 ≥0.85） |
| YAML 降级为脚手架 | `taxonomy_config.yaml` + `BuildEffectiveTaxonomy` |

## 二、原子 NER

| 能力 | 实现 |
|------|------|
| LLM 优先 Query4D | `retriever/query4d.go` `ExtractQuery4D` |
| 短词博弈 | `model/entity_sanitize.go` `FinalSanitize` / `atomicLongShortDedup` |
| 分区实体别名 | `classifier/alias.go` `CanonicalizeEntities` + `postgres/entity_alias_repo.go` |

## 三、瀑布 + 软过滤

| Tier | 后缀 | 行为 |
|------|------|------|
| 1 | (none) | 全量 4D |
| 2 | `_wf_core` | 去 scene/style |
| 3 | `_wf_ent` | 最短实体 |
| 4 | `_wf_dom` | 仅 domain |
| 5 | `_wf_soft` | 无 SQL 4D + `BoostSemantic4D` |

实体 SQL 双向 ILIKE：`postgres/semantic_filter_entities.go`

## 四、进化闭环

| 能力 | 实现 |
|------|------|
| Delta | `EnqueueMappingDelta` / `RecordDelta` |
| Merge / Birth | `merge_llm.go`, `birth.go`, `Compress` |
| 影子回填 | Core `taxonomyevolve/backfill.go` → Worker `POST /v1/taxonomy/shadow-backfill` |

## 五、动态门限

| 意图 | 基础阈值 |
|------|----------|
| fact | 0.25 |
| concept | 0.15 |
| default | 0.20 |

动态调整：`confidence/threshold.go` `RetrievalGateThresholdDynamic`（质量分/标题命中略放宽）

## 配置建议

- `classifier.llm_enabled: true`
- `taxonomy_evolve.blueprint_enabled: true`
- `retrieval.semantic_4d_llm_enabled: true`
- `retrieval.semantic_4d_entity_strict: false`（dev/prod 均推荐，配合瀑布）

---

## 六、进化 Roadmap（勾选表）

状态说明：**[x]** 已落地 · **[~]** 混合/部分 · **[ ]** 待做

| 状态 | 能力 | Before（程序员规则） | After（目标） | 代码 / 配置 |
|:---|:---|:---|:---|:---|
| **[x]** | Universal Scaffold | 垂直行业 YAML seed | 仅 `universal` 通用标签 | `taxonomy_config.yaml`, `model/taxonomy_catalog.go` `UniversalDomainKey` |
| **[~]** | 场景 Domain | 改 YAML + 重启 | Blueprint 自发现 | `BuildFromBlueprint` + `WorkspacePendingTaxonomy`；冷启动不再行业推断 |
| **[x]** | 入库分区 | tutorial/paper/note | 仅 `gd_*` | `smartimport/kb_type.go`, GrayDraft `pkg/nexus/client.go` `hint_kb_type` |
| **[~]** | 标签归一化 Scene/Style | `GenericNormalize` 子串 | Embedding 优先 | `taxonomyevolve/aligner.go`（`FromBlueprint` 时跳过 `GenericNormalize`）<br>YAML 脚手架仍走关键词：`model/taxonomy.go` |
| **[~]** | 实体 NER | 停用词/正则表 | LLM 轻净化 + 规则兜底 | `SanitizeQueryEntitiesLLM`, `FinalSanitizeWithMethod` |
| **[x]** | 检索可观测性 | 黑盒 waterfall | `retrieval_trace` | `model/retrieval_trace.go`, `hybrid.Search` |
| **[x]** | 实体别名对齐 | 手工 Mapping | KB 别名组最短词 | `classifier/alias.go` `CanonicalizeEntities`<br>`retriever/hybrid.go`, `postgres/entity_alias_repo.go` |
| **[x]** | 检索硬过滤 | Strict AND 一票否决 | 瀑布 + 软加分 | `retriever/waterfall.go`, `retriever/query4d.go` `BoostSemantic4D` |
| **[x]** | 实体 SQL | 单向 ILIKE | 双向 ILIKE | `postgres/semantic_filter_entities.go` |
| **[x]** | prod 实体严模式 | `entity_strict: true` | 与瀑布一致 `false` | `config.prod.yaml` `retrieval.semantic_4d_entity_strict` |
| **[x]** | 聊天 Grounding | 吃/行程/位置关键词 | 质量分 + 命中信号 + 词元重叠 | `confidence/grounding.go` |
| **[~]** | 知识进化 | 人工改 Mapping | Delta/Merge/Birth + 影子回填 | `taxonomyevolve/service.go`, `backfill.go`<br>Worker `worker/pipeline/taxonomy_backfill.py`<br>依赖 `ai_worker.url`、`000001_init.up.sql` |
| **[x]** | Blueprint 冷启动 | 仅背景文案触发 | 有文档即可排队 | `taxonomyevolve/blueprint.go` `MaybeEnqueueBlueprint` + `ListRecentDocSummaries` |
| **[x]** | 导入 mixed 判定 | tutorial/paper 词频 | 骨架 `doc_type` | `smartimport/split_signals.go`, `skeleton.go` |
| **[x]** | 导入拆段/标题 | 规则骨架 + 正则标题 | 统一 LLM 为主 | `Run` → `runUnifiedImportLLM`；`PlanSegments` 优先 `loadCachedSplitPlan` / `splitPlanFromUnifiedLLM` |
| **[x]** | 去掉行业 YAML seed | 垂直行业预置 | Universal 底座 | 见上；文件保留为 universal 配置 |
| **[x]** | 实体→蓝图向量对齐 | 无 | 专名 embedding + 别名 | `taxonomyevolve/entity_align.go` `AlignEntitiesEmbedding`（Blueprint）；`classifier/alias.go` |
| **[x]** | 控制台/E2E 遗留 | 分区与教育语料一致 | 仅 gd_* | `internal/console/frontend/src/App.tsx`, `tests/contest/edu/` |
| **[x]** | Blueprint Validator | 无自检 | scene/style 区分度 + 重生 | `taxonomyevolve/blueprint_validator.go`, `blueprint_validator_enabled` |
| **[~]** | Global Entity Pool | 无跨分区 | DB `gd_*` DISTINCT 注入 query 实体 | `postgres/entity_pool.go`, `retriever/entity_pool.go`, `hybrid.go` |
| **[x]** | Chat 轨迹回显 | 仅 Search JSON | Agent 面板 `retrieval_trace` | `model/chat.go`, GrayDraft `AgentDock.vue` |
| **[x]** | 导入规则门控 | 失败即规则拆段 | 双次 LLM 失败才 `planSegmentsRules` | `unified_import_service.go`, `plan_segments.go` |

### 专名 Query 链路（对照案例）

| 步骤 | 实现 |
|------|------|
| 原子实体 | `model/entity_sanitize.go` `FinalSanitize` |
| 别名最短词 | `classifier/alias.go` → `hybrid.go` |
| 瀑布退避 | `retriever/waterfall.go` Tier 1→5 |
| 双向实体匹配 | `semantic_filter_entities.go` |
| 拒答门限 | `confidence/gate.go` + `threshold.go`（非领域关键词） |

### 运维检查清单

- [ ] 工作区已保存 Context Profile 或已有 ≥1 篇 `index_ready` 文档
- [ ] `workspace_4d_taxonomy.blueprint_status = ready`
- [ ] Core `taxonomy_evolve.blueprint_enabled: true`，Worker 可达
- [ ] Blueprint version bump 后 Redis purge + 影子回填完成
- [ ] `retrieval.semantic_4d_llm_enabled: true`，`semantic_4d_entity_strict: false`
