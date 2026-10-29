# Agent 快速导入

支持**粘贴长文本或上传文件**，由 Core 自动识别 `kb_type`、标题、正文，并走 UGC 异步入库。流程说明见 [系统逻辑总览 · 流程 B](../architecture/系统逻辑总览.md#5-流程-b智能快速导入)。相同知识以**更新同一文档**方式处理（`doc_id` 复用，`kb_version` 递增）。

支持**拆分导入**：一条混杂长文本拆成多条知识分别入库。

## `POST /api/v1/ugc/smart-import`

### 请求

```json
{
  "raw_text": "# 社区规范\n\n禁止广告…\n---\n## 产品说明\n\nindex_ready…",
  "filename": "rules.md",
  "mime_type": "text/markdown",
  "source_url": "https://example.com/doc",
  "hint_kb_type": "",
  "generate_qa": true,
  "use_llm": true,
  "split": true,
  "submit": false,
  "min_confidence": 0.55,
  "extensions": {
    "article_id": "post_88421",
    "article_type": "guide"
  }
}
```

| 字段 | 必填 | 说明 |
|------|------|------|
| `raw_text` | 是 | 原始正文（粘贴或读取文件后传入） |
| `filename` | 否 | 用于推断 `mime_type` |
| `hint_kb_type` | 否 | 工作区分区；教育语料为 tutorial、paper、note |
| `use_llm` | 否 | 是否调用 Gateway LLM 辅助分类 |
| `split` | 否 | `true` 时按分隔线/标题/段落/主题块拆成多条，返回 `items[]` |
| `submit` | 否 | `false` 仅预览；`true` 分析后提交 `ugc/notify` |
| `min_confidence` | 否 | 覆盖配置阈值（默认见 `smart_import.min_confidence`，通常 `0.55`） |

### 置信度与拦截

**所有响应**（含 `400`/`422` 失败）均包含：

| 字段 | 说明 |
|------|------|
| `confidence` | 本条或拆分后的**最低**分段置信度，0–1 |
| `min_confidence_required` | 本次生效的最低阈值 |
| `import_allowed` | 是否允许提交入库 |
| `message` | 说明（含低于阈值时的原因） |

- 单条模式：`preview.confidence` 与顶层 `confidence` 一致。  
- 拆分模式：`items[].preview.confidence` 各段独立；顶层 `confidence` 为各段最小值。  
- `submit=true` 且 `import_allowed=false` 时返回 **HTTP 422**，body 仍带完整 `confidence` / `items` / `skipped`。  
- 拆分提交时，低于阈值的段写入 `skipped`，其余段正常入 `tasks`。

配置（`config.dev.yaml`）：

```yaml
smart_import:
  min_confidence: 0.55
```

环境变量：`SMART_IMPORT_MIN_CONFIDENCE`

### 响应预览 `preview`（单条） / `items[]`（拆分）

| 字段 | 说明 |
|------|------|
| `kb_type` | 识别出的知识库类型 |
| `title` | 识别标题（须在原文中出现，防 LLM 编造） |
| `content` | 规范化正文 |
| `content_hash` | SHA256，用于去重 |
| `action` | `create` / `update` / `unchanged` |
| `confidence` | 该段置信度 |
| `import_allowed` | 该段是否可入库（含阈值判断） |
| `existing_doc_id` | 更新/未变时已有文档 ID |
| `method` | `rules` / `rules+llm` |

`submit=true` 成功时返回 `task`（单条）或 `tasks[]`（拆分），结构同 [`ugc.md`](ugc.md)。

同时返回 `readiness`：

| 字段 | 说明 |
|------|------|
| `queryable` | 当前提交是否已经可被问答检索 |
| `status` | `preview` / `queued` / `ready` / `no_task` |
| `task_ids` | 已创建的 UGC task id 列表 |
| `doc_ids` | 同步入库路径可直接返回的文档 id |

当 `status=queued` 时，Agent 应在问答前调用 `GET /api/v1/ugc/tasks/:task_id?wait=true&timeout_ms=...`，或订阅 `GET /api/v1/ugc/tasks/:task_id/events`，直到 `queryable=true` 或终态；这能避免刚提交异步入库就被误判为「知识库未入录」。

### 智能拆分 2.0（`split=true`）

实现见 [智能快速导入（功能说明）](../features/smart-import.md)。

1. **结构预扫描**：提取标题、分隔线、Q/A 行等带行号骨架，推断 `doc_analysis`（主题、文档类型等）。  
2. **宏观策略**：单体文档默认 `splitting_strategy=single`；综合合集/分段/多 `##` 或两侧均 ≥40 字的分隔线 → `multi_segment`。  
3. **坐标切割**：按 `line_range [start,end]`（1-based）从原文截取，不依赖 `start_marker`。  
4. **可选 LLM**（`use_llm=true`）：基于骨架 JSON 决策；失败回退规则。  
5. 入库时写入 `extensions`：`subject`、`parent_title`、`context_enhancement`、`line_range`、`segment_index/total` 等。

响应额外字段：

| 字段 | 说明 |
|------|------|
| `doc_analysis` | `{ subject, version, doc_type, parent_title }` |
| `splitting_strategy` | `single` \| `multi_segment` |
| `split_method` | `rules` \| `llm` \| `rules+llm` |
| `items[].line_range` | 该段在原文中的行号范围 |
| `items[].context_enhancement` | QA/检索增强语境 |

### 去重规则

1. 同 `kb_type` + 相同 `content_hash` → `unchanged`  
2. 同 `kb_type` + 相同标题、内容变化 → `update`  
3. Worker 更新时 `kb_version` +1，并重建 chunks/QA  

## Agent 对话置信度

Agent v2 中，`knowledge.semantic` 工具输出与 `answer.validated` / BFF `done` 事件携带置信度字段：

```json
{ "retrieval_grade": "B", "coverage_score": 0.72, "retrieval_score": 0.68, "hits": [...] }
```

- 无检索结果时 grade 为 `D`、`retrieval_score` 为 0  
- 有结果时由 `confidence/gate.go` 分级（见 [检索分级](../learning/03-algorithms/04-retrieval-grading-and-gating.md)）

## H5 测试页

`tests/contest/edu` → **快速导入**：勾选 **拆分导入** → 粘贴 `samples/messy-mixed-import.txt` → **分析预览** 查看 `items` 与 `confidence` → **确认入库**。
