// Package smartimport provides markdown-to-natural-language conversion for ingest.
// Stripping happens AFTER analysis is complete and BEFORE chunk embedding,
// ensuring LLM splitting retains structural signals while stored text is pure NL.
package smartimport

import (
	"regexp"
	"strings"
)

var (
	reMdHeading     = regexp.MustCompile(`(?m)^#{1,6}\s+`)
	reMdBold        = regexp.MustCompile(`\*{1,2}([^*]+)\*{1,2}`)
	reMdItalic      = regexp.MustCompile(`(?m)(?:^|\s)_([^_\n]+)_(?:\s|$)`)
	reMdLink        = regexp.MustCompile(`\[([^\]]*)\]\([^)]*\)`)
	reMdImage       = regexp.MustCompile(`!\[[^\]]*\]\([^)]*\)`)
	reMdCodeInline  = regexp.MustCompile("`([^`\n]+)`")
	reMdCodeBlock   = regexp.MustCompile("(?s)```[^`]*```")
	reMdHr          = regexp.MustCompile(`(?m)^\s*[-=*_]{3,}\s*$`)
	reMdBlockquote  = regexp.MustCompile(`(?m)^>\s?`)
	reMdListBullet  = regexp.MustCompile(`(?m)^\s*[-*+]\s+`)
	reMdListNumber  = regexp.MustCompile(`(?m)^\s*\d+[.)]\s+`)
	reMdTableRow    = regexp.MustCompile(`(?m)^\|.*\|$`)
	reMediaURLLine  = regexp.MustCompile(`(?m)^media_url:\s*/api/v1/multimodal/media/[a-f0-9]+\s*$`)
	reMultiNewline  = regexp.MustCompile(`\n{3,}`)
	reHTMLTag       = regexp.MustCompile(`<[^>]*>`)
)

// CleanMarkdownForIngest converts markdown-formatted text to pure natural language
// suitable for vector embedding and knowledge retrieval.
//
// Transformations:
//   - ### heading → heading
//   - | col1 | col2 | → col1：val1，col2：val2 (table rows → KV natural language)
//   - **bold** → bold
//   - [text](url) → text
//   - `code` → code
//   - ```block``` → removed
//   - --- → removed
//   - > quote → quote
//   - - list item → list item
//   - HTML tags → removed
func CleanMarkdownForIngest(raw string) string {
	if raw == "" {
		return ""
	}

	s := raw

	// Remove code blocks first (can contain misleading patterns)
	s = reMdCodeBlock.ReplaceAllString(s, "")

	// Remove images
	s = reMdImage.ReplaceAllString(s, "")

	// Remove horizontal rules
	s = reMdHr.ReplaceAllString(s, "")

	// Remove HTML tags
	s = reHTMLTag.ReplaceAllString(s, "")

	// Remove multimodal media ref lines (internal metadata, not knowledge content)
	s = reMediaURLLine.ReplaceAllString(s, "")

	// Convert tables to natural language
	s = convertMarkdownTables(s)

	// Remove heading markers, keep text
	s = reMdHeading.ReplaceAllString(s, "")

	// Remove bold/italic markers, keep text
	s = reMdBold.ReplaceAllString(s, "$1")
	s = reMdItalic.ReplaceAllString(s, " $1 ")

	// Remove link syntax, keep text
	s = reMdLink.ReplaceAllString(s, "$1")

	// Remove inline code markers, keep text
	s = reMdCodeInline.ReplaceAllString(s, "$1")

	// Remove blockquote markers
	s = reMdBlockquote.ReplaceAllString(s, "")

	// Remove list markers
	s = reMdListBullet.ReplaceAllString(s, "")
	s = reMdListNumber.ReplaceAllString(s, "")

	// Normalize whitespace
	s = reMultiNewline.ReplaceAllString(s, "\n\n")
	s = strings.TrimSpace(s)

	return s
}

// convertMarkdownTables converts markdown table rows to natural language KV format.
// Header row is used to prefix values: | Name | ATK | → Name：Excalibur，ATK：150
// Separator rows (---|---) are skipped.
func convertMarkdownTables(s string) string {
	lines := strings.Split(s, "\n")
	var result []string
	i := 0

	for i < len(lines) {
		line := strings.TrimSpace(lines[i])
		if !reMdTableRow.MatchString(line) {
			result = append(result, lines[i])
			i++
			continue
		}

		// Collect consecutive table rows
		var tableLines []string
		for i < len(lines) && reMdTableRow.MatchString(strings.TrimSpace(lines[i])) {
			tableLines = append(tableLines, strings.TrimSpace(lines[i]))
			i++
		}

		if len(tableLines) < 2 {
			// Single-row table, treat as regular line
			result = append(result, strings.Trim(tableLines[0], "|"))
			continue
		}

		// Parse header
		headers := splitTableCells(tableLines[0])

		// Skip separator row if present
		startRow := 1
		if startRow < len(tableLines) && isTableSeparator(tableLines[startRow]) {
			startRow++
		}

		// Convert each data row
		for r := startRow; r < len(tableLines); r++ {
			cells := splitTableCells(tableLines[r])
			var parts []string
			for ci, cell := range cells {
				cell = strings.TrimSpace(cell)
				if cell == "" {
					continue
				}
				if ci < len(headers) {
					h := strings.TrimSpace(headers[ci])
					if h != "" {
						parts = append(parts, h+"："+cell)
					} else {
						parts = append(parts, cell)
					}
				} else {
					parts = append(parts, cell)
				}
			}
			if len(parts) > 0 {
				result = append(result, strings.Join(parts, "，"))
			}
		}
	}

	return strings.Join(result, "\n")
}

