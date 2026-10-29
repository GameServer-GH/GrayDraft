package smartimport

import (
	"strings"

	"github.com/graydraft/core/model"
)

// applySegmentOverrides rebuilds PlannedSegment list from user line ranges (O-06).
func applySegmentOverrides(raw string, overrides []model.SmartImportSegmentOverride, base []PlannedSegment) []PlannedSegment {
	if len(overrides) == 0 {
		return base
	}
	lines := strings.Split(strings.ReplaceAll(raw, "\r\n", "\n"), "\n")
	if len(lines) == 0 {
		return base
	}
	out := make([]PlannedSegment, 0, len(overrides))
	for _, ov := range overrides {
		start, end := ov.LineRange[0], ov.LineRange[1]
		if start < 1 {
			start = 1
		}
		if end > len(lines) {
			end = len(lines)
		}
		if end < start {
			continue
		}
		content := strings.TrimSpace(strings.Join(lines[start-1:end], "\n"))
		if content == "" {
			continue
		}
		seg := PlannedSegment{
			Content:   content,
			LineRange: [2]int{start, end},
		}
		if ov.Index >= 0 && ov.Index < len(base) {
			seg.Title = base[ov.Index].Title
			seg.ContextEnhancement = base[ov.Index].ContextEnhancement
		}
		out = append(out, seg)
	}
	if len(out) == 0 {
		return base
	}
	return out
}
