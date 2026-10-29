package multimodal

import (
	"path/filepath"
	"regexp"
	"strings"
	"unicode"
)

var (
	reTrailingIndex   = regexp.MustCompile(`[_-]?\d+$`)
	reEventInText     = regexp.MustCompile(`(S\d+\s*[\p{Han}A-Za-z0-9「」]{0,20}(?:接口说明|赛季|活动|版本)?)`)
	reSeasonStem    = regexp.MustCompile(`(?i)^(S\d+[\p{Han}A-Za-z0-9「」]{2,24})`)

	// Time-detection pattern: detects actual time expressions (HH:MM, date
	// spans, durations, periods) rather than matching keywords like
	// "时间"/"日历" which appear in non-temporal contexts too.
	reTimePattern  = regexp.MustCompile(`\d{1,2}[:：]\d{2}|第\d+[天周月赛季]|每[天周日]|\d+[-–—~至到]\d+|(\d+年)?\d{1,2}月\d{1,2}日|\d+[时分钟秒天周月年]|凌晨|上午|下午|晚上|傍晚|午夜|中午`)
)

// IsGenericConsolidatedTitle reports placeholder titles that must not be used for KB documents.
func IsGenericConsolidatedTitle(title string) bool {
	t := strings.TrimSpace(title)
	if t == "" {
		return true
	}
	lower := strings.ToLower(t)
	generic := []string{
		"多图要点汇总", "要点汇总", "多图汇总", "多图整理", "图片汇总", "附件汇总",
		"多图识别", "识别汇总", "内容汇总", "信息汇总",
	}
	for _, g := range generic {
		if t == g || strings.HasPrefix(t, g) && len([]rune(t)) <= len([]rune(g))+4 {
			return true
		}
	}
	if strings.HasPrefix(lower, "图 ") || strings.HasPrefix(t, "图") && len([]rune(t)) <= 4 {
		return true
	}
	// Only meta words, no concrete topic.
	if strings.Contains(t, "多图") && (strings.Contains(t, "汇总") || strings.Contains(t, "要点")) {
		return true
	}
	return false
}

// InferConsolidatedTitle builds a document title from attachments, fact rows, and user note (no LLM).
func InferConsolidatedTitle(userNote string, results []ExtractionResult) string {
	combined := combineFormattedExtractions(results)
	rows := extractFactRows(combined)

	event := extractEventLabel(results, combined)
	suffix := topicSuffixFromNote(userNote, combined, rows)

	if suffix == "" {
		if strings.Contains(combined, "版本发布") {
			suffix = "发布时间表"
		} else if looksTimeRelated(combined) {
			suffix = "时间表"
		}
	}

	var title string
	switch {
	case event != "":
		title = event + suffix
	case len(rows) > 0:
		if subj := sampleSubjectNames(rows, 2); subj != "" {
			title = subj + suffix
		}
	default:
		// Free-form VLM prose without markdown tables or legacy JSON.
		// Extract entity names from colon-separated key:value text
		// (e.g. "赞妮" followed by "搭配角色：菲比" -> entity names).
		if subjects := subjectsFromColonProse(combined); len(subjects) > 0 {
			if suffix == "" {
				// No time suffix needed — entity list alone suffices
				title = strings.Join(subjects[:min2(len(subjects), 3)], "、")
			} else {
				if len(subjects) == 1 {
					title = subjects[0] + suffix
				} else {
					title = subjects[0] + "等" + suffix
				}
			}
		}
	}

	title = strings.TrimSpace(title)
	if title == "" || IsGenericConsolidatedTitle(title) {
		return ""
	}
	if len([]rune(title)) > 48 {
		title = string([]rune(title)[:48])
	}
	return title
}

// looksTimeRelated uses structural and pattern analysis (not keyword matching)
// to determine whether combined text contains scheduling/timeline content.
// Strategy:
//   1. Fact-row column analysis: check if any column header implies time semantics.
//   2. Time-pattern density: ratio of date/time/duration patterns to total tokens.
//   3. Combined signal: both must pass for time classification.
func looksTimeRelated(combined string) bool {
	rows := extractFactRows(combined)

	// 1. Structural analysis: do fact rows have time window fields?
	// The Window field in factRow carries temporal data (e.g., "10:00-12:00").
	// Team composition tables have no Window data — their rows describe
	// character teams, equipment, etc. Time tables always have Window populated.
	hasTimeColumn := false
	for _, r := range rows {
		if strings.TrimSpace(r.Window) != "" {
			hasTimeColumn = true
			break
		}
	}

	// 2. Time-pattern density: actual date/time/duration tokens in the text
	timeDensity := timePatternDensity(combined)

	// Must have either time columns in structured data OR significant time patterns
	return hasTimeColumn || timeDensity >= 0.03
}

