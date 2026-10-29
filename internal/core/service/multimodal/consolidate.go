package multimodal

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
)

// factRow is a domain-neutral row for multi-image consolidation.
type factRow struct {
	Subject  string
	Window   string
	KeyPoint string
	Detail   string
	Extra    string
	Notes    string
}

var (
	reSubjectAlt   = regexp.MustCompile(`(?:主题|条目|名称|对象|版本发布)[：:]\s*([^\s\n，,。.]+)`)
	reDateWindow   = regexp.MustCompile(`(\d{2}\.\d{2})\s*[-–—~～]\s*(\d{2}\.\d{2})`)
	reDateRangeCN  = regexp.MustCompile(`(\d{4}[-/年]\d{1,2}[-/月]\d{1,2})\s*[-–—~～至到]\s*(\d{4}[-/年]?\d{1,2}[-/月]?\d{1,2})`)
	reMaterials    = regexp.MustCompile(`(?:材料|合成材料|所需|依赖)[：:]\s*(.+)`)
	reRecommend    = regexp.MustCompile(`(?:推荐|建议)[：:]\s*([^\n]+)`)
	reAttr         = regexp.MustCompile(`(?:属性|类型|适用|易捕捉)\s*([^\n]+)`)
	reBallName     = regexp.MustCompile(`([^\s\n]{2,10}球)`)
)

// IsImportIntentNote detects short user instructions (not document titles).
func IsImportIntentNote(s string) bool {
	s = strings.TrimSpace(s)
	if s == "" {
		return false
	}
	runes := len([]rune(s))
	if runes > 120 {
		return false
	}
	if strings.Contains(s, "「") || strings.Contains(s, "《") || strings.Contains(s, "》") {
		return false
	}
	intentKW := []string{"梳理", "汇总", "上传", "导入", "帮我", "请", "附件", "图片", "识别", "分析"}
	hasIntent := false
	for _, k := range intentKW {
		if strings.Contains(s, k) {
			hasIntent = true
			break
		}
	}
	if !hasIntent {
		return runes <= 24
	}
	return runes <= 48
}

// ExtractUserImportNote returns the user's brief import instruction (excludes attachment footers).
func ExtractUserImportNote(userRaw string) string {
	userRaw = strings.TrimSpace(userRaw)
	if userRaw == "" {
		return ""
	}
	var lines []string
	for _, line := range strings.Split(userRaw, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "（已附加") {
			continue
		}
		if strings.HasPrefix(line, "## 附件") {
			break
		}
		lines = append(lines, line)
	}
	note := strings.TrimSpace(strings.Join(lines, "\n"))
	if note == "" || !IsImportIntentNote(note) {
		if len(lines) == 1 {
			return lines[0]
		}
		return ""
	}
	return note
}

// ConsolidateIngestMarkdown merges multi-image extractions into one fact table body.
func ConsolidateIngestMarkdown(userRaw string, results []ExtractionResult) (body string, ok bool) {
	_ = userRaw
	if len(results) < 2 {
		return "", false
	}
	combined := combineFormattedExtractions(results)
	rows := extractFactRows(combined)
	rows = dedupeFactRows(rows)
	if len(rows) < 2 {
		return "", false
	}
	return buildFactMarkdownTable(rows), true
}

// CombinePreviewForTitle builds a short text snippet for LLM title inference.
func CombinePreviewForTitle(userNote string, results []ExtractionResult) string {
	var b strings.Builder
	if note := ExtractUserImportNote(userNote); note != "" {
		b.WriteString("用户说明: ")
		b.WriteString(note)
		b.WriteString("\n\n")
	}
	b.WriteString(combineFormattedExtractions(results))
	return strings.TrimSpace(b.String())
}

func combineFormattedExtractions(results []ExtractionResult) string {
	var parts []string
	for _, r := range results {
		t := FormatExtractionText(r.Text)
		if t != "" {
			parts = append(parts, t)
		}
	}
	return strings.Join(parts, "\n\n")
}

func extractFactRows(text string) []factRow {
	text = FormatExtractionText(text)
	if text == "" {
		return nil
	}
	if obj, ok := tryParseJSONObject(text); ok {
		if formatted := parseModelOutputJSONFields(obj); formatted != "" {
			text = formatted
		}
	}
	if rows := extractFactRowsPaired(text); len(rows) > 0 {
		return rows
	}
	var out []factRow
	for _, block := range splitFactBlocks(text) {
		if row := parseFactBlock(block); row.Subject != "" {
			out = append(out, row)
		}
	}
	if len(out) > 0 {
		return out
	}
	return parseFactRowsLinear(text)
}

