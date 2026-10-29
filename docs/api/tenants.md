# 多租户与知识库（PaaS API · Phase A）

> **基线日期**：2026-10-28

GrayDraft 从「仅 `kb_type` 分区」演进为 **`tenant_id` + `kb_id`** 的平台 API，底层仍使用 `kb_types.name` 作为 Postgres 存储分区键（`storage_partition`），保证与现有数据兼容。

GrayDraft 工作区映射：

| 概念 | 值 |
|------|-----|
| `tenant_id` | `tn_` + SHA256(workspace_key)[:16] |
| `kb_id` | `default`（单工作区单库） |
| `storage_partition` | 原 `gd_<hash>`（与 `nexusKbType` 一致） |

旧路径 **`/api/v1/kb/{kb_type}/...`** 继续可用；新集成请优先使用本页路径。

---

## 注册租户

### `POST /api/v1/tenants/ensure`

```json
{ "tenant_id": "tn_a1b2...", "display_name": "Acme Corp" }
```

幂等：已存在则更新 `display_name`（非空时）。

---

## 注册知识库

### `POST /api/v1/tenants/{tenant_id}/kbs/ensure`

```json
{
  "kb_id": "default",
  "display_name": "Main KB",
  "description": "optional",
  "storage_partition": "gd_abc..."
}
```

| 字段 | 说明 |
|------|------|
| `kb_id` | 租户下知识库 ID（字母数字、`_`、`-`，≤64） |
| `storage_partition` | 可选；为空时自动生成 `t_{tenant}_{kb}` 并注册 `kb_types` |

---

## 文档面板（与 panel-documents 等价）

| 方法 | 路径 |
|------|------|
| GET | `/api/v1/tenants/{tenant_id}/kbs/{kb_id}/documents` |
| GET | `/api/v1/tenants/{tenant_id}/kbs/{kb_id}/documents/{doc_id}` |
| DELETE | `/api/v1/tenants/{tenant_id}/kbs/{kb_id}/documents/{doc_id}` |

列表响应示例：

```json
{
  "tenant_id": "tn_...",
  "kb_id": "default",
  "storage_partition": "gd_...",
  "kb_type": "gd_...",
  "total": 1,
  "items": []
}
```

`kb_type` 为 `storage_partition` 别名，便于旧客户端过渡。

---

## 无状态 API 的作用域

以下接口支持 **`tenant_id` + `kb_id`**（JSON body）或请求头：

| Header | 说明 |
|--------|------|
| `X-GrayDraft-Tenant-Id` | 租户 ID |
| `X-GrayDraft-KB-Id` | 知识库 ID |

仍可使用仅 **`kb_type`** 的 legacy 写法。

| 接口 | 作用域字段 |
|------|------------|
| `POST /api/v1/ugc/notify` | `tenant_id`, `kb_id` 或 `kb_type` |
| `POST /api/v1/ugc/smart-import` | 同上；`hint_kb_type` 为 legacy |
| `POST /api/v1/retrieval/search` | 同上 |
| `POST /api/v2/agent-runs` | 同上；`X-Authorized-KBs` 约束检索范围 |

解析规则：若 `tenant_id` 与 `kb_id` 均非空，查 `knowledge_bases` 表得 `storage_partition`；否则使用 `kb_type`。

---

## 迁移

1. 执行迁移 `deploy/migrations/000001_init.up.sql`（含 `tenants`、`knowledge_bases`）
2. GrayDraft 打开工作区时调用 `tenants/ensure` + `kbs/ensure`，写入 `settings.json`：`nexusTenantId`、`nexusKbId`、`nexusKbType`
3. 第三方应用：自建 `tenant_id`，在其下创建多个 `kb_id`，无需 `gd_` 前缀

---

## 相关文档

- [panel-documents.md](panel-documents.md) — legacy `kb_type` 路径
- [ugc.md](ugc.md) · [smart-import.md](smart-import.md) · [retrieval.md](retrieval.md) · [agent-v2.md](agent-v2.md)
