package multimodal

import (
	"regexp"
	"strings"
)

var (
	reCodeFenceTitle = regexp.MustCompile("^`{3,}")
	reBareJSONTitle  = regexp.MustCompile(`(?i)^json$`)
)

// SanitizeDocumentTitle strips code-fence/json artifacts from LLM or parser output.
func SanitizeDocumentTitle(title string) string {
	title = strings.TrimSpace(title)
	if title == "" || isInvalidSanitizedTitle(title) {
		return ""
	}
	return title
}

func isInvalidSanitizedTitle(title string) bool {
	if reBareJSONTitle.MatchString(strings.ToLower(title)) {
		return true
	}
	if reCodeFenceTitle.MatchString(title) {
		return true
	}
	if strings.HasPrefix(title, "{") || strings.HasPrefix(title, "[") {
		return true
	}
	if LooksLikeAttachmentFilename(title) {
		return true
	}
	if IsGenericConsolidatedTitle(title) {
		return true
	}
	return len([]rune(title)) < 2
}
