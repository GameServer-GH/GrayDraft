package multimodal

import (
	"path/filepath"
	"regexp"
	"strings"
)

var (
	reFileExtSuffix = regexp.MustCompile(`\.(jpe?g|png|gif|webp|bmp|heic|tiff?|pdf|mp4|mov|avi|mkv|webm)(?:[)\]】」》\s]|$)`)
	reNumberedStem  = regexp.MustCompile(`_\d+_`)
)

// genericImageStems are stems that should never be used as document titles
// because they are obviously machine-generated attachment filenames.
var genericImageStems = map[string]bool{
	"image": true, "photo": true, "picture": true, "img": true, "pic": true,
	"screenshot": true, "screen": true, "capture": true, "snapshot": true,
	"attachment": true, "file": true, "download": true, "upload": true,
	"未命名": true, "unnamed": true, "untitled": true, "无标题": true,
}

// LooksLikeAttachmentFilename reports upload names that must not be used as document titles or split headings.
func LooksLikeAttachmentFilename(s string) bool {
	s = strings.TrimSpace(s)
	if s == "" {
		return false
	}
	lower := strings.ToLower(s)
	base := strings.TrimSpace(filepath.Base(lower))
	if base != "" {
		lower = base
	}
	if reFileExtSuffix.MatchString(lower) {
		return true
	}
	if strings.Contains(s, "_来自") && (strings.Contains(s, "小红书") || strings.Contains(s, "网页版")) {
		return true
	}
	if reNumberedStem.MatchString(s) && len([]rune(s)) > 24 {
		return true
	}
	// Reject stems that are obviously machine-generated (e.g., "image" from "image_2.webp")
	if genericImageStems[lower] {
		return true
	}
	return false
}

// PerImageSectionTitle picks a human-readable section title (never the raw upload filename).
func PerImageSectionTitle(formattedText string, index int, filename string) string {
	if t := firstSemanticHeading(formattedText); t != "" && !LooksLikeAttachmentFilename(t) {
		return t
	}
	if t := strings.TrimSpace(firstMeaningfulLine(formattedText)); t != "" && !LooksLikeAttachmentFilename(t) {
		if len([]rune(t)) <= 80 {
			return t
		}
	}
	_ = filename
	if index > 0 {
		return "图 " + itoa(index)
	}
	return "图 1"
}

func firstSemanticHeading(text string) string {
	for _, line := range strings.Split(text, "\n") {
		trim := strings.TrimSpace(line)
		if strings.HasPrefix(trim, "### ") {
			t := strings.TrimSpace(strings.TrimPrefix(trim, "### "))
			if t != "" {
				return t
			}
		}
	}
	return ""
}

func itoa(n int) string {
	if n <= 0 {
		return "1"
	}
	var digits []byte
	for n > 0 {
		digits = append([]byte{byte('0' + n%10)}, digits...)
		n /= 10
	}
	return string(digits)
}
