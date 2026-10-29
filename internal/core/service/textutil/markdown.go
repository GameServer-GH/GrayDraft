package textutil

import (
	"fmt"
	"regexp"
	"strings"
	"unicode"
)

var (
	mdDateRangeRe        = regexp.MustCompile(`\d{2}\.\d{2}(?:\s*[-–—~～]\s*\d{2}\.\d{2})?`)
	mdHeadingSpaceRe     = regexp.MustCompile(`(?m)(^#{1,6})([^\s#\n])`)
	mdHeadingGlueRe      = regexp.MustCompile(`([^\n#\s])(#{1,6}\s*)`)
	mdHeadingTableGlueRe = regexp.MustCompile(`(?m)^(#{1,6}\s+[^\n|]+)\|`)
	mdNumDotLooseRe      = regexp.MustCompile(`(\d{1,2})\.([^\s\n])`)
	mdNumListGlueRe      = regexp.MustCompile(`([^\n\d])(\d{1,2})\.(\s*)`)
	mdNumAfterBoldRe     = regexp.MustCompile(`(\*\*)(\d{1,2})\.(\s*)`)
	mdBulletGlueRe       = regexp.MustCompile(`([^\n\s\-*\d.])(-\s+)`)
	mdBulletAfterBoldRe  = regexp.MustCompile(`(\*\*)(-)([^\s\n])`)
	mdBulletBoldGlueRe   = regexp.MustCompile(`([^\n\s])(-)(\*\*)`)
	mdExcessNewlinesRe   = regexp.MustCompile(`\n{3,}`)
)

// NormalizeKBMarkdown fixes common LLM Markdown spacing issues (glued headings, lists, bullets).
func NormalizeKBMarkdown(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return s
	}
	s, dates := shieldDates(s)
	s = mdHeadingSpaceRe.ReplaceAllString(s, `$1 $2`)
	s = mdHeadingGlueRe.ReplaceAllString(s, "$1\n\n$2")
	s = fixLooseListDots(s)
	s = fixNumListGlue(s)
	s = fixNumAfterBold(s)
	s = mdBulletAfterBoldRe.ReplaceAllString(s, "$1\n$2 $3")
	s = mdBulletGlueRe.ReplaceAllString(s, "$1\n$2")
	s = mdBulletBoldGlueRe.ReplaceAllString(s, "$1\n$2 $3")
	s = normalizeMarkdownTables(s)
	s = mdExcessNewlinesRe.ReplaceAllString(s, "\n\n")
	s = unshieldDates(strings.TrimSpace(s), dates)
	return strings.TrimSpace(s)
}

// fixLooseListDots turns "1.**" into "1. **" but keeps "06.19" intact.
func fixLooseListDots(s string) string {
	return mdNumDotLooseRe.ReplaceAllStringFunc(s, func(m string) string {
		sub := mdNumDotLooseRe.FindStringSubmatch(m)
		if len(sub) < 3 {
			return m
		}
		next := sub[2]
		if next != "" && unicode.IsDigit(rune(next[0])) {
			return m
		}
		return sub[1] + ". " + next
	})
}

func fixNumListGlue(s string) string {
	return mdNumListGlueRe.ReplaceAllStringFunc(s, func(m string) string {
		sub := mdNumListGlueRe.FindStringSubmatch(m)
		if len(sub) < 4 {
			return m
		}
		return sub[1] + "\n\n" + sub[2] + "." + sub[3]
	})
}

func fixNumAfterBold(s string) string {
	return mdNumAfterBoldRe.ReplaceAllStringFunc(s, func(m string) string {
		sub := mdNumAfterBoldRe.FindStringSubmatch(m)
		if len(sub) < 4 {
			return m
		}
		return sub[1] + "\n\n" + sub[2] + "." + sub[3]
	})
}

func shieldDates(s string) (string, []string) {
	var dates []string
	out := mdDateRangeRe.ReplaceAllStringFunc(s, func(d string) string {
		dates = append(dates, d)
		return fmt.Sprintf("\x00DATE%d\x00", len(dates)-1)
	})
	return out, dates
}

func unshieldDates(s string, dates []string) string {
	for i, d := range dates {
		s = strings.ReplaceAll(s, fmt.Sprintf("\x00DATE%d\x00", i), d)
	}
	return s
}

func normalizeMarkdownTables(s string) string {
	s = mdHeadingTableGlueRe.ReplaceAllString(s, "$1\n\n|")
	s = splitGluedMarkdownTableRows(s)
	return canonicalizeMarkdownTableBlocks(s)
}

