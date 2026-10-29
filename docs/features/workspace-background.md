# 工作区背景（Workspace Background / Context）实现方案

> 目标：让 GrayDraft 从“通用知识库”进化为“领域专家系统”。
> **状态：已实现**（`000001_init.up.sql`、Core API、导入/向量化/问答注入、GrayDraft 设置页）。

已实现能力：

| 环节 | 实现 |
|------|------|
| 存储 | `kb_types.context_profile` JSONB；`GET/PUT /api/v1/kb/:kb_type/context` |
| 导入 | `ComposePromptWithContext` + `InferTitleLLM` 背景块；UGC `extensions.workspace_context_profile` |
| 向量化 | Worker `contextual_embed.py` 前缀 `【描述】实体 | 正文` |
| 检索 | `workspace_context` + `context_profile`；实体命中加权 |
| 问答 | Agent v2 Run + BFF SSE 映射；工作区背景注入 prompt |
| 配置 | GrayDraft **设置 → Agent 配置** 结构化表单；保存同步 Core |

## 1. 价值与问题定位

| 问题 | 无背景 | 有结构化背景 |
|------|--------|--------------|
| 实体消歧义 | “小夜”→ 人名/傍晚 | “InternLM 教程”→ 多模态模型“小夜” |
| OCR 错误 | 模型名模糊 | 按工作区实体矫正为 InternVL 等专名 |
| 视觉噪音 | 复古风格、UI 描述入库 | `rules` 过滤 |
| 标题质量 | `json`、`梳理附近...` | LLM 结合背景生成 `【InternLM】模型发布详情` |
| 检索置信度 | 0.158 | 背景关键词 boost → ≥0.3 |
| 幻觉 | 通用“编造实验数值” | 工作区规则“使用引用原文” |

**核心价值**：统一 Prompt 工程 + 上下文向量化 + 动态 System Message。

## 2. 设计原则（三位一体）

1. **结构化画像**（非纯文本）：`description` + `entities` + `rules` + `negative_context`
2. **导入端注入**：VLM 解析时把背景作为 High-Level Context
3. **存储端上下文向量化**：Embedding 前缀 `【工作区】entities | text`
4. **检索/问答端动态注入**：query embedding 前缀 + Chat System Message

## 3. 数据模型

### 3.1 扩展 `kb_types` 表

```sql
ALTER TABLE kb_types
  ADD COLUMN IF NOT EXISTS context_profile JSONB;

COMMENT ON COLUMN kb_types.context_profile IS
  '工作区背景：description, entities[], rules[], negative_context';
```

**示例**（书生教程）：

```json
{
  "description": "书生模型教程、API 说明与课件页",
  "entities": ["InternVL", "InternLM", "参数", "章节", "图注"],
  "rules": [
    "忽略与资料无关的装饰",
    "引用必须落到原文切片",
    "优先保留参数表和章节标题",
    "标题必须体现文档与主题"
  ],
  "negative_context": ["未注明出处的实验数值", "广告"]
}
```

### 3.2 迁移脚本

`deploy/migrations/000001_init.up.sql` 中的 `kb_types.context_profile`

```sql
ALTER TABLE kb_types ADD COLUMN IF NOT EXISTS context_profile JSONB;
CREATE INDEX IF NOT EXISTS idx_kb_types_context_profile_gin
  ON kb_types USING GIN (context_profile);
```

## 4. 实现集成点

### 4.1 GrayDraft 配置入口（推荐）

- **位置**：`SettingsAgentPanel.vue` 新增「工作区背景」Tab / Section
- **表单**：
  - `description`（文本）
  - `entities`（多标签输入）
  - `rules`（多行文本，每行一条规则）
  - `negative_context`（可选）
- **持久化**：BFF 调用 `PUT /api/v1/workspaces/:kb_type/context`（Core Admin API）
- **下发**：`SearchRequest.workspace_context` + `SmartImportRequest.workspace_context` 携带结构化 JSON 字符串或 profile 快照

### 4.2 Core API 扩展

新增端点（`internal/core/handler/workspace_context.go`）：

```
GET  /api/v1/workspaces/:kb_type/context
PUT  /api/v1/workspaces/:kb_type/context   （Admin 或 BFF 密钥）
```

`WorkspaceContext` 模型：

```go
type WorkspaceContext struct {
    KBType           string   `json:"kb_type"`
    Description      string   `json:"description"`
    Entities         []string `json:"entities"`
    Rules            []string `json:"rules"`
    NegativeContext  []string `json:"negative_context,omitempty"`
}
```

