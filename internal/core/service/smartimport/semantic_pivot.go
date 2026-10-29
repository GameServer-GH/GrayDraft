package smartimport

import (
	"fmt"
	"strings"

	"context"

	"github.com/graydraft/core/model"
	"github.com/graydraft/core/service/promptmd"
	"github.com/graydraft/core/service/taxonomyevolve"
	"github.com/graydraft/core/service/textutil"
)

// SemanticTags4D is retrieval-oriented document metadata (domain / entity / scene / style).
type SemanticTags4D struct {
	Domain   string   `json:"domain"`
	Entities []string `json:"entities"`
	Scene    string   `json:"scene"`
	Style    string   `json:"style"`
}

// VirtualOutlineSection is one logical block for header injection.
type VirtualOutlineSection struct {
	Level     int    `json:"level"` // 2 or 3
	Title     string `json:"title"`
	StartLine int    `json:"start_line"` // 1-based inclusive
}

type semanticPivotLLMResponse struct {
	Domain              string                  `json:"domain"`
	Entities            []string                `json:"entities"`
	Scene               string                  `json:"scene"`
	Style               string                  `json:"style"`
	GlobalSummary       string                  `json:"global_summary"`
	StructuredMarkdown  string                  `json:"structured_markdown"`
	Outline             []VirtualOutlineSection `json:"outline"`
}

// SemanticPivotResult is the semantic restructuring output for one import body.
type SemanticPivotResult struct {
	Tags4D             SemanticTags4D
	GlobalSummary      string
	StructuredMarkdown string
	Outline            []VirtualOutlineSection
}

const pivotSnippetMaxRunes = 12000

// RunSemanticPivot asks Fast LLM to extract 4D tags, summary, and optional re-headed markdown.
// workspaceBackground is used to resolve domain-scoped taxonomy (YAML catalog).
func RunSemanticPivot(content string, workspaceBackground string, complete func(string) (string, error)) (SemanticPivotResult, error) {
	if complete == nil {
		return SemanticPivotResult{}, fmt.Errorf("llm not available")
	}
	snippet := strings.TrimSpace(content)
	if len([]rune(snippet)) > pivotSnippetMaxRunes {
		snippet = string([]rune(snippet)[:pivotSnippetMaxRunes]) + "…"
	}
	tax := taxonomyevolve.Effective(context.Background(), "", nil, workspaceBackground)
	prompt := fmt.Sprintf(`你是 GrayDraft「语义中轴」助手。根据下列正文（可能来自 VLM/OCR，排版不可靠），完成语义重构元数据提取。

任务 A — 4D 标签（用于检索过滤，须基于正文，勿编造）：
- domain：领域（优先与工作区一致，如 %s）
- entities：核心实体名列表（角色、产品、活动名等，最多 8 个）
- scene、style：须从下列字典择一

%s

任务 B — global_summary：全文 80–200 字中文摘要。

任务 C — 二选一（优先 structured_markdown）：
1. structured_markdown：按下列 Markdown 规范重排正文（可空则走 outline）。
2. 若全文很短或已是良好结构，structured_markdown 可为空，改填 outline：段落列表，每项含 level(2|3)、title、start_line(1-based)。

%s

约束：不要 Emoji；不要 JSON 代码块围栏；禁止「多图要点汇总」等空泛标题。

正文：
%s

只输出一行 JSON：
{"domain":"","entities":[],"scene":"","style":"","global_summary":"","structured_markdown":"","outline":[]}`, tax.Domain, tax.GeneratePromptBlock(), promptmd.SceneStructuredMarkdown, snippet)

	out, err := complete(prompt)
	if err != nil {
		return SemanticPivotResult{}, err
	}
	var parsed semanticPivotLLMResponse
	if err := parseJSONFromLLM(out, &parsed); err != nil {
		return SemanticPivotResult{}, err
	}
	domain := textutil.StripEmojis(strings.TrimSpace(parsed.Domain))
	if domain == "" {
		domain = tax.Domain
	}
	tags := SemanticTags4D{
		Domain:   domain,
		Scene:    tax.NormalizeScene(textutil.StripEmojis(strings.TrimSpace(parsed.Scene))),
		Style:    tax.NormalizeStyle(textutil.StripEmojis(strings.TrimSpace(parsed.Style))),
		Entities: cleanEntityList(parsed.Entities, snippet),
	}
	summary := textutil.StripEmojis(strings.TrimSpace(parsed.GlobalSummary))
	md := strings.TrimSpace(parsed.StructuredMarkdown)
	outline := normalizeOutline(parsed.Outline, len(strings.Split(content, "\n")))
	return SemanticPivotResult{
		Tags4D:             tags,
		GlobalSummary:      summary,
		StructuredMarkdown: md,
		Outline:            outline,
	}, nil
}

func cleanEntityList(in []string, ground string) []string {
	var cleaned []string
	for _, e := range in {
		cleaned = append(cleaned, textutil.StripEmojis(strings.TrimSpace(e)))
	}
	out := model.FinalSanitize(cleaned, ground)
	if len(out) > 8 {
		out = out[:8]
	}
	return out
}

