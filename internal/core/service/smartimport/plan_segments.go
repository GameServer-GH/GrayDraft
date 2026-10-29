package smartimport

import (
	"strings"

	"github.com/graydraft/core/model"
)

const (
	strategySingle = "single"
	strategyMulti  = "multi_segment"
)

// PlanSegments uses unified LLM split output when available; total LLM failure falls back to one skeleton segment.
func PlanSegments(raw string, useLLM bool, llmComplete func(string) (string, error), forceSingle bool, ext model.Extensions) SplitPlan {
	raw = normalizeRaw(raw)
	if cached, ok := loadCachedSplitPlan(ext, raw, forceSingle); ok {
		return cached
	}
	sk := ScanSkeleton(raw)
	if unifiedImportCacheHit(ext) {
		return splitPlanFromUnifiedLLM(raw, sk, ext, forceSingle)
	}
	if useLLM && unifiedImportFailCount(ext) == 0 {
		return splitPlanFromUnifiedLLM(raw, sk, ext, forceSingle)
	}
	return singleSkeletonPlan(raw, sk, "single_skeleton")
}

func singleSkeletonPlan(raw string, sk DocumentSkeleton, method string) SplitPlan {
	a := sk.Analysis
	return SplitPlan{
		DocAnalysis: a,
		Strategy:    strategySingle,
		Method:      method,
		Segments:    []PlannedSegment{singleSegment(raw, a)},
	}
}

// splitPlanFromUnifiedLLM uses unified import as authoritative split/title (rules only as coordinate fallback).
func splitPlanFromUnifiedLLM(raw string, sk DocumentSkeleton, ext model.Extensions, forceSingle bool) SplitPlan {
	a := sk.Analysis
	if hint := importTitleHint(ext); hint != "" {
		a.ParentTitle = hint
	}
	if forceSingle {
		return SplitPlan{
			DocAnalysis: a,
			Strategy:    strategySingle,
			Method:      "unified_llm",
			Segments:    []PlannedSegment{singleSegment(raw, a)},
		}
	}
	return SplitPlan{
		DocAnalysis: a,
		Strategy:    strategySingle,
		Method:      "unified_llm",
		Segments:    []PlannedSegment{singleSegment(raw, a)},
	}
}

func mergeAnalysis(base, llm DocAnalysis) DocAnalysis {
	out := base
	if llm.Subject != "" {
		out.Subject = llm.Subject
	}
	if llm.Version != "" {
		out.Version = llm.Version
	}
	if llm.DocType != "" {
		out.DocType = llm.DocType
	}
	if llm.ParentTitle != "" {
		out.ParentTitle = llm.ParentTitle
	}
	return out
}

func planSegmentsRules(raw string, sk DocumentSkeleton, forceSingle bool) SplitPlan {
	a := sk.Analysis
	if forceSingle || raw == "" {
		return SplitPlan{
			DocAnalysis: a,
			Strategy:    strategySingle,
			Method:      "rules",
			Segments:    []PlannedSegment{singleSegment(raw, a)},
		}
	}

	if shouldSplitMulti(raw, sk) {
		segs := splitByCoordinates(raw, sk)
		if len(segs) > 1 {
			enrichPlannedSegments(segs, a)
			return SplitPlan{
				DocAnalysis: a,
				Strategy:    strategyMulti,
				Method:      "rules",
				Segments:    segs,
			}
		}
	}

	// Unstructured long text fallback (> 5000 runes, no headings/separators)
	if len([]rune(raw)) > 5000 {
		segs := splitUnstructuredLongText(raw, sk)
		if len(segs) > 1 {
			enrichPlannedSegments(segs, a)
			return SplitPlan{
				DocAnalysis: a,
				Strategy:    strategyMulti,
				Method:      "unstructured_fallback",
				Segments:    segs,
			}
		}
	}

	return SplitPlan{
		DocAnalysis: a,
		Strategy:    strategySingle,
		Method:      "rules",
		Segments:    []PlannedSegment{singleSegment(raw, a)},
	}
}

func shouldSplitMulti(raw string, sk DocumentSkeleton) bool {
	if hasSubstantialSeparatorSplit(raw, sk) {
		return true
	}
	switch sk.Analysis.DocType {
	case "综合合集", "分段文档":
		return true
	case "问答集":
		h2, seps := 0, 0
		for _, sl := range sk.Lines {
			if sl.Kind == "h2" {
				h2++
			}
			if sl.Kind == "sep" {
				seps++
			}
		}
		return seps >= 2 || h2 >= 2
	default:
		h2, seps := 0, 0
		for _, sl := range sk.Lines {
			if sl.Kind == "h2" {
				h2++
			}
			if sl.Kind == "sep" {
				seps++
			}
		}
		return h2 >= 3 || seps >= 2
	}
}

