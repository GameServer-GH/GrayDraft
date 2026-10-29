# 业务扩展字段 `extensions`

知识库文档（`kb_documents`）支持**任意 JSON 对象**。检索与 Agent 对话会在引用中**原样带回**该对象。主题字段名是 `subject`。

## 存储位置

| 层级 | 字段 | 说明 |
|------|------|------|
| 文档 | `kb_documents.extensions` | 入库时写入，业务主键 |
| 切片 | `kb_chunks.metadata` | 预留；当前对话以文档级 `extensions` 为准 |

迁移：`deploy/migrations/000001_init.up.sql`（`kb_documents.extensions`）

## 入库

### `POST /api/v1/ugc/notify`

```json
{
  "kb_type": "tutorial",
  "title": "教程说明",
  "content": "……正文……",
  "extensions": {
    "article_id": "post_88421",
    "article_type": "guide",
    "doc_id": "internvl-quickstart",
    "author_uid": "u_123"
  }
}
```

### `POST /api/v1/ugc/smart-import`

同一请求体可带 `extensions`，提交入库时写入对应文档。

### 旅游示例

```json
{
  "kb_type": "tutorial",
  "title": "大理古城步行路线",
  "content": "……",
  "extensions": {
    "place_name": "大理古城",
    "country": "CN",
    "lat": 25.6065,
    "lng": 100.2676,
    "poi_type": "scenic"
  }
}
```

**约定**：字段名与结构由**调用方**定义，GrayDraft 不校验 schema；建议同一 `kb_type` 内保持字段稳定，便于客户端解析。

## 对话与检索返回

### 检索 `POST /api/v1/retrieval/search`

每个 `chunks[]` 项含：

```json
{
  "doc_id": "…",
  "doc_title": "…",
  "extensions": { "article_id": "post_88421", "article_type": "guide" }
}
```

### Agent v2 对话（`answer.validated` / BFF `done`）

`answer.validated` 或 BFF 映射后的 `done` 事件中：

| 字段 | 说明 |
|------|------|
| `sources[]` | 每个引用 chunk，含 `extensions` |
| `citations[]` | 按 **文档** 聚合，同一文章只出现一次，含 `chunk_ids` 与 `extensions` |

示例：

```json
{
  "type": "done",
  "confidence": 0.72,
  "citations": [
    {
      "doc_id": "550e8400-e29b-41d4-a716-446655440000",
      "doc_title": "教程说明",
      "kb_type": "tutorial",
      "extensions": {
        "article_id": "post_88421",
        "article_type": "guide"
      },
      "chunk_ids": ["…", "…"]
    }
  ],
  "sources": [
    {
      "chunk_id": "…",
      "doc_id": "550e8400-e29b-41d4-a716-446655440000",
      "extensions": { "article_id": "post_88421", "article_type": "guide" }
    }
  ]
}
```

调用方收到 `done` 后可根据 `citations[].extensions.article_id` 渲染原文链接。

### 点赞 / 点踩

`POST /api/v1/chat/feedback` 的 `chunk_ids` 仍来自 `sources`；响应 `docs[]` 仅含质量分变化。扩展字段不因评价而修改。

## 更新扩展字段

更新文档内容并保留 `doc_id` 再次 `notify` 时，传入新的 `extensions` 会覆盖旧值（Worker `upsert_document`）。

## 集成建议

1. 入库：`article_id` 等业务主键放在 `extensions`，正文放在 `content`。  
2. 问答：解析 SSE `citations` 而非仅从答案文本正则提取 ID。  
3. 多库：不同 `kb_type` 可使用不同 extensions 形状（tutorial 与 paper）。  
4. 可选：在业务侧维护 extensions JSON Schema，入库前自行校验。
