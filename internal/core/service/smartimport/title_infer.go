package smartimport

import (
	"regexp"
	"strings"
)

var (
	// 「InternVL（视觉模型）是书生的…」→ InternVL
	reSubjectBeforeIs = regexp.MustCompile(`(?m)^\s*([\p{Han}A-Za-z0-9·_-]{2,24})(?:（[^）\n]{0,40}）|\([^)\n]{0,40}\))?\s*是`)
)

// inferSubjectTitle picks a specific topic name from prose (hero, feature, concept).
func inferSubjectTitle(content string) string {
	content = normalizeRaw(content)
	if m := reMarkdownH1.FindStringSubmatch(content); len(m) > 1 {
		t := trimTitleRunes(strings.TrimSpace(m[1]), 120)
		if !isInvalidTitle(t) {
			return t
		}
	}
	line := firstNonEmptyLine(content)
	if line == "" {
		return ""
	}
	if m := reSubjectBeforeIs.FindStringSubmatch(line); len(m) > 1 {
		t := strings.TrimSpace(m[1])
		if len([]rune(t)) >= 2 && !isGenericFranchiseTitle(t, franchiseFromContent(content)) {
			return t
		}
	}
	return ""
}

func firstNonEmptyLine(content string) string {
	for _, line := range strings.Split(content, "\n") {
		if t := strings.TrimSpace(line); t != "" {
			return t
		}
	}
	return ""
}

func franchiseFromContent(content string) string {
	if m := reBookTitle.FindStringSubmatch(content); len(m) > 1 {
		return strings.TrimSpace(m[1])
	}
	return ""
}

func isGenericFranchiseTitle(title, primaryGame string) bool {
	title = strings.TrimSpace(title)
	if title == "" {
		return false
	}
	if primaryGame != "" && title == primaryGame {
		return true
	}
	return false
}

func trimTitleRunes(s string, max int) string {
	r := []rune(strings.TrimSpace(s))
	if len(r) > max {
		return string(r[:max])
	}
	return string(r)
}

// preferDocumentTitle chooses the best document-level title for a segment.
func preferDocumentTitle(raw string, a DocAnalysis) string {
	if t := inferSubjectTitle(raw); t != "" {
		return t
	}
	if t := extractTitle(raw); t != "" && !isGenericFranchiseTitle(t, a.Subject) {
		return t
	}
	if t := strings.TrimSpace(a.ParentTitle); t != "" && !isGenericFranchiseTitle(t, a.Subject) {
		return t
	}
	return fallbackTitle(raw)
}

func resolvePreviewTitle(segTitle, content string, analysis DocAnalysis) string {
	t := strings.TrimSpace(segTitle)
	if t != "" && isGenericFranchiseTitle(t, analysis.Subject) {
		if subj := inferSubjectTitle(content); subj != "" {
			t = subj
		}
	}
	if t == "" {
		t = preferDocumentTitle(content, analysis)
	}
	if t != "" && (titleAppearsInContent(t, content) || strings.Contains(t, "·")) {
		return t
	}
	if subj := inferSubjectTitle(content); subj != "" {
		return subj
	}
	if t := strings.TrimSpace(analysis.ParentTitle); t != "" && !isGenericFranchiseTitle(t, analysis.Subject) {
		return t
	}
	return ""
}
