package smartimport

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	"github.com/graydraft/core/model"
	"github.com/graydraft/core/service/textutil"
)

var (
	reMarkdownH1 = regexp.MustCompile(`(?m)^#\s+(.+)$`)
)

// Draft is the analyzed document fields before dedup.
type Draft struct {
	KBType      string
	Title       string
	Content     string
	ContentHash string
	MimeType    string
	Confidence  float64
	Method      string
	Reason      string
}

// Analyze extracts title and normalized content; kb_type must be workspace partition (gd_*).
func Analyze(raw, filename, mimeType, hintKBType, workspaceBackground string, useLLM bool, llmComplete func(prompt string) (string, error), ext model.Extensions) (Draft, error) {
	content := normalizeRaw(raw)
	if len([]rune(content)) < 10 {
		return Draft{}, fmt.Errorf("content too short (min 10 runes)")
	}
	if d, ok := draftFromUnifiedCache(ext, content, filename, mimeType); ok {
		return d, nil
	}

	mime := inferMime(filename, mimeType)
	method := "rules"
	title := ""
	if useLLM && llmComplete != nil && useLegacyImportLLM(ext) {
		if t, err := InferTitleLLM(content, "", nil, workspaceBackground, llmComplete); err == nil {
			if !isGenericFranchiseTitle(t, franchiseFromContent(content)) {
				title = t
				method = "llm"
			}
		}
	}
	if title == "" {
		title = extractTitle(content)
	}

	kbType, conf, reason := ResolveKBType(hintKBType)
	if kbType == "" {
		return Draft{}, fmt.Errorf("%w", ErrWorkspaceKBTypeRequired)
	}

	if useLLM && llmComplete != nil && useLegacyImportLLM(ext) {
		if refined, err := refineTitleWithLLM(content, title, llmComplete); err == nil {
			t := strings.TrimSpace(refined.Title)
			if t != "" && titleAppearsInContent(t, content) && !isGenericFranchiseTitle(t, franchiseFromContent(content)) {
				title = t
				if method != "llm" {
					method = "rules+llm"
				}
			}
		}
	}

	if title == "" {
		title = fallbackTitle(content)
	}
	if subj := inferSubjectTitle(content); subj != "" {
		if title == "" || isGenericFranchiseTitle(title, franchiseFromContent(content)) {
			title = subj
		}
	}

	hash := sha256.Sum256([]byte(content))
	return Draft{
		KBType:      kbType,
		Title:       title,
		Content:     content,
		ContentHash: hex.EncodeToString(hash[:]),
		MimeType:    mime,
		Confidence:  conf,
		Method:      method,
		Reason:      reason,
	}, nil
}

func normalizeRaw(s string) string {
	s = strings.TrimSpace(strings.ReplaceAll(s, "\r\n", "\n"))
	s = strings.ReplaceAll(s, "\r", "\n")
	return textutil.StripEmojis(strings.TrimSpace(s))
}

func inferMime(filename, mime string) string {
	if mime != "" {
		return mime
	}
	lower := strings.ToLower(filename)
	switch {
	case strings.HasSuffix(lower, ".md"), strings.HasSuffix(lower, ".markdown"):
		return "text/markdown"
	case strings.HasSuffix(lower, ".html"), strings.HasSuffix(lower, ".htm"):
		return "text/html"
	default:
		return "text/plain"
	}
}

func extractTitle(content string) string {
	if m := reMarkdownH1.FindStringSubmatch(content); len(m) > 1 {
		t := trimTitleRunes(strings.TrimSpace(m[1]), 120)
		if !isInvalidTitle(t) {
			return t
		}
	}
	if subj := inferSubjectTitle(content); subj != "" && !isInvalidTitle(subj) {
		return subj
	}
	for _, line := range strings.Split(content, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if m := reMarkdownAnyH.FindStringSubmatch(line); len(m) > 2 {
			t := strings.TrimSpace(m[2])
			if !isInvalidTitle(t) {
				return trimTitleRunes(t, 120)
			}
			continue
		}
		r := []rune(line)
		if len(r) >= 4 && len(r) <= 120 && !isInvalidTitle(line) {
			return string(r)
		}
	}
	return ""
}

func fallbackTitle(content string) string {
	r := []rune(strings.ReplaceAll(content, "\n", " "))
	if len(r) > 80 {
		return string(r[:80]) + "…"
	}
	return string(r)
}

type llmDraft struct {
	Title      string  `json:"title"`
	Confidence float64 `json:"confidence"`
	Reason     string  `json:"reason"`
}

func refineTitleWithLLM(content, title string, complete func(string) (string, error)) (llmDraft, error) {
	snippet := content
	if len([]rune(snippet)) > 4000 {
		snippet = string([]rune(snippet)[:4000])
	}
	prompt := fmt.Sprintf(`你是 GrayDraft 知识库入库助手。仅根据正文提炼标题（不要判断 kb_type）。
规则猜测 title=%q

要求：
1. title 必须从正文已有文字截取，禁止编造；若无合适标题则返回空字符串。
2. 输出中不要使用 Emoji。

正文：
%s

只输出一行 JSON：{"title":"","confidence":0.8,"reason":""}`, title, snippet)

	out, err := complete(prompt)
	if err != nil {
		return llmDraft{}, err
	}
	out = strings.TrimSpace(out)
	if i := strings.Index(out, "{"); i >= 0 {
		out = out[i:]
	}
	if j := strings.LastIndex(out, "}"); j >= 0 {
		out = out[:j+1]
	}
	var d llmDraft
	if err := json.Unmarshal([]byte(out), &d); err != nil {
		return llmDraft{}, err
	}
	d.Title = textutil.StripEmojis(strings.TrimSpace(d.Title))
	return d, nil
}

func titleAppearsInContent(title, content string) bool {
	t := strings.TrimSpace(title)
	if t == "" {
		return false
	}
	if strings.Contains(content, t) {
		return true
	}
	for _, line := range strings.Split(content, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if strings.HasPrefix(line, t) || strings.HasPrefix(t, line) {
			return true
		}
		if len([]rune(t)) >= 8 && strings.Contains(line, string([]rune(t)[:minInt(8, len([]rune(t)))])) {
			return true
		}
		break
	}
	// LCS ratio check (third matching condition per optimization plan 2.1)
	if titleLCSMatch(title, content) {
		return true
	}
	return false
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}
