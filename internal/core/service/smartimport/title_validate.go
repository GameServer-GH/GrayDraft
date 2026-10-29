package smartimport

import (
	"regexp"
	"strings"

	"github.com/graydraft/core/service/multimodal"
)

var (
	reCodeFenceLine = regexp.MustCompile("^`{3}")
	reBareJSON      = regexp.MustCompile(`(?i)^json$`)
	// reListMarker matches markdown ordered list prefixes: "1.", "2)", "1、", etc.
	reListMarker = regexp.MustCompile(`^\d+[.)、]\s`)
)

func isInvalidTitle(title string) bool {
	t := strings.TrimSpace(title)
	if t == "" {
		return true
	}
	lower := strings.ToLower(t)
	if reBareJSON.MatchString(lower) {
		return true
	}
	if reCodeFenceLine.MatchString(t) {
		return true
	}
	if strings.HasPrefix(t, "{") || strings.HasPrefix(t, "[") {
		return true
	}
	if strings.HasPrefix(t, "## 附件") || strings.HasPrefix(t, "附件:") {
		return true
	}
	if multimodal.LooksLikeAttachmentFilename(t) {
		return true
	}
	if multimodal.IsGenericConsolidatedTitle(t) {
		return true
	}
	// Strip leading markdown list markers ("- ", "* ", "1. ") that
	// leak from VLM-extracted content into the title.
	if strings.HasPrefix(t, "- ") || strings.HasPrefix(t, "* ") {
		return true
	}
	if reListMarker.MatchString(t) {
		return true
	}
	if len([]rune(t)) < 2 {
		return true
	}
	// User import instructions are context, not document titles.
	if isImportIntentNoteTitle(t) {
		return true
	}
	return false
}

func isImportIntentNoteTitle(title string) bool {
	return len([]rune(title)) <= 80 && strings.Contains(title, "梳理")
}