func splitGluedMarkdownTableRows(s string) string {
	lines := strings.Split(s, "\n")
	for i, line := range lines {
		trim := strings.TrimSpace(line)
		// Find table start within line — may be preceded by text
		tableStart := strings.Index(trim, "|")
		if tableStart < 0 || !strings.Contains(trim[tableStart:], "||") || strings.Count(trim[tableStart:], "|") < 4 {
			continue
		}
		// indent is leading whitespace (preserved for subsequent rows)
		indent := line[:len(line)-len(trim)]
		tablePart := trim[tableStart:]
		sep := ""
		if tableStart > 0 {
			// Insert newline so table starts on its own line (isMarkdownTableLine
			// requires lines to start with |).
			sep = "\n"
		}
		lines[i] = line[:len(line)-len(trim)+tableStart] + sep +
			strings.ReplaceAll(tablePart, "||", "|\n"+indent+"|")
	}
	return strings.Join(lines, "\n")
}

func canonicalizeMarkdownTableBlocks(s string) string {
	lines := strings.Split(s, "\n")
	var out []string
	for i := 0; i < len(lines); {
		if !isMarkdownTableLine(lines[i]) {
			out = append(out, lines[i])
			i++
			continue
		}
		start := i
		for i < len(lines) && isMarkdownTableLine(lines[i]) {
			i++
		}
		block := lines[start:i]
		if normalized, ok := normalizeMarkdownTableBlock(block); ok {
			if len(out) > 0 && strings.TrimSpace(out[len(out)-1]) != "" {
				out = append(out, "")
			}
			out = append(out, normalized...)
			if i < len(lines) && strings.TrimSpace(lines[i]) != "" {
				out = append(out, "")
			}
			continue
		}
		out = append(out, block...)
	}
	return strings.Join(out, "\n")
}

func isMarkdownTableLine(line string) bool {
	trim := strings.TrimSpace(line)
	return strings.HasPrefix(trim, "|") && strings.Count(trim, "|") >= 2
}

func normalizeMarkdownTableBlock(lines []string) ([]string, bool) {
	if len(lines) < 2 {
		return nil, false
	}
	rows := make([][]string, 0, len(lines))
	separatorIdx := -1
	width := 0
	for _, line := range lines {
		cells := parseMarkdownTableCells(line)
		if len(cells) < 2 {
			return nil, false
		}
		if isMarkdownSeparatorRow(cells) && separatorIdx == -1 {
			separatorIdx = len(rows)
		}
		if len(cells) > width {
			width = len(cells)
		}
		rows = append(rows, cells)
	}
	if separatorIdx == 0 {
		return nil, false
	}
	if separatorIdx == -1 && !tableRowsConsistent(rows) {
		return nil, false
	}
	if width < 2 {
		return nil, false
	}
	header := rows[0]
	data := rows[1:]
	if separatorIdx == 1 {
		data = rows[2:]
	}
	out := []string{
		formatMarkdownTableRow(header, width),
		formatMarkdownSeparatorRow(width),
	}
	for _, row := range data {
		if isMarkdownSeparatorRow(row) {
			continue
		}
		out = append(out, formatMarkdownTableRow(row, width))
	}
	return out, true
}

func parseMarkdownTableCells(line string) []string {
	trim := strings.TrimSpace(line)
	trim = strings.TrimPrefix(trim, "|")
	trim = strings.TrimSuffix(trim, "|")
	parts := strings.Split(trim, "|")
	cells := make([]string, 0, len(parts))
	for _, p := range parts {
		cells = append(cells, strings.TrimSpace(p))
	}
	return cells
}

func isMarkdownSeparatorRow(cells []string) bool {
	if len(cells) == 0 {
		return false
	}
	for _, c := range cells {
		c = strings.TrimSpace(c)
		if c == "" {
			return false
		}
		hasDash := false
		for _, r := range c {
			switch r {
			case '-', ':', ' ':
				if r == '-' {
					hasDash = true
				}
			default:
				return false
			}
		}
		if !hasDash {
			return false
		}
	}
	return true
}

func tableRowsConsistent(rows [][]string) bool {
	if len(rows) < 2 {
		return false
	}
	width := len(rows[0])
	for _, row := range rows[1:] {
		if len(row) != width {
			return false
		}
	}
	return true
}

func formatMarkdownTableRow(cells []string, width int) string {
	padded := make([]string, width)
	for i := 0; i < width; i++ {
		if i < len(cells) {
			padded[i] = strings.TrimSpace(cells[i])
		}
	}
	return "| " + strings.Join(padded, " | ") + " |"
}

func formatMarkdownSeparatorRow(width int) string {
	cells := make([]string, width)
	for i := range cells {
		cells[i] = "---"
	}
	return "| " + strings.Join(cells, " | ") + " |"
}
