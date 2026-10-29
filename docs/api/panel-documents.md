# 面板文档 CRUD API（GrayDraft 前置）

面向 [GrayDraft 知识库面板](../../GrayDraft_Web/docs/knowledge-base-panel-migration.md) 的按 `kb_type` 文档管理接口。GrayDraft BFF 代理这些路由，**不直连数据库**。

> **状态**：**已实现**（Core `KBPanelHandler`）。GrayDraft BFF 通过 `ListKnowledgeEntries` / `GetKnowledgeEntry` / `DeleteKnowledgeEntry` 代理。

## 约定

- `kb_type` 与 GrayDraft 工作区 1:1（`gd_<sha256(workspace_key)[:32]>`）。
- 正文变更（触发重新分块/向量化）仍走 [`ugc/notify`](ugc.md) 或 [`smart-import`](smart-import.md)；本组 API 侧重**列表、详情、元数据、删除**。

## `GET /api/v1/kb/{kb_type}/documents`

分页列出该分区下文档，供侧栏与画布卡片使用。

### Query

| 参数 | 说明 |
|------|------|
| `limit` | 默认 50，最大 100 |
| `offset` | 默认 0 |
| `index_ready` | 可选 `true` / `false` 筛选 |

### 响应 `200`

```json
{
  "kb_type": "gd_abcd…",
  "total": 12,
  "items": [
    {
      "id": "uuid",
      "title": "社区规范",
      "preview": "禁止广告…",
      "index_ready": true,
      "quality_score": 0.72,
      "chunk_count": 4,
      "updated_at": "2026-10-28T10:00:00Z"
    }
  ]
}
```

`preview` 取正文或首 chunk 前 N 字符（建议 N=120）。

## `GET /api/v1/kb/{kb_type}/documents/{doc_id}`

单条详情，含 chunks 与 QA 摘要。

### 响应 `200`

```json
{
  "id": "uuid",
  "kb_type": "gd_abcd…",
  "title": "社区规范",
  "content": "完整正文…",
  "source_url": "https://…",
  "index_ready": true,
  "quality_score": 0.72,
  "kb_version": 2,
  "extensions": {},
  "chunks": [
    {
      "id": "uuid",
      "chunk_index": 0,
      "content": "…",
      "index_ready": true,
      "quality_score": 0.7
    }
  ],
  "qa_pairs": [
    { "question": "…", "answer": "…" }
  ],
  "updated_at": "2026-10-28T10:00:00Z"
}
```

## `DELETE /api/v1/kb/{kb_type}/documents/{doc_id}`

删除文档及该 `kb_type` 分区下的关联 chunks / QA。

### 响应

- `204 No Content` 成功  
- `404` 文档不存在或 `kb_type` 不匹配  

## `PATCH /api/v1/kb/{kb_type}/documents/{doc_id}`（可选）

仅更新元数据，**不**触发重分块。

```json
{
  "title": "新标题",
  "extensions": { "article_id": "x" }
}
```

正文修改请使用 `POST /api/v1/ugc/notify`（带 `doc_id`）或 smart-import `action: update`。

## 与现有 API 的分工

| 操作 | 使用接口 |
|------|----------|
| 新增 / 更新正文 | `POST /api/v1/ugc/notify`、`POST /api/v1/ugc/smart-import` |
| 列表 / 详情 / 删除 | 本文档 |
| 入库任务状态 | `GET /api/v1/ugc/tasks/:task_id` |
| RAG 问答 | `POST /api/v2/agent-runs` + SSE（BFF：`/api/knowledge/chat/stream`） |

## `POST /api/v1/kb/types/ensure`

GrayDraft 打开工作区时注册分区（幂等）。

```json
{ "name": "gd_…", "description": "GrayDraft workspace partition …" }
```

响应：`{ "name": "…", "ensured": true }`

## 实现清单（Core）

- [x] `document_panel_repo.go`：List / Get / Delete + `EnsureKBType`
- [x] `handler/kb_panel.go` + `cmd/gateway` 路由
- [x] 删除时 bump `kb_catalog_version`
- [ ] 集成测试或场景样例扩展「列表面板」用例