// timePatternDensity returns the ratio of time-related tokens to total CJK bigrams.
// Uses regex-based structural detection: HH:MM, date-like strings, duration expressions.
func timePatternDensity(combined string) float64 {
	matches := reTimePattern.FindAllString(combined, -1)
	totalPatterns := len(matches)

	// Count CJK bigrams as a proxy for total semantic tokens
	runes := []rune(combined)
	cjkBigrams := 0
	for i := 0; i < len(runes)-1; i++ {
		a, b := runes[i], runes[i+1]
		if a >= 0x4E00 && a <= 0x9FFF && b >= 0x4E00 && b <= 0x9FFF {
			cjkBigrams++
		}
	}
	if cjkBigrams == 0 {
		return 0
	}
	return float64(totalPatterns) / float64(cjkBigrams)
}

func topicSuffixFromNote(note, combined string, rows []factRow) string {
	note = strings.TrimSpace(note)
	ctx := note + "\n" + combined
	hasModel := strings.Contains(ctx, "模型")
	hasSpawn := strings.Contains(ctx, "发布") || strings.Contains(ctx, "版本发布")
	hasCite := strings.Contains(ctx, "引用") || strings.Contains(ctx, "抓取")
	hasSchedule := strings.Contains(ctx, "时间") || strings.Contains(ctx, "日历") || strings.Contains(ctx, "档期")

	switch {
	case hasModel && hasSpawn:
		return "模型发布时间表"
	case hasSpawn:
		return "发布时间表"
	case hasCite && hasModel:
		return "模型引用说明"
	case hasCite:
		return "引用说明"
	case hasSchedule && len(rows) >= 2:
		return "时间表"
	case strings.Contains(ctx, "活动") && hasSchedule:
		return "活动时间表"
	default:
		return ""
	}
}

func extractEventLabel(results []ExtractionResult, combined string) string {
	if stem := commonAttachmentEventStem(results); stem != "" {
		return stem
	}
	if m := reEventInText.FindStringSubmatch(combined); len(m) > 1 {
		return strings.TrimSpace(strings.ReplaceAll(m[1], " ", ""))
	}
	for _, line := range strings.Split(combined, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || len([]rune(line)) > 40 {
			continue
		}
		if strings.Contains(line, "活动") && strings.Contains(line, "日历") {
			return strings.TrimSpace(strings.TrimSuffix(line, "拆解图鉴"))
		}
	}
	return ""
}

func commonAttachmentEventStem(results []ExtractionResult) string {
	counts := map[string]int{}
	for _, r := range results {
		stem := normalizeAttachmentStem(r.Filename)
		if stem == "" {
			continue
		}
		counts[stem]++
	}
	best, bestN := "", 0
	for s, n := range counts {
		if n > bestN || (n == bestN && len([]rune(s)) > len([]rune(best))) {
			best, bestN = s, n
		}
	}
	return best
}

func normalizeAttachmentStem(filename string) string {
	base := strings.TrimSpace(filepath.Base(filename))
	if base == "" {
		return ""
	}
	ext := filepath.Ext(base)
	stem := strings.TrimSuffix(base, ext)
	stem = reTrailingIndex.ReplaceAllString(stem, "")
	stem = strings.TrimSpace(stem)
	for _, tail := range []string{"版本发布", "发布时间表", "出没"} {
		if strings.HasSuffix(stem, tail) {
			stem = strings.TrimSuffix(stem, tail)
		}
	}
	stem = strings.Trim(strings.TrimSpace(stem), "_-")
	if stem == "" || LooksLikeAttachmentFilename(stem) {
		return ""
	}
	if m := reSeasonStem.FindStringSubmatch(stem); len(m) > 1 {
		return strings.TrimSpace(m[1])
	}
	// Keep short meaningful stems (e.g. 接口说明) when prefixed with S2 elsewhere — handled by counts.
	if len([]rune(stem)) >= 4 && len([]rune(stem)) <= 28 && !LooksLikeAttachmentFilename(stem) {
		return stem
	}
	return ""
}

func sampleSubjectNames(rows []factRow, max int) string {
	if max <= 0 {
		max = 1
	}
	seen := map[string]bool{}
	var names []string
	for _, r := range rows {
		s := strings.TrimSpace(r.Subject)
		if s == "" || seen[s] {
			continue
		}
		if isGenericSubjectName(s) {
			continue
		}
		seen[s] = true
		names = append(names, s)
		if len(names) >= max {
			break
		}
	}
	if len(names) == 0 {
		return ""
	}
	if len(names) == 1 {
		return names[0]
	}
	return names[0] + "等"
}

func isGenericSubjectName(s string) bool {
	for _, g := range []string{"活动", "图鉴", "日历", "说明", "整理"} {
		if strings.Contains(s, g) && len([]rune(s)) > 8 {
			return true
		}
	}
	for _, r := range s {
		if !unicode.IsDigit(r) && !unicode.IsLetter(r) && !unicode.Is(unicode.Han, r) {
			return true
		}
	}
	return false
}

func min2(a, b int) int {
	if a < b {
		return a
	}
	return b
}
