package smartimport

import (
	"strings"

	"github.com/graydraft/core/service/multimodal"
)

// ExtractByLineRange returns inclusive 1-based line slice from normalized raw text.
func ExtractByLineRange(raw string, start, end int) string {
	raw = normalizeRaw(raw)
	lines := strings.Split(raw, "\n")
	if len(lines) == 0 {
		return ""
	}
	if start < 1 {
		start = 1
	}
	if end > len(lines) {
		end = len(lines)
	}
	if start > end || start > len(lines) {
		return ""
	}
	return strings.TrimSpace(strings.Join(lines[start-1:end], "\n"))
}

func segmentTitleFromRange(raw string, start, end int, analysis DocAnalysis) string {
	content := ExtractByLineRange(raw, start, end)
	for _, line := range strings.Split(content, "\n") {
		trim := strings.TrimSpace(line)
		if m := reMarkdownAnyH.FindStringSubmatch(trim); len(m) > 2 {
			t := strings.TrimSpace(m[2])
			if !isInvalidTitle(t) {
				return t
			}
		}
	}
	if t := inferSubjectTitle(content); t != "" {
		return t
	}
	if t := extractTitle(content); t != "" && !isGenericFranchiseTitle(t, analysis.Subject) {
		return t
	}
	if analysis.ParentTitle != "" && !isGenericFranchiseTitle(analysis.ParentTitle, analysis.Subject) {
		return analysis.ParentTitle
	}
	return fallbackTitle(content)
}

func displayTitle(parent, segment string) string {
	parent = strings.TrimSpace(parent)
	segment = strings.TrimSpace(segment)
	if segment == "" {
		return parent
	}
	if parent == "" || strings.Contains(segment, parent) {
		return segment
	}
	return parent + " · " + segment
}

func isAttachmentFilenameOnlySegment(content string) bool {
	content = strings.TrimSpace(content)
	if content == "" {
		return true
	}
	var heading string
	bodyRunes := 0
	for _, line := range strings.Split(content, "\n") {
		trim := strings.TrimSpace(line)
		if trim == "" {
			continue
		}
		if strings.HasPrefix(trim, "### ") || strings.HasPrefix(trim, "## ") {
			heading = strings.TrimSpace(strings.TrimLeft(trim, "# "))
			continue
		}
		bodyRunes += len([]rune(trim))
	}
	if bodyRunes >= 40 {
		return false
	}
	if heading != "" && multimodal.LooksLikeAttachmentFilename(heading) {
		return true
	}
	return multimodal.LooksLikeAttachmentFilename(content)
}

func isClosingFluff(content string) bool {
	trimmed := strings.TrimSpace(content)
	if len([]rune(trimmed)) >= 200 {
		return false
	}
	for _, phrase := range []string{"以上就是", "全文完", "欢迎转载", "转载请注明出处"} {
		if strings.Contains(trimmed, phrase) {
			return len([]rune(trimmed)) < 120
		}
	}
	return false
}
