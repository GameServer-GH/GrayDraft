# 存量文档 semantic 4D 回填

`semantic_tags_4d`、`global_summary` 由 **Smart Import 一体化 LLM** 写入 `kb_documents.extensions`，无法仅靠 SQL 从正文可靠推导。

## 方式 A：回填 CLI（Fast LLM，仅补 metadata）

```bash
cd /path/to/GrayDraft
CONFIG_PATH=internal/core/config/config.dev.yaml \
  go run ./internal/core/cmd/backfill-semantic4d -kb_type=gd_xxx -limit=50
```

完成后一键串联：

```bash
make backfill-semantic-4d-all KB_TYPE=你的工作区分区 LIMIT=50
```

或分步：`make backfill-semantic-4d` → `make backfill-search-text KB_TYPE=...`

## 方式 B：重新智能导入（推荐质量最高）

1. 部署含 unified import 的 Core + Worker。
2. 对需要 4D 的文档 **重新执行智能导入**（或 PATCH 后触发再索引）。
3. 若仅需刷新检索文本，可执行 `make backfill-search-text`。

## 验证

- 文档详情 / Agent 检索 footer 应能看到 4D 标签。
- `POST /api/v1/retrieval/search` 响应含 `query_semantic_4d`、`semantic_4d_filter_applied`（配置开启时）。

## 说明

检索预过滤对**无 4D 的旧文档**仍宽松放行；回填后过滤与 `search_text` 注入才会完全生效。