func normalizeOutline(sections []VirtualOutlineSection, lineCount int) []VirtualOutlineSection {
	if lineCount <= 0 {
		return nil
	}
	var out []VirtualOutlineSection
	for _, s := range sections {
		title := textutil.StripEmojis(strings.TrimSpace(s.Title))
		if title == "" || s.StartLine < 1 || s.StartLine > lineCount {
			continue
		}
		level := s.Level
		if level < 2 {
			level = 2
		}
		if level > 3 {
			level = 3
		}
		out = append(out, VirtualOutlineSection{Level: level, Title: title, StartLine: s.StartLine})
	}
	return out
}

// ApplySemanticPivot mutates req.RawText when restructuring succeeds and writes extensions.
func ApplySemanticPivot(req *model.SmartImportRequest, pivot SemanticPivotResult) {
	if req.Extensions == nil {
		req.Extensions = model.Extensions{}
	}
	if pivot.Tags4D.Domain != "" || pivot.Tags4D.Scene != "" || pivot.Tags4D.Style != "" || len(pivot.Tags4D.Entities) > 0 {
		req.Extensions["semantic_tags_4d"] = map[string]any{
			"domain":   pivot.Tags4D.Domain,
			"entities": pivot.Tags4D.Entities,
			"scene":    pivot.Tags4D.Scene,
			"style":    pivot.Tags4D.Style,
		}
	}
	if pivot.GlobalSummary != "" {
		req.Extensions["global_summary"] = pivot.GlobalSummary
	}
	body := strings.TrimSpace(req.RawText)
	if md := strings.TrimSpace(pivot.StructuredMarkdown); len([]rune(md)) >= 24 {
		req.RawText = md
		req.Extensions["semantic_pivot"] = "structured_markdown"
	} else if len(pivot.Outline) > 0 && body != "" {
		req.RawText = injectVirtualOutline(body, pivot.Outline)
		req.Extensions["semantic_pivot"] = "virtual_outline"
	}
	if _, ok := req.Extensions["semantic_pivot"]; ok {
		if len(pivot.Outline) > 0 {
			req.Extensions["virtual_outline"] = pivot.Outline
		}
	}
	applyIngestChunkStrategyAfterPivot(req)
}

// applyIngestChunkStrategyAfterPivot maps semantic restructuring to Worker chunking (section by ##).
func applyIngestChunkStrategyAfterPivot(req *model.SmartImportRequest) {
	if req == nil {
		return
	}
	if req.Extensions == nil {
		req.Extensions = model.Extensions{}
	}
	if s, _ := req.Extensions["ingest_chunk_strategy"].(string); strings.TrimSpace(s) == "fact_table" {
		return
	}
	kind, _ := req.Extensions["semantic_pivot"].(string)
	switch strings.TrimSpace(kind) {
	case "structured_markdown", "virtual_outline":
		req.Extensions["ingest_chunk_strategy"] = "section"
	}
}

func injectVirtualOutline(raw string, outline []VirtualOutlineSection) string {
	if len(outline) == 0 {
		return raw
	}
	lines := strings.Split(raw, "\n")
	type insert struct {
		at    int
		block string
	}
	var ins []insert
	for _, sec := range outline {
		idx := sec.StartLine - 1
		if idx < 0 || idx >= len(lines) {
			continue
		}
		prefix := strings.Repeat("#", sec.Level)
		if prefix == "" {
			prefix = "##"
		}
		title := strings.TrimSpace(sec.Title)
		if title == "" {
			continue
		}
		// Skip if line already is a heading with same text.
		trim := strings.TrimSpace(lines[idx])
		if strings.HasPrefix(trim, "#") && strings.Contains(trim, title) {
			continue
		}
		ins = append(ins, insert{at: idx, block: prefix + " " + title})
	}
	if len(ins) == 0 {
		return raw
	}
	// Insert from bottom to top to preserve indices.
	for i := 0; i < len(ins); i++ {
		for j := i + 1; j < len(ins); j++ {
			if ins[j].at < ins[i].at {
				ins[i], ins[j] = ins[j], ins[i]
			}
		}
	}
	for _, x := range ins {
		at := x.at
		block := x.block
		lines = append(lines[:at], append([]string{block, ""}, lines[at:]...)...)
	}
	return strings.TrimSpace(strings.Join(lines, "\n"))
}

// MergePivotExtensions copies semantic pivot fields into segment extensions for ingest.
func MergePivotExtensions(base model.Extensions, docExt model.Extensions) model.Extensions {
	out := cloneExtensions(base)
	if docExt == nil {
		return out
	}
	for _, key := range []string{"semantic_tags_4d", "global_summary", "semantic_pivot", "virtual_outline"} {
		if v, ok := docExt[key]; ok {
			out[key] = v
		}
	}
	return out
}