func hasSubstantialSeparatorSplit(raw string, sk DocumentSkeleton) bool {
	for _, sl := range sk.Lines {
		if sl.Kind != "sep" {
			continue
		}
		before := ExtractByLineRange(raw, 1, sl.LineNo-1)
		after := ExtractByLineRange(raw, sl.LineNo+1, sk.LineCount)
		if len([]rune(before)) >= 40 && len([]rune(after)) >= 40 {
			return true
		}
	}
	return false
}

func singleSegment(raw string, a DocAnalysis) PlannedSegment {
	end := strings.Count(raw, "\n") + 1
	if raw == "" {
		end = 1
	}
	title := preferDocumentTitle(raw, a)
	seg := PlannedSegment{
		Content:            raw,
		LineRange:          [2]int{1, end},
		Title:              title,
		ContextEnhancement: buildContextEnhancement(a, title),
		SemanticSummary:    title,
	}
	return seg
}

func splitByCoordinates(raw string, sk DocumentSkeleton) []PlannedSegment {
	lineCount := sk.LineCount
	if lineCount == 0 {
		return nil
	}

	// Prefer separator blocks for 分段文档; otherwise split at h2.
	breakLines := collectBreakLines(sk, preferSeparators(sk))
	if len(breakLines) == 0 {
		return []PlannedSegment{singleSegment(raw, sk.Analysis)}
	}

	var ranges [][2]int
	start := 1
	for _, br := range breakLines {
		if br <= start {
			continue
		}
		end := br - 1
		if end >= start {
			ranges = append(ranges, [2]int{start, end})
		}
		start = br
	}
	if start <= lineCount {
		ranges = append(ranges, [2]int{start, lineCount})
	}

	var out []PlannedSegment
	for _, rg := range ranges {
		content := ExtractByLineRange(raw, rg[0], rg[1])
		if len([]rune(content)) < 40 {
			if len(out) > 0 {
				prev := &out[len(out)-1]
				prev.Content = ExtractByLineRange(raw, prev.LineRange[0], rg[1])
				prev.LineRange[1] = rg[1]
				prev.Title = segmentTitleFromRange(raw, prev.LineRange[0], prev.LineRange[1], sk.Analysis)
				continue
			}
			continue
		}
		if isClosingFluff(content) || isAttachmentFilenameOnlySegment(content) {
			continue
		}
		seg := PlannedSegment{
			Content:   content,
			LineRange: rg,
			Title:     segmentTitleFromRange(raw, rg[0], rg[1], sk.Analysis),
		}
		out = append(out, seg)
	}
	if len(out) == 0 {
		return []PlannedSegment{singleSegment(raw, sk.Analysis)}
	}
	return mergeShortSegments(out, raw, sk)
}

func preferSeparators(sk DocumentSkeleton) bool {
	seps := 0
	for _, sl := range sk.Lines {
		if sl.Kind == "sep" {
			seps++
		}
	}
	return seps >= 2 || sk.Analysis.DocType == "分段文档"
}

func collectBreakLines(sk DocumentSkeleton, useSep bool) []int {
	var breaks []int
	for _, sl := range sk.Lines {
		if useSep && sl.Kind == "sep" {
			breaks = append(breaks, sl.LineNo+1)
			continue
		}
		if !useSep && sl.Kind == "h2" {
			breaks = append(breaks, sl.LineNo)
		}
	}
	if len(breaks) == 0 && !useSep {
		for _, sl := range sk.Lines {
			if sl.Kind == "sep" {
				breaks = append(breaks, sl.LineNo+1)
			}
		}
	}
	return breaks
}

func enrichPlannedSegments(segs []PlannedSegment, a DocAnalysis) {
	total := len(segs)
	for i := range segs {
		segs[i].Title = displayTitle(a.ParentTitle, segs[i].Title)
		segs[i].ContextEnhancement = buildContextEnhancement(a, segs[i].Title)
		if segs[i].SemanticSummary == "" {
			segs[i].SemanticSummary = segs[i].Title
		}
		_ = total // segment index injected in service via index
	}
}

func buildContextEnhancement(a DocAnalysis, title string) string {
	var parts []string
	if a.Subject != "" {
		parts = append(parts, a.Subject)
	}
	if a.Version != "" {
		parts = append(parts, a.Version)
	}
	if title != "" && title != a.ParentTitle {
		parts = append(parts, title)
	} else if a.ParentTitle != "" {
		parts = append(parts, a.ParentTitle)
	}
	return strings.TrimSpace(strings.Join(parts, " "))
}
