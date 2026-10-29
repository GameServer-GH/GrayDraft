// Package promptmd holds reusable Markdown output instructions for LLM prompts.
package promptmd

// StandardBlock is the generic four-part MD output spec (role, format, content, negative).
const StandardBlock = `# 角色
你是精通专业文档排版的助手。根据任务要求输出结构清晰、逻辑严密、排版规范的 Markdown。

# 格式规范
1. 严格使用标准 Markdown（# 标题、**加粗**、- 列表、` + "`行内代码`" + `、> 引用、表格等）。
2. 层级清晰：H1 (#) 作总标题，H2 (##)、H3 (###) 作子章节；标题符号后必须有空格。
3. 关键信息用 **加粗** 或 ` + "`行内代码`" + ` 标注；参数对比、多维数据优先用 Markdown 表格。
4. 在输出前于内部梳理信息优先级，仅保留最高优先级内容（勿写出推理过程）。
5. 表格必须使用标准 Markdown：表格前后各空一行；表头、分隔行、每条数据行都必须独占一行；单元格写作 ` + "`| 列名 | 列名 |`" + `，分隔行写作 ` + "`| --- | --- |`" + `。严禁把表格压成一行，严禁使用 ` + "`||`" + ` 连接多行。

# 内容约束
1. 仅基于给定材料，严禁虚构或误导。
2. 极致简洁：禁止废话、开场白（如「好的，这是…」）与结尾套话。
3. 段首先结论后细节（Topic Sentence）；每个列表项尽量不超过 20 个汉字。

# 负向约束
- 严禁输出无 MD 标记的纯文本。
- 严禁冗余解释性文字。
- Skip the preamble. 直接从第一个 Markdown 标题或正文结构开始输出。
- 严禁破坏语法（如 # 后漏空格、标题与正文粘在同一行）。
- 严禁输出不完整表格（缺少分隔行、行尾缺 |、多行粘连）。
`

// GradeDBlock is a lighter MD output spec for Grade D (no KB hits).
// It allows a coverage preamble and removes reference-material constraints.
const GradeDBlock = `# 角色
你是精通专业文档排版的助手。知识库中未检索到相关内容，请基于通用知识输出规范 Markdown。

# 格式规范
1. 严格使用标准 Markdown（# 标题、**加粗**、- 列表、` + "`行内代码`" + `、> 引用、表格等）。
2. 层级清晰：H1 (#) 作总标题，H2 (##)、H3 (###) 作子章节；标题符号后必须有空格。
3. 关键信息用 **加粗** 或 ` + "`行内代码`" + ` 标注；参数对比、多维数据优先用 Markdown 表格。
4. 表格必须使用标准 Markdown：表格前后各空一行；表头、分隔行、每条数据行都必须独占一行；单元格写作 ` + "`| 列名 | 列名 |`" + `，分隔行写作 ` + "`| --- | --- |`" + `。严禁把表格压成一行，严禁使用 ` + "`||`" + ` 连接多行。

# 内容约束
1. 首句可包含覆盖声明，之后直接进入正题；禁止无关废话与结尾套话。
2. 极致简洁，每个列表项尽量不超过 20 个汉字。

# 负向约束
- 严禁破坏语法（如 # 后漏空格、标题与正文粘在同一行）。
- 严禁输出不完整表格（缺少分隔行、行尾缺 |、多行粘连）。
`


// LineBreakRules is the GrayDraft kb_only numbered-list layout (headings + ordered + bullets).
const LineBreakRules = `Markdown 换行与列表排版（必须遵守）：
- 标题独占一行，# 后留空格，标题后空一行。
- 每条编号（1. 2. …）独占一行；编号行内可用 **加粗** 作小标题。
- 每个 - 列表项独占一行，写在对应编号下方。
- 不同编号条目之间空一行。
- Markdown 表格每一行必须以 | 开头并以 | 结尾，表头下一行必须是 | --- | --- | 形式的分隔行。

格式示例：
### 新手快速升级指南

1. **优先推主线**
- 主线任务是前期经验的主要来源。

2. **集中资源培养主力**
- 经验果优先喂给主力宠物。
`

// TableRules is a standalone table-formatting instruction block for kb_only / chat modes.
// It provides explicit BAD/GOOD examples to stop the LLM from collapsing table rows into one line.
const TableRules = `Markdown 表格格式化（极其重要！绝对禁止 || 粘连）：
- 表头、分隔行、数据行必须各占独立的一行，绝对禁止用 || 把多行接在一起。
- 每个单元格前后各加一个空格（| 列A | 列B |），分隔行写 | --- | --- |。
- 表格前后各空一行。

GOOD 示例（正确）：

| 类型 | 英雄 |
| ---- | ---- |
| 克制 | 嫦娥、戈娅、上官婉儿 |
| 被克制 | 盘古、刘禅、蒙恬 |

BAD 示例（绝对禁止！）：

克制关系|类型|英雄||---|---||克制|嫦娥||被克制|盘古|
`

// SceneContentExtract is for direct MD extraction (multimodal VLM/OCR).
const SceneContentExtract = `【场景：内容提炼】
仅输出 Markdown 正文。不要回答问题，不要解释逻辑，不要 JSON 代码块围栏。
`

// SceneStructuredMarkdown is for structured_markdown inside JSON (import / semantic pivot).
const SceneStructuredMarkdown = `【场景：结构化正文 — structured_markdown 字段】
- 在逻辑段落前补 ## 或 ### 标题，标题须概括该段语义；保留已有正确标题；不删正文事实。
- 涉及步骤时用 H3 分步，每步不超过 50 字；有示例时用代码块展示。
- 对比、参数表优先 Markdown 表格；表头概念加粗，单元格内用 - 精炼分点。
`

// SceneOCRExtract is appended when OCR/VLM must recover tables and forms as MD.
const SceneOCRExtract = `【场景：文档 OCR】
重点提取表格、表单、键值对与长文。表格必须用 Markdown 表格呈现；键值对可用列表。
可输出 JSON {summary,text_content,structured_kv}；若选 Markdown 则须符合上述格式规范。
`

// SceneChainVLM is appended for multi-image sequential extraction.
const SceneChainVLM = `结合前序图理解当前图（装备与属性、列表与详情等）；在 summary 中点明关联。
输出可为 JSON（含 summary、text_content、可选 structured_kv）或规范 Markdown；选 Markdown 时遵守格式规范。
`
