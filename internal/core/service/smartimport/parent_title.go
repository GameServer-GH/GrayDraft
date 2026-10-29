package smartimport

import (
	"regexp"
	"strings"
	"unicode/utf8"
)

var (
	reSegmentSuffix = regexp.MustCompile(`[（(]\s*\d+\s*[/／]\s*\d+\s*[）)]\s*$`)
	reTrailingNoise = regexp.MustCompile(`[\s·\-—|/：:]+$`)
)

var titleStemSeparators = []string{" · ", "：", ":", " - ", " — ", "｜", "|", " / "}

// SummarizeParentTitleFromChildTitles derives a group parent title from segment document titles.
func SummarizeParentTitleFromChildTitles(titles []string) string {
	cleaned := make([]string, 0, len(titles))
	seen := map[string]bool{}
	for _, t := range titles {
		t = normalizeChildDocTitle(t)
		if t == "" || seen[t] {
			continue
		}
		seen[t] = true
		cleaned = append(cleaned, t)
	}
	switch len(cleaned) {
	case 0:
		return ""
	case 1:
		return cleaned[0]
	}

	if stem := commonTitleStem(cleaned); stem != "" {
		return stem
	}
	if p := longestCommonPrefix(cleaned); utf8.RuneCountInString(p) >= 2 {
		return trimParentTitleSuffix(p)
	}
	if s := shortestPrefixOfAll(cleaned); s != "" {
		return s
	}
	return cleaned[0]
}

func normalizeChildDocTitle(title string) string {
	title = strings.TrimSpace(title)
	if title == "" {
		return ""
	}
	title = reSegmentSuffix.ReplaceAllString(title, "")
	title = strings.TrimSpace(title)
	return title
}

func commonTitleStem(titles []string) string {
	stems := make([]string, 0, len(titles))
	for _, t := range titles {
		stems = append(stems, titleStem(t))
	}
	first := stems[0]
	if first == "" {
		return ""
	}
	for _, s := range stems[1:] {
		if s != first {
			return ""
		}
	}
	return first
}

func titleStem(title string) string {
	title = normalizeChildDocTitle(title)
	if title == "" {
		return ""
	}
	best := title
	for _, sep := range titleStemSeparators {
		if i := strings.Index(title, sep); i > 0 {
			part := strings.TrimSpace(title[:i])
			if part != "" && (best == title || len([]rune(part)) < len([]rune(best))) {
				best = part
			}
		}
	}
	return trimParentTitleSuffix(best)
}

func longestCommonPrefix(titles []string) string {
	if len(titles) == 0 {
		return ""
	}
	prefix := titles[0]
	for _, t := range titles[1:] {
		for !strings.HasPrefix(t, prefix) {
			if prefix == "" {
				return ""
			}
			_, size := utf8.DecodeLastRuneInString(prefix)
			prefix = prefix[:len(prefix)-size]
		}
	}
	return trimParentTitleSuffix(prefix)
}

func shortestPrefixOfAll(titles []string) string {
	shortest := titles[0]
	for _, t := range titles[1:] {
		if len([]rune(t)) < len([]rune(shortest)) {
			shortest = t
		}
	}
	for _, t := range titles {
		if t == shortest || strings.HasPrefix(t, shortest) {
			continue
		}
		return ""
	}
	return trimParentTitleSuffix(shortest)
}

func trimParentTitleSuffix(s string) string {
	s = strings.TrimSpace(s)
	s = reTrailingNoise.ReplaceAllString(s, "")
	return strings.TrimSpace(s)
}
