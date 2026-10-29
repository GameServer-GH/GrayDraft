package smartimport

import (
	"strings"
	"testing"
)

func TestPlanSegments_singleGuide(t *testing.T) {
	raw := "# 《InternLM》推理部署指南\n\n" + strings.Repeat("公式与参数详细说明段落。", 20)
	plan := PlanSegments(raw, false, nil, false, nil)
	if plan.Strategy != strategySingle {
		t.Fatalf("expected single strategy, got %q", plan.Strategy)
	}
	if len(plan.Segments) != 1 {
		t.Fatalf("expected 1 segment, got %d", len(plan.Segments))
	}
	if plan.DocAnalysis.Subject == "" {
		t.Fatal("expected subject from book title")
	}
}

func TestPlanSegments_noRuleSplitBySeparator(t *testing.T) {
	raw := strings.Repeat("第一段社区规范说明内容。", 4) + "\n---\n" + strings.Repeat("第二段产品说明 index_ready 介绍。", 4)
	plan := PlanSegments(raw, false, nil, false, nil)
	if plan.Strategy != strategySingle || len(plan.Segments) != 1 {
		t.Fatalf("expected single skeleton segment, got strategy=%q segs=%d", plan.Strategy, len(plan.Segments))
	}
	for _, seg := range plan.Segments {
		if seg.LineRange[0] < 1 || seg.LineRange[1] < seg.LineRange[0] {
			t.Fatalf("invalid line range: %v", seg.LineRange)
		}
		if seg.Content == "" {
			t.Fatal("empty segment content")
		}
	}
}

func TestExtractByLineRange(t *testing.T) {
	raw := "a\nb\nc"
	got := ExtractByLineRange(raw, 2, 3)
	if got != "b\nc" {
		t.Fatalf("got %q", got)
	}
}

func TestPlanSegments_forceSingle(t *testing.T) {
	raw := "## A\n" + strings.Repeat("x", 50) + "\n---\n## B\n" + strings.Repeat("y", 50)
	plan := PlanSegments(raw, false, nil, true, nil)
	if plan.Strategy != strategySingle || len(plan.Segments) != 1 {
		t.Fatalf("expected forced single, got strategy=%s segs=%d", plan.Strategy, len(plan.Segments))
	}
}
