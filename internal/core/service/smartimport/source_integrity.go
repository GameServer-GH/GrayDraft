package smartimport

import "strings"

// preserveSourcePlan is the final barrier after ALL coordinate transformations.
// Invalid or incomplete plans fall back to a lossless source segment; summaries
// never substitute for source coverage. The worker can split long narratives.
func preserveSourcePlan(raw string, plan SplitPlan) SplitPlan {
	raw = normalizeRaw(raw)
	lines := strings.Split(raw, "\n")
	covered := make([]bool, len(lines))
	valid := len(plan.Segments) > 0
	for _, seg := range plan.Segments {
		a, b := seg.LineRange[0], seg.LineRange[1]
		if a < 1 || b < a || b > len(lines) || seg.Content != ExtractByLineRange(raw, a, b) || len([]rune(seg.Content)) > 12000 {
			valid = false
			break
		}
		for i := a - 1; i < b; i++ {
			if covered[i] && strings.TrimSpace(lines[i]) != "" {
				valid = false
			}
			covered[i] = true
		}
	}
	for i, line := range lines {
		if strings.TrimSpace(line) != "" && !covered[i] {
			valid = false
		}
	}
	if valid {
		return plan
	}
	plan.Strategy = strategySingle
	plan.Method = "source_integrity_fallback"
	plan.Segments = nil
	start, size := 1, 0
	for i, line := range lines {
		n := len([]rune(line)) + 1
		if size+n > 12000 && i+1 > start {
			seg := singleSegment(raw, plan.DocAnalysis)
			seg.LineRange = [2]int{start, i}
			seg.Content = ExtractByLineRange(raw, start, i)
			plan.Segments = append(plan.Segments, seg)
			start = i + 1
			size = 0
		}
		size += n
	}
	seg := singleSegment(raw, plan.DocAnalysis)
	seg.LineRange = [2]int{start, len(lines)}
	seg.Content = ExtractByLineRange(raw, start, len(lines))
	plan.Segments = append(plan.Segments, seg)
	if len(plan.Segments) > 1 {
		plan.Strategy = strategyMulti
	}
	return plan
}
