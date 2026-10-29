package smartimport

import "testing"

func TestBuildSplitPlanFromLogicSkeleton(t *testing.T) {
	raw := "# InternVL\n\n## Q技能\n\n突进说明文字足够长以满足最小长度要求。\n\n## W技能\n\n风墙说明文字同样足够长以满足最小长度要求。\n"
	sk := ScanSkeleton(raw)
	sections := []LogicSkeletonSection{
		{Range: [2]int{1, sk.LineCount}, AnchorKind: "h1", TitleHint: "InternVL", SemanticSummary: "InternVL与技能总览"},
	}
	plan, err := buildSplitPlanFromLogicSkeleton(sections, raw, sk, sk.Analysis)
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.Segments) != 1 {
		t.Fatalf("segments=%d want 1", len(plan.Segments))
	}
	if plan.Segments[0].SemanticSummary != "InternVL与技能总览" {
		t.Fatalf("summary=%q", plan.Segments[0].SemanticSummary)
	}
}
