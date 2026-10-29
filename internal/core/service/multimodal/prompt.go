package multimodal

import (
	"strings"

	"github.com/graydraft/core/service/promptmd"
)

// DefaultBasePrompt is the system-wide multimodal extraction prompt (4.0).
var DefaultBasePrompt = buildDefaultBasePrompt()

func buildDefaultBasePrompt() string {
	var b strings.Builder
	b.WriteString("你是一个多模态内容提取助手。只提取图中可读的事实信息（名称、时间、数值、步骤、关系、条件等）。\n\n")
	b.WriteString("严禁输出：背景色/配色、字体、装饰、水印、图鉴风格、纸张质感、卡通形象描述等任何视觉风格内容。\n")
	b.WriteString("禁止 JSON、禁止 Markdown 代码块围栏（不要输出 json 代码块标记）。\n\n")
	b.WriteString(promptmd.StandardBlock)
	b.WriteString("\n")
	b.WriteString(promptmd.SceneContentExtract)
	b.WriteString(`直接输出 Markdown：
- 第一行用三级标题：### {本图主题}（示例：「产品说明 · 版本 2.0」「活动日程 · 条目 A」——按实际内容命名）
- 其后用列表写出关键字段（时间/区间、主体名称、建议动作、依赖项、备注等）；表格类内容用 Markdown 表格
若无可读文字，仅写一行：（未识别到文字）`)
	return b.String()
}

// ComposePrompt stacks base + workspace override (4.0 §3.1).
func ComposePrompt(base, workspace string) string {
	base = strings.TrimSpace(base)
	if base == "" {
		base = DefaultBasePrompt
	}
	ws := strings.TrimSpace(workspace)
	if ws == "" {
		return base
	}
	return base + "\n\n【工作区补充指令】\n" + ws
}

// ComposePromptWithContext stacks base, workspace override, and structured background.
func ComposePromptWithContext(base, workspace, contextProfile string) string {
	p := ComposePrompt(base, workspace)
	ctx := strings.TrimSpace(contextProfile)
	if ctx == "" {
		return p
	}
	return p + "\n\n【工作区背景】\n" + ctx
}