// extractFactRowsPaired binds each 版本发布 label to the nearest date window in text.
func extractFactRowsPaired(text string) []factRow {
	type anchor struct {
		pos     int
		subject string
	}
	var anchors []anchor
	for _, m := range reSubjectAlt.FindAllStringSubmatchIndex(text, -1) {
		if len(m) < 4 {
			continue
		}
		anchors = append(anchors, anchor{pos: m[0], subject: strings.TrimSpace(text[m[2]:m[3]])})
	}
	if len(anchors) == 0 {
		return nil
	}
	windows := reDateWindow.FindAllStringSubmatchIndex(text, -1)
	var out []factRow
	for i, a := range anchors {
		end := len(text)
		if i+1 < len(anchors) {
			end = anchors[i+1].pos
		}
		seg := text[a.pos:end]
		row := factRow{Subject: a.subject}
		if m := reDateWindow.FindStringSubmatch(seg); len(m) > 2 {
			row.Window = m[1] + " - " + m[2]
		}
		if m := reAttr.FindStringSubmatch(seg); len(m) > 1 {
			row.Detail = strings.TrimSpace(m[1])
		}
		if m := reMaterials.FindStringSubmatch(seg); len(m) > 1 {
			row.Extra = strings.TrimSpace(m[1])
		}
		if m := reBallName.FindStringSubmatch(seg); len(m) > 1 {
			row.KeyPoint = strings.TrimSpace(m[1])
		}
		if row.KeyPoint == "" {
			if m := reSubjectAlt.FindStringSubmatch(seg); len(m) > 1 {
				row.KeyPoint = "版本发布：" + strings.TrimSpace(m[1])
			}
		}
		out = append(out, row)
	}
	// date-only segments without 版本发布 (fallback)
	if len(out) < len(windows) {
		for _, w := range windows {
			if len(w) < 6 {
				continue
			}
			win := text[w[2]:w[3]] + " - " + text[w[4]:w[5]]
			found := false
			for _, r := range out {
				if r.Window == win {
					found = true
					break
				}
			}
			if !found {
				out = append(out, factRow{Window: win})
			}
		}
	}
	return out
}

func splitFactBlocks(text string) []string {
	parts := regexp.MustCompile(`(?m)^#{1,3}\s+`).Split(text, -1)
	var blocks []string
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			blocks = append(blocks, p)
		}
	}
	if len(blocks) <= 1 {
		return []string{text}
	}
	return blocks
}

func parseFactBlock(block string) factRow {
	var row factRow
	if m := reSubjectAlt.FindStringSubmatch(block); len(m) > 1 {
		row.Subject = strings.TrimSpace(m[1])
	} else {
		line := firstMeaningfulLine(block)
		if line != "" && !isGenericSectionTitle(line) {
			row.Subject = line
		}
	}
	if m := reDateWindow.FindStringSubmatch(block); len(m) > 2 {
		row.Window = m[1] + " - " + m[2]
	}
	if m := reRecommend.FindStringSubmatch(block); len(m) > 1 {
		row.KeyPoint = strings.TrimSpace(m[1])
	}
	if m := reAttr.FindStringSubmatch(block); len(m) > 1 {
		row.Detail = strings.TrimSpace(m[1])
	}
	if m := reMaterials.FindStringSubmatch(block); len(m) > 1 {
		row.Extra = strings.TrimSpace(m[1])
	}
	return row
}

func isGenericSectionTitle(line string) bool {
	generic := []string{"活动", "图鉴", "日历", "说明", "整理", "拆解"}
	for _, g := range generic {
		if strings.Contains(line, g) && len([]rune(line)) > 12 {
			return true
		}
	}
	return false
}

func parseFactRowsLinear(text string) []factRow {
	var out []factRow
	subjects := reSubjectAlt.FindAllStringSubmatch(text, -1)
	windows := reDateWindow.FindAllStringSubmatch(text, -1)
	n := len(subjects)
	if n == 0 {
		return nil
	}
	for i := 0; i < n; i++ {
		row := factRow{Subject: strings.TrimSpace(subjects[i][1])}
		if i < len(windows) {
			row.Window = windows[i][1] + " - " + windows[i][2]
		}
		out = append(out, row)
	}
	return out
}

