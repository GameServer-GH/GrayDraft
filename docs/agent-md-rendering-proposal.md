# Agent Markdown 输出规范 & 前端渲染方案

> **文档类型：设计方案与计划。** 正文含设计目标或待办，实际交付以架构、接口文档和代码为准。 [当前文档入口](README.md)

> 针对 GrayDraft Agent 输出 Markdown 格式不标准（标题缺空格、表格行粘连为单行）的前后端系统性改进方案。

---

## 一、问题分析（基于 2026-10-28 日志样本）

| 问题类型 | 实际输出 | 期望输出 |
|----------|---------|----------|
| H2 标题缺空格 | `##英雄定位` | `## 英雄定位` |
| 表格行粘连 | `\|类型\|英雄\|\|---\|---\|\|克制\|嫦娥\|\|被克制\|盘古\|` | 每行独立，含分隔行 |
| 多行压缩为单行 | 整个表格为一行 | 表头/分隔行/数据行各占一行 |

根因：LLM 在长输出时倾向于省略换行符以节省 token，尽管 system prompt 已通过 `promptmd.StandardBlock` 明确禁止 `||` 多行粘连，但 LLM 不完全遵守。

---

## 二、Agent 侧 (Backend) 改进方案

### 2.1 System Prompt 强化（promptmd.go）

当前 `StandardBlock` 已有表格规范，但可进一步强化：

```
// 在 StandardBlock 中补充更激进的表格示例（正面 + 反面对比）
// 在 LineBreakRules 中增加表格的 BAD/GOOD 示例
```

具体改动见 `/internal/core/service/promptmd/promptmd.go`，新增：

- **`TableRules`**：独立的表格格式化指令块，包含 BAD/GOOD 对比
- 在 `appendMarkdownOutputSpec` 中注入 `TableRules`

### 2.2 后端 Markdown 后处理（防御性修复）

在 `chat/prompt.go` 的 answer 构建阶段增加轻量级后处理：

```go
// internal/core/service/chat/mdfix/mdfix.go (新包)
// 对 LLM 输出做二次清洗：
// 1. 修复 ##标题 → ## 标题
// 2. 拆解 || 连接的表格行
// 3. 补全表格分隔行
```

**优先级：低** —— 前端 `normalizeKBMarkdown` 已足够处理，且后端改动风险更大（可能影响 SSE 流式增量）。

### 2.3 推荐策略

| 层级 | 策略 | 优先级 |
|------|------|--------|
| System Prompt | 强化表格规范 + BAD/GOOD 示例 | **P0 立即** |
| 前端 normalize | 增强 `normalizeKBMarkdown` 表修复 | **P0 立即** |
| 后端后处理 | Go 侧 mdfix 包 | P2 后续 |

---

## 三、前端 Markdown 渲染改进方案

### 3.1 normalizeKBMarkdown 增强（P0）

当前 `normalizeKBMarkdown` 修复了标题、编号、列表项粘连，但未处理表格。需要新增：

```ts
// 新增正则，将 "||" 连接的表格行转换为标准多行格式
// "|类型|英雄||---|---||克制|嫦娥||被克制|盘古|"
//   → "| 类型 | 英雄 |\n| --- | --- |\n| 克制 | 嫦娥 |\n| 被克制 | 盘古 |"
const MD_TABLE_GLUE = /\|\|/g
```

以及表格管道前后的空格补齐。

### 3.2 CSS 样式升级（P1 — 已实施）

采用 **typographic table** 风格（横线分隔，无竖线），类似传统印刷表格。同时升级了标题层级（H1/H2/H3 差异化字号）、列表行距、引用块和代码块样式。

**表格核心策略**：`thead` 底部分隔线 2px（强调表头），`tbody tr` 底部分隔线 1px（行间横线），最后一行无线。表头使用次要文字色 + 略小字号，正文行偶数行微灰背景。

```css
/* 表格：typographic horizontal-rule style */
.agent-dock__msg-md table {
  width: 100%;
  border-collapse: collapse;
  margin: 0.5em 0;
  font-size: 0.92em;
}
.agent-dock__msg-md thead {
  border-bottom: 2px solid rgba(60, 60, 67, 0.18);
}
.agent-dock__msg-md th {
  padding: 6px 10px 7px;
  font-weight: 650;
  color: var(--color-text-secondary);
  font-size: 0.92em;
}
.agent-dock__msg-md td {
  padding: 6px 10px 7px;
  line-height: 1.5;
}
.agent-dock__msg-md tbody tr {
  border-bottom: 1px solid rgba(60, 60, 67, 0.08);
}
.agent-dock__msg-md tbody tr:last-child {
  border-bottom: none;
}
.agent-dock__msg-md tr:nth-child(even) td {
  background: rgba(60, 60, 67, 0.02);
}
```

**标题层级**：H1 1.15em / H2 1.08em / H3-H4 1.02em，形成清晰视觉层级。

### 3.3 markdown-it 插件建议（P2）

当前仅使用 `markdown-it` 基础能力。可考虑：

| 插件 | 作用 | 建议 |
|------|------|------|
| `markdown-it-multimd-table` | 增强表格（colspan、对齐） | P2 备选 |
| `highlight.js` + `markdown-it-highlightjs` | 代码语法高亮 | P2 备选 |
| 自定义 `markdown-it` rule | `||` → 换行拆分 | **已通过 normalize 实现，无需** |

### 3.4 安全审计

当前 `DOMPurify` 配置 `{ USE_PROFILES: { html: true } }` + `markdown-it` 的 `html: false` 是安全的，保持不变。

---

## 四、实施优先级总览

| # | 改动项 | 文件 | 优先级 |
|---|--------|------|--------|
| 1 | System Prompt 强化表格规范 | `promptmd.go` | P0 |
| 2 | normalizeKBMarkdown 增加表格修复 | `normalizeKBMarkdown.ts` | P0 |
| 3 | CSS 补充表格/代码块样式 | `AgentDock.vue` | P1 |
| 4 | 后端 mdfix 后处理包 | 新包 `chat/mdfix` | P2 |

---

## 五、验证方法

修改后使用日志中的相同 query ("推荐一个战士") 触发 kb_only 模式，验证：

1. H2 标题格式为 `## xxx`（空格存在）
2. 表格每行独立，包含完整分隔行
3. 前端渲染表格有边框、斑马纹、横向滚动
4. 暗色主题下表格可读性正常

---

## 六、长期建议

1. **结构化输出**：对于 kb_only 模式，考虑用 JSON Schema constrained 输出替代自由 Markdown，彻底消除格式问题
2. **表格专用 prompt 模板**：在 `chat/mode.go` 中检测到查询意图为对比/多维度时，使用更严格的表格输出模板
3. **前端 markdown 预览组件**：提供 Agent 输出预览 + 一键修正格式的功能入口