// splitTableCells splits a markdown table row into cells.
func splitTableCells(row string) []string {
	row = strings.Trim(row, "|")
	parts := strings.Split(row, "|")
	cells := make([]string, 0, len(parts))
	for _, p := range parts {
		cells = append(cells, strings.TrimSpace(p))
	}
	return cells
}

// isTableSeparator checks if a table row is a separator (e.g., |---|---|).
func isTableSeparator(row string) bool {
	row = strings.Trim(row, "|")
	parts := strings.Split(row, "|")
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		// Check if the cell is all dashes, colons, and optional leading/trailing colons
		cleaned := strings.Trim(p, " :")
		cleaned = strings.ReplaceAll(cleaned, "-", "")
		if cleaned != "" {
			return false
		}
	}
	return true
}

// ── Knowledge text normalization pipeline (optimization plan 8.4) ────────────

var (
	reFullWidthDigit = strings.NewReplacer(
		"０", "0", "１", "1", "２", "2", "３", "3", "４", "4",
		"５", "5", "６", "6", "７", "7", "８", "8", "９", "9",
	)
	reFullWidthAlpha = strings.NewReplacer(
		"Ａ", "A", "Ｂ", "B", "Ｃ", "C", "Ｄ", "D", "Ｅ", "E",
		"Ｆ", "F", "Ｇ", "G", "Ｈ", "H", "Ｉ", "I", "Ｊ", "J",
		"Ｋ", "K", "Ｌ", "L", "Ｍ", "M", "Ｎ", "N", "Ｏ", "O",
		"Ｐ", "P", "Ｑ", "Q", "Ｒ", "R", "Ｓ", "S", "Ｔ", "T",
		"Ｕ", "U", "Ｖ", "V", "Ｗ", "W", "Ｘ", "X", "Ｙ", "Y",
		"Ｚ", "Z", "ａ", "a", "ｂ", "b", "ｃ", "c", "ｄ", "d",
		"ｅ", "e", "ｆ", "f", "ｇ", "g", "ｈ", "h", "ｉ", "i",
		"ｊ", "j", "ｋ", "k", "ｌ", "l", "ｍ", "m", "ｎ", "n",
		"ｏ", "o", "ｐ", "p", "ｑ", "q", "ｒ", "r", "ｓ", "s",
		"ｔ", "t", "ｕ", "u", "ｖ", "v", "ｗ", "w", "ｘ", "x",
		"ｙ", "y", "ｚ", "z",
	)
	reURLStrict = regexp.MustCompile(`https?://[^\s\n]{4,}`)
	reHTMLCharEntity = map[string]string{
		"&amp;":  "&",
		"&lt;":   "<",
		"&gt;":   ">",
		"&quot;": "\"",
		"&apos;": "'",
		"&#39;":  "'",
		"&nbsp;": " ",
	}
)

// NormalizeKnowledgeText performs the final text normalization pipeline
// before knowledge ingestion. Handles:
//   - HTML entity decoding
//   - URL removal
//   - Full-width → half-width conversion
//   - Zero-width and control character removal
//   - Consecutive newline compression
func NormalizeKnowledgeText(s string) string {
	if s == "" {
		return ""
	}

	// HTML entity decoding
	for entity, decoded := range reHTMLCharEntity {
		s = strings.ReplaceAll(s, entity, decoded)
	}

	// URL removal
	s = reURLStrict.ReplaceAllString(s, "")

	// Remove multimodal media ref lines (belt-and-suspenders with CleanMarkdownForIngest)
	s = reMediaURLLine.ReplaceAllString(s, "")

	// Full-width digit conversion
	s = reFullWidthDigit.Replace(s)
	// Full-width alpha conversion
	s = reFullWidthAlpha.Replace(s)

	// Zero-width characters
	s = strings.ReplaceAll(s, "\u200b", "") // ZERO WIDTH SPACE
	s = strings.ReplaceAll(s, "\u200c", "") // ZERO WIDTH NON-JOINER
	s = strings.ReplaceAll(s, "\u200d", "") // ZERO WIDTH JOINER
	s = strings.ReplaceAll(s, "\ufeff", "") // BYTE ORDER MARK / ZWNBSP

	// Remove C0/C1 control characters except \n and \t
	var result []rune
	for _, r := range s {
		if r < 0x20 && r != '\n' && r != '\t' {
			continue
		}
		if r >= 0x7f && r < 0xa0 {
			continue
		}
		result = append(result, r)
	}
	s = string(result)

	// Compress 3+ consecutive newlines to 2
	s = reMultiNewline.ReplaceAllString(s, "\n\n")

	return strings.TrimSpace(s)
}
