# GrayDraft：语义中轴 + 动态切片（4.1）

## 目标

- **准**：切片带文档主题、4D 语义标签、全文摘要，降低向量误召回。
- **快**：语义重构仅在 Core 用 **Fast LLM 一次**；Worker 切片仍为规则，不增加 LLM。

## 流程

| 阶段 | 模块 | 动作 | 模型 |
|------|------|------|------|
| 识别 | `multimodal.Processor` | VLM → Markdown / 事实表 | 豆包 VL |
| 一体化导入 | `smartimport.RunUnifiedImportLLM` | 标题 + 4D + 摘要/结构 + `kb_type` + 拆分 | Gateway Fast |
| 切段 | `PlanSegments` | unified 缓存 / 规则 / `planWithLLM` 兜底 | 成功路径 0 次 |
| 分类 | `Analyze` | unified 缓存 / 规则 / 旧 LLM 兜底 | 成功路径 0 次 |
| 切片 | Worker `semantic_chunker` | `fact_table` / `section` / 段落切 | 无 LLM |
| 索引 | `build_search_text` + `metadata` | 文档 + **chunk** 级 4D 注入 | `text-embedding-v4` |

## 扩展字段

| 位置 | 键 | 说明 |
|------|-----|------|
| `kb_documents.extensions` | `semantic_tags_4d` | 文档级 4D |
| `kb_chunks.metadata` | `semantic_tags_4d` | **chunk 级派生 4D**（检索时与 doc 合并） |
| | `semantic_summary` / `search_intent` | 片段级（可选） |

检索 SQL：`d.extensions || c.metadata`（chunk 键覆盖文档同名字段）。

---

## 实现状态总览

| # | 能力 | 状态 |
|---|------|------|
| 1–14 | 核心链路（导入/检索/缓存/Console/回填 CLI 等） | ✅ 见历史提交 |
| 15 | **`kb_chunks.metadata` 持久化 chunk 4D** | ✅ `chunk_metadata_payload` + `insert_chunks` |
| 16 | **回填后串联 search_text** | ✅ `make backfill-semantic-4d-all` + Worker 脚本回填 |
| 17 | **Agent 面板 Query 4D 卡片** | ✅ `AgentDock` meta 区 + chat `meta` 透传 |

**方案 4.1 能力已全部落地。**

---

## 配置

```yaml
retrieval:
  semantic_4d_filter_enabled: true
  semantic_4d_llm_enabled: true
  semantic_4d_strict_filter: false
```

## 运维

| 命令 | 作用 |
|------|------|
| `make backfill-search-text KB_TYPE=xxx` | Worker 内重建 `search_text` + `metadata` |
| `make backfill-semantic-4d KB_TYPE=xxx` | Core LLM 回填文档 4D |
| `make backfill-semantic-4d-all KB_TYPE=xxx` | 先 4D 再 search_text |

## 代码入口

| 区域 | 路径 |
|------|------|
| Chunk metadata | `worker/indexer/chunk_semantic.py`、`worker/db/repository.py` |
| 检索 extensions 合并 | `postgres/chunk_extensions.go` |
| Agent Query 4D UI | `AgentDock.vue`、`agentMessageMeta.ts`、`useKnowledgeChat.ts` |
| 回填串联 | `Makefile` → `backfill-semantic-4d-all` |
