package smartimport

import (
	"fmt"
	"strings"
)

// LogicSkeletonSection is one semantic rail segment (LLM outline aligned to structure anchors).
type LogicSkeletonSection struct {
	Range           [2]int `json:"range"`
	AnchorKind      string `json:"anchor_kind,omitempty"` // h1|h2|h3|sep
	TitleHint       string `json:"title_hint,omitempty"`
	SemanticSummary string `json:"semantic_summary"`
}

// FormatSkeletonAnchorsForLLM lists structural anchors for LLM fill-in (reduces hallucinated ranges).
func FormatSkeletonAnchorsForLLM(sk DocumentSkeleton) string {
	var b strings.Builder
	b.WriteString("结构锚点（行号 1-based，拆分 range 应尽量对齐这些锚点，勿编造行号）：\n")
	n := 0
	for _, sl := range sk.Lines {
		switch sl.Kind {
		case "h1", "h2", "h3", "sep":
			text := strings.TrimSpace(sl.Text)
			if sl.Kind == "sep" {
				text = "---"
			}
			if len([]rune(text)) > 60 {
				text = string([]rune(text)[:60]) + "…"
			}
			b.WriteString(fmt.Sprintf("- L%d %s %s\n", sl.LineNo, sl.Kind, text))
			n++
		}
	}
	if n == 0 {
		return "结构锚点：无明显标题，按段落语义拆分。\n"
	}
	return b.String()
}

func buildSplitPlanFromLogicSkeleton(sections []LogicSkeletonSection, raw string, sk DocumentSkeleton, analysis DocAnalysis) (SplitPlan, error) {
	if len(sections) == 0 {
		return SplitPlan{}, fmt.Errorf("empty logic_skeleton")
	}
	strategy := strategyMulti
	if len(sections) == 1 {
		strategy = strategySingle
	}
	var segs []PlannedSegment
	for _, sec := range sections {
		start, end := sec.Range[0], sec.Range[1]
		if start < 1 || end < start || end > sk.LineCount {
			continue
		}
		content := ExtractByLineRange(raw, start, end)
		if len([]rune(content)) < 20 || isClosingFluff(content) || isAttachmentFilenameOnlySegment(content) {
			continue
		}
		t := strings.TrimSpace(sec.TitleHint)
		if t == "" || !titleAppearsInContent(t, content) {
			t = segmentTitleFromRange(raw, start, end, analysis)
		}
		summary := strings.TrimSpace(sec.SemanticSummary)
		if summary == "" {
			summary = t
		}
		seg := PlannedSegment{
			Content:            content,
			LineRange:          [2]int{start, end},
			Title:              displayTitle(analysis.ParentTitle, t),
			ContextEnhancement: buildContextEnhancement(analysis, t),
			SemanticSummary:    summary,
			SearchIntent:       summary,
		}
		segs = append(segs, seg)
	}
	if len(segs) == 0 {
		return SplitPlan{}, fmt.Errorf("no valid logic_skeleton sections")
	}
	if len(segs) == 1 {
		strategy = strategySingle
	}
	return SplitPlan{
		DocAnalysis: analysis,
		Strategy:    strategy,
		Method:      "unified_llm+rail",
		Segments:    segs,
	}, nil
}

// alignSplitPlanToSkeletonRails snaps segment boundaries to nearby structural anchors.
func alignSplitPlanToSkeletonRails(plan SplitPlan, sk DocumentSkeleton, raw string) SplitPlan {
	if len(plan.Segments) <= 1 || sk.LineCount == 0 {
		return plan
	}
	anchorLines := make([]int, 0, 16)
	for _, sl := range sk.Lines {
		if sl.Kind == "h1" || sl.Kind == "h2" || sl.Kind == "h3" || sl.Kind == "sep" {
			anchorLines = append(anchorLines, sl.LineNo)
		}
	}
	if len(anchorLines) == 0 {
		return plan
	}
	for i := range plan.Segments {
		start := plan.Segments[i].LineRange[0]
		end := plan.Segments[i].LineRange[1]
		if i > 0 {
			if a := nearestAnchor(start, anchorLines, 3); a > plan.Segments[i-1].LineRange[0] && a < end {
				start = a
				plan.Segments[i-1].LineRange[1] = a - 1
			}
		}
		if i < len(plan.Segments)-1 {
			if a := nearestAnchor(end+1, anchorLines, 3); a > start && a <= sk.LineCount {
				end = a - 1
			}
		}
		if end >= start {
			plan.Segments[i].LineRange = [2]int{start, end}
			plan.Segments[i].Content = ExtractByLineRange(raw, start, end)
		}
	}
	for i := range plan.Segments {
		seg := &plan.Segments[i]
		seg.Content = ExtractByLineRange(raw, seg.LineRange[0], seg.LineRange[1])
	}
	return preserveSourcePlan(raw, plan)
}

func nearestAnchor(line int, anchors []int, maxDist int) int {
	best, bestD := 0, maxDist+1
	for _, a := range anchors {
		d := line - a
		if d < 0 {
			d = -d
		}
		if d <= maxDist && d < bestD {
			best, bestD = a, d
		}
	}
	return best
}
