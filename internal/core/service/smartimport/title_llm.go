package smartimport

import (
	"fmt"
	"strings"

	"github.com/graydraft/core/service/multimodal"
	"github.com/graydraft/core/service/textutil"
)

type llmTitleResponse struct {
	Title string `json:"title"`
}

// InferTitleLLM asks the model for a document title (no rule-based title heuristics).
func InferTitleLLM(content, userNote string, attachmentNames []string, workspaceBackground string, complete func(string) (string, error)) (string, error) {
	if complete == nil {
		return "", fmt.Errorf("llm not available")
	}
	snippet := strings.TrimSpace(content)
	if len([]rune(snippet)) > 6000 {
		snippet = string([]rune(snippet)[:6000]) + "…"
	}
	userNote = strings.TrimSpace(userNote)
	if multimodal.IsImportIntentNote(userNote) {
		// keep as context only
	} else if userNote != "" && len([]rune(userNote)) <= 120 {
		// long user text may be a real title candidate — still pass as note
	}

	var names []string
	for _, n := range attachmentNames {
		n = strings.TrimSpace(n)
		if n != "" {
			names = append(names, n)
		}
	}
	namesBlock := strings.Join(names, "\n")
	if namesBlock == "" {
		namesBlock = "(无)"
	}

	bgBlock := "(无)"
	if b := strings.TrimSpace(workspaceBackground); b != "" {
		bgBlock = b
	}
	prompt := fmt.Sprintf(`你是 GrayDraft 知识库入库助手。请根据下列信息生成一条「文档标题」（用于知识库列表展示）。

工作区背景（用于消歧义与主题归纳，可体现在标题中）：
%s

要求：
1. 标题 8–48 字，概括附件与正文的核心主题（概念、API、参数、模型或章节）。
2. 可参考附件文件名中的文档名和章节名；不要照搬文件名中的序号、作者、平台后缀。
3. 用户的「导入说明」（如「梳理…」「整理…」）只是操作意图，不能当作标题。
4. 不要编造正文中完全不存在的专名；可适度归纳（如「InternVL 视觉输入说明」）。
5. 禁止使用泛化占位标题，例如「多图要点汇总」「要点汇总」「多图汇总」「图片汇总」。
6. 若正文是参数表、接口列表或课件页，标题须包含具体模型、接口或章节，不要只写「汇总」。
7. 不要使用 Emoji；不要加书名号外的引号包裹整句。
8. 只输出一行 JSON：{"title":"..."}

用户导入说明：
%s

附件文件名：
%s

正文（节选）：
%s`, bgBlock, orDefault(userNote, "(无)"), namesBlock, orDefault(snippet, "(无)"))

	out, err := complete(prompt)
	if err != nil {
		return "", err
	}
	var parsed llmTitleResponse
	if err := parseJSONFromLLM(out, &parsed); err != nil {
		return "", err
	}
	title := textutil.StripEmojis(strings.TrimSpace(parsed.Title))
	if title == "" || multimodal.IsImportIntentNote(title) || multimodal.IsGenericConsolidatedTitle(title) {
		return "", fmt.Errorf("llm returned empty or invalid title")
	}
	if len([]rune(title)) > 120 {
		title = string([]rune(title)[:120])
	}
	return title, nil
}

func orDefault(s, def string) string {
	if strings.TrimSpace(s) == "" {
		return def
	}
	return s
}

func attachmentNamesFromResults(results []multimodal.ExtractionResult) []string {
	names := make([]string, 0, len(results))
	for _, r := range results {
		if r.Filename != "" {
			names = append(names, r.Filename)
		}
	}
	return names
}