func dedupeFactRows(rows []factRow) []factRow {
	bySubject := map[string]factRow{}
	for _, r := range rows {
		key := r.Subject
		if key == "" {
			key = r.Window
		}
		if key == "" {
			continue
		}
		prev, ok := bySubject[key]
		if !ok {
			bySubject[key] = r
			continue
		}
		if factRowRichness(r) > factRowRichness(prev) {
			bySubject[key] = mergeFactRow(prev, r)
		}
	}
	out := make([]factRow, 0, len(bySubject))
	for _, r := range bySubject {
		out = append(out, r)
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].Window < out[j].Window
	})
	return out
}

func factRowRichness(r factRow) int {
	n := 0
	if r.Window != "" {
		n++
	}
	if r.KeyPoint != "" {
		n++
	}
	if r.Detail != "" {
		n++
	}
	if r.Extra != "" {
		n++
	}
	return n
}

func mergeFactRow(a, b factRow) factRow {
	out := a
	if out.Subject == "" {
		out.Subject = b.Subject
	}
	if out.Window == "" {
		out.Window = b.Window
	}
	if out.KeyPoint == "" {
		out.KeyPoint = b.KeyPoint
	}
	if out.Detail == "" {
		out.Detail = b.Detail
	}
	if out.Extra == "" {
		out.Extra = b.Extra
	}
	if out.Notes == "" {
		out.Notes = b.Notes
	}
	return out
}

// subjectsFromColonProse extracts entity names from colon-separated structured
// VLM prose (key：value lines). Unlike extractFactRows which handles markdown
// tables and legacy JSON, this handles free-form structured text like:
//
//   赞妮
//   搭配角色：菲比（效应叠层）、守岸人（辅助）
//   术语配置：此间永驻之光×2
//
// Strategy (no keywords, no hardcoded lists):
//   1. Split into lines, classify each as colon-line or plain-line.
//   2. Plain lines surrounded by ≥2 colon-lines are entity names.
//   3. Filter: 2-16 CJK chars, no ASCII, no punctuation, no numeric-only.
//   4. Deduplicate.
func subjectsFromColonProse(text string) []string {
	lines := strings.Split(text, "\n")

	// Classify lines
	type lineInfo struct {
		text      string
		isColon   bool
	}
	infos := make([]lineInfo, 0, len(lines))
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		hasColon := strings.Contains(line, "：") || strings.Contains(line, ":")
		infos = append(infos, lineInfo{text: line, isColon: hasColon})
	}

	if len(infos) < 3 {
		return nil
	}

	// Must have at least 2 colon-lines to indicate structured data
	colonCount := 0
	for _, info := range infos {
		if info.isColon {
			colonCount++
		}
	}
	if colonCount < 2 {
		return nil
	}

	// Collect plain lines that are between colon-lines as subjects
	seen := map[string]bool{}
	var subjects []string
	for i, info := range infos {
		if info.isColon {
			continue
		}
		// Must be adjacent to at least one colon-line
		prevColon := i > 0 && infos[i-1].isColon
		nextColon := i+1 < len(infos) && infos[i+1].isColon
		if !prevColon && !nextColon {
			continue
		}

		candidate := info.text
		// Filter: only CJK text, no punctuation, 2-16 chars
		if !isValidEntityName(candidate) {
			continue
		}
		if seen[candidate] {
			continue
		}
		seen[candidate] = true
		subjects = append(subjects, candidate)
	}

	if len(subjects) == 0 {
		return nil
	}
	return subjects
}

// isValidEntityName checks if text looks like a CJK entity name
// (character, item, skill name) rather than noise or prose.
func isValidEntityName(s string) bool {
	runes := []rune(s)
	if len(runes) < 2 || len(runes) > 16 {
		return false
	}
	hanCount := 0
	for _, r := range runes {
		if r >= 0x4E00 && r <= 0x9FFF {
			hanCount++
			continue
		}
		// Allow hyphen, dot, CJK punctuation
		if r == '·' || r == '-' || r == '•' {
			continue
		}
		// Reject ASCII letters, digits, punctuation
		return false
	}
	return hanCount >= 2
}