### 4.3 导入端注入（Multimodal + SmartImport）

**修改点**：

1. `multimodal/prompt.go` 增加 `ComposePromptWithContext`：

```go
func ComposePromptWithContext(base, workspace, ctxProfile string) string {
    p := ComposePrompt(base, workspace)
    if ctxProfile == "" {
        return p
    }
    return p + "\n\n【工作区背景】\n" + ctxProfile
}
```

2. `smartimport/preprocess.go`：

   - `SmartImportRequest` 新增 `WorkspaceContext string`
   - `preprocessRequest` 调用 `ProcessAttachments` 时传入上下文
   - LLM 标题推断（`InferTitleLLM`）也接收上下文，生成带 `【工作区】` 的标题

3. **效果**：
   - VLM 看到规则“忽略视觉描述”→ 不再输出“复古风格”
   - 标题从 `json` 变为 `【InternLM 教程】模型发布详情`

### 4.4 存储端上下文向量化（Worker）

**修改点**：`internal/worker/worker/pipeline/ingest.py`

```python
def build_contextual_text(title, text, workspace_ctx: dict | None) -> str:
    if not workspace_ctx:
        return text
    prefix = f"【{workspace_ctx.get('kb_type','')}】"
    ents = " ".join(workspace_ctx.get("entities", []))
    return f"{prefix} {ents} | {text}"
```

- 在 `embed_texts` 前对每个 chunk 调用
- 同时支持 `embedding`（通用语义）和 `embedding_structured`（结构化 KV）

**效果**：同一工作区的内容在向量空间自然聚拢，检索“小夜”时背景相似的 chunk 余弦分更高。

### 4.5 检索 / 问答端动态注入

**已有**（`hybrid.go:136`）：

```go
if wc := strings.TrimSpace(req.WorkspaceContext); wc != "" {
    embedQuery = wc + "\n" + embedQuery
}
```

**Agent v2（当前）**：

- BFF / Launcher 将 `context_profile` 注入 Run prompt；`knowledge.semantic` 检索时可附带 workspace 上下文
- 实体 bonus 仍由 `confidence/` 与工作区 profile 计算

**历史（v1 Chat，已下线）**：

- 旧 Chat handler（`service/chat/service.go`）曾在 System Message 注入工作区背景

## 5. Prompt 示例（多模态）

```markdown
你是一个多模态内容提取助手。...（DefaultBasePrompt）

【工作区补充指令】
忽略所有视觉风格描述

【工作区背景】
书生模型教程、API 说明与课件页
核心实体：模型、发布时间、参数说明、引用原文
规则：
- 所有日期默认为 2026 年
- 标题必须体现工作区与主题
```

## 6. 端到端示例

**用户操作**：
1. 设置工作区背景：`description="InternLM 教程"`，`entities=["模型","发布时间"]`，`rules=["忽略视觉描述"]`
2. 快速导入 5 张模型发布图 + 文字“梳理附近中模型发布时间”

**系统行为**：
- VLM 解析时自动过滤“复古风格”，标题生成 `【InternLM 教程】模型发布详情`
- 正文合并为要点表 + 分图详情（带教程版本上下文）
- Embedding 前缀 `【InternLM 教程】模型 发布时间 | ...`
- 用户问“小夜”→ 检索命中带背景的 chunk，置信度从 0.158 → 0.35+

## 7. 兼容性与降级

- `context_profile` 为空 → 完全回退到现有行为
- `workspace_context` 字符串与结构化 profile 可同时存在（profile 优先级更高）
- 旧数据无需迁移

## 8. 相关文档

- [多模态内容处理](multimodal.md) — Prompt 叠加机制
- [智能快速导入](smart-import.md) — 标题 LLM + 预处理
- [系统逻辑总览](../architecture/系统逻辑总览.md) — `kb_types`、`workspace_context`
- [GrayDraft 集成](../integrations/graydraft.md) — BFF 设置同步

## 9. 后续优化（roadmap 对齐）

| 编号 | 项 | 说明 |
|------|-----|------|
| O-20 | 背景驱动的检索重排序 | 命中 `entities` 的 chunk 额外加权 |
| O-21 | 背景版本管理 | profile 变更触发全量 re-embed（可选） |
| O-22 | Console 工作区背景管理页 | 取代 GrayDraft 独占 |

---

**总结**：通过「结构化 `context_profile` + 导入端 VLM 注入 + 存储端上下文 Embedding + 问答端 System Message」，GrayDraft 真正成为每个工作区的“领域专家”，同时解决标题质量与检索置信度两大痛点。
