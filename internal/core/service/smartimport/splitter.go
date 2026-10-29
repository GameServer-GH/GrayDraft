package smartimport

import (
	"regexp"
	"strings"
)

var (
	reSplitSep   = regexp.MustCompile(`(?m)^\s*[-=*#]{3,}\s*$`)
	reMarkdownH2 = regexp.MustCompile(`(?m)^#{1,3}\s+.+$`)
	reQALine     = regexp.MustCompile(`(?m)^\s*(Q[:：]|问[:：]|A[:：]|答[:：])`)
)

// Segment is one knowledge piece split from raw text.
type Segment struct {
	Content string
}

// SplitRawText splits messy/long text into importable segments (2.0 coordinate plan, rules-only).
func SplitRawText(raw string) []Segment {
	raw = normalizeRaw(raw)
	if raw == "" {
		return nil
	}
	plan := PlanSegments(raw, false, nil, false, nil)
	if len(plan.Segments) == 0 {
		return nil
	}
	out := make([]Segment, 0, len(plan.Segments))
	for _, p := range plan.Segments {
		out = append(out, Segment{Content: p.Content})
	}
	return out
}

func splitBySeparators(raw string) []string {
	return splitByLinePredicate(raw, func(line string) bool {
		return reSplitSep.MatchString(line)
	})
}

func splitByMarkdownHeadings(raw string) []string {
	return splitByLinePredicate(raw, func(line string) bool {
		return reMarkdownH2.MatchString(line) && !strings.HasPrefix(strings.TrimSpace(line), "# ")
	})
}

func splitByLinePredicate(raw string, isBreak func(string) bool) []string {
	lines := strings.Split(raw, "\n")
	var blocks []string
	var cur []string
	flush := func() {
		if len(cur) > 0 {
			blocks = append(blocks, strings.Join(cur, "\n"))
			cur = nil
		}
	}
	for _, line := range lines {
		if isBreak(line) {
			flush()
			continue
		}
		cur = append(cur, line)
	}
	flush()
	return blocks
}

func splitByParagraphs(raw string) []string {
	parts := strings.Split(raw, "\n\n")
	var blocks []string
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if len([]rune(p)) >= 40 {
			blocks = append(blocks, p)
		}
	}
	return blocks
}