func buildFactMarkdownTable(rows []factRow) string {
	var b strings.Builder
	b.WriteString("| 主题 | 时间/区间 | 要点 | 属性/类型 | 补充 |\n")
	b.WriteString("| :--- | :--- | :--- | :--- | :--- |\n")
	for _, r := range rows {
		b.WriteString(fmt.Sprintf("| %s | %s | %s | %s | %s |\n",
			escapeTableCell(r.Subject),
			escapeTableCell(r.Window),
			escapeTableCell(r.KeyPoint),
			escapeTableCell(r.Detail),
			escapeTableCell(r.Extra),
		))
	}
	return strings.TrimSpace(b.String())
}

func buildPerImageDetailSections(results []ExtractionResult) string {
	var blocks []string
	for i, r := range results {
		text := FormatExtractionText(r.Text)
		if text == "" {
			continue
		}
		title := PerImageSectionTitle(text, i+1, r.Filename)
		block := fmt.Sprintf("### %s\n\n%s", title, text)
		if r.MediaURL != "" {
			block += fmt.Sprintf("\n\nmedia_url: %s", r.MediaURL)
		}
		blocks = append(blocks, block)
	}
	return strings.Join(blocks, "\n\n")
}

func escapeTableCell(s string) string {
	s = strings.ReplaceAll(s, "|", "\\|")
	s = strings.ReplaceAll(strings.TrimSpace(s), "\n", " ")
	return s
}

// IsConsolidatedFactTable reports merged multi-image fact-table markdown.
func IsConsolidatedFactTable(s string) bool {
	s = strings.TrimSpace(s)
	return strings.Contains(s, "| 主题 |") && strings.Contains(s, "时间")
}

func IsConsolidatedSpawnGuide(s string) bool {
	return IsConsolidatedFactTable(s)
}

// ExtractMarkdownH1 returns the first # heading text.
func ExtractMarkdownH1(s string) string {
	for _, line := range strings.Split(s, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "# ") {
			t := strings.TrimSpace(strings.TrimPrefix(line, "# "))
			if IsImportIntentNote(t) {
				return ""
			}
			return t
		}
	}
	return ""
}

// FirstMeaningfulLine returns the first non-empty, non-media-url line from
// VLM-extracted text, trimmed to maxRunes. Used as a fallback title hint
// when InferConsolidatedTitle can't derive a title from structured rows.
func FirstMeaningfulLine(text string, maxRunes int) string {
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		// Skip media_url lines
		if strings.HasPrefix(strings.ToLower(line), "media_url") {
			continue
		}
		// Skip JSON artifacts
		if strings.HasPrefix(line, "{") || strings.HasPrefix(line, "```") {
			continue
		}
		runes := []rune(line)
		if maxRunes > 0 && len(runes) > maxRunes {
			return string(runes[:maxRunes])
		}
		return line
	}
	return ""
}

// FinalizeIngestText merges user note + consolidated table + per-image details for ingest.
// documentTitle should come from LLM (smartimport.InferTitleLLM); if empty, a generic placeholder is used.
func FinalizeIngestText(userRaw string, results []ExtractionResult, documentTitle string) (final string, title string, consolidated bool) {
	table, ok := ConsolidateIngestMarkdown(userRaw, results)
	if !ok || table == "" {
		return "", "", false
	}
	title = SanitizeDocumentTitle(documentTitle)
	if title == "" || IsGenericConsolidatedTitle(title) {
		title = InferConsolidatedTitle(userRaw, results)
	}
	note := ExtractUserImportNote(userRaw)

	var b strings.Builder
	b.WriteString("# ")
	b.WriteString(title)
	b.WriteString("\n\n")
	if note != "" {
		b.WriteString("> **导入说明**：")
		b.WriteString(note)
		b.WriteString("\n\n")
	}
	b.WriteString("## 要点汇总\n\n")
	b.WriteString(table)
	// Per-image blocks use upload filenames as ### headings and create meaningless smart-import
	// segments; consolidated fact table is sufficient for analyze/split/ingest.
	return strings.TrimSpace(b.String()), title, true
}

func firstMeaningfulLine(s string) string {
	for _, line := range strings.Split(s, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.EqualFold(line, "json") {
			continue
		}
		if strings.HasPrefix(line, "{") {
			continue
		}
		return line
	}
	return ""
}
