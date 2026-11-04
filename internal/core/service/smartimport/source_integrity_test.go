package smartimport

import (
	"strings"
	"testing"
)

func TestSourcePlanRejectsLostBodyAndInvalidRanges(t *testing.T) {
	raw := "# 论文\n\n原论文标题\n\n观测时间为12:41:04，信噪比32.4。\n重要的质量条件与误差必须保留。"
	for _, ranges := range [][][2]int{{{1, 0}, {1, 3}}, {{1, 3}}, {{1, 6}, {4, 6}}, {{1, 99}}} {
		p := SplitPlan{}
		for _, r := range ranges {
			p.Segments = append(p.Segments, PlannedSegment{LineRange: r, Content: ExtractByLineRange(raw, r[0], r[1])})
		}
		got := preserveSourcePlan(raw, p)
		if len(got.Segments) != 1 || got.Segments[0].Content != raw || !strings.Contains(got.Segments[0].Content, "32.4") {
			t.Fatalf("lost body: %+v", got)
		}
	}
}
func TestRailsCannotLeaveStaleInvalidContent(t *testing.T) {
	raw := "# 这是一个足够长的论文标题用于对齐测试\n\n第一段重要科学数据32.4和1.7秒。\n第二段重要数据及误差范围。"
	p := SplitPlan{Segments: []PlannedSegment{{LineRange: [2]int{1, 2}, Content: "stale"}, {LineRange: [2]int{3, 4}, Content: "stale"}}}
	got := alignSplitPlanToSkeletonRails(p, ScanSkeleton(raw), raw)
	for _, seg := range got.Segments {
		if seg.LineRange[1] < seg.LineRange[0] || seg.Content != ExtractByLineRange(raw, seg.LineRange[0], seg.LineRange[1]) {
			t.Fatal(got)
		}
	}
}

func TestSourcePlanBoundsLongPaperWithoutLosingTail(t *testing.T) {
	raw := strings.Repeat("This is an independently measured scientific observation.\n", 1500) + "FINAL_EVIDENCE_32.4"
	p := SplitPlan{Segments: []PlannedSegment{singleSegment(raw, DocAnalysis{})}}
	got := preserveSourcePlan(raw, p)
	if len(got.Segments) < 2 {
		t.Fatal("paper not bounded")
	}
	var body strings.Builder
	for _, seg := range got.Segments {
		if len([]rune(seg.Content)) > 12000 {
			t.Fatal("over budget")
		}
		body.WriteString(seg.Content)
	}
	if !strings.Contains(body.String(), "FINAL_EVIDENCE_32.4") {
		t.Fatal("lost tail")
	}
}

func TestEnglishNoiseUsesRepeatedPhrasesNotAlphabetBigrams(t *testing.T) {
	text := "The observatory measures gravitational radiation from two compact objects. Scientists compare the measured waveform with numerical simulations and report distances, masses, confidence intervals, calibration methods, and independent observations across several instruments. The paper discusses astrophysical implications and experimental limitations."
	if NgramRepeatRate(text) > .5 {
		t.Fatal("ordinary English rejected as repetition")
	}
	if NgramRepeatRate(strings.Repeat("buy cheap products now ", 100)) < .5 {
		t.Fatal("repeated phrase not detected")
	}
}
