package smartimport

import (
	"testing"

	"github.com/graydraft/core/model"
)

func TestPlanSegments_prefersUnifiedCache(t *testing.T) {
	raw := "问：如何重置？\n答：设置页。\n问：2FA？\n答：安全设置。"
	ext := model.Extensions{
		extImportLLMUnifiedOK:     true,
		extImportLLMSplitPlanJSON: `{"strategy":"multi_segment","method":"unified_llm","doc_analysis":{},"segments":[{"content":"问：如何重置？\n答：设置页。","line_range":[1,2],"title":"重置密码"}]}`,
	}
	plan := PlanSegments(raw, false, nil, false, ext)
	if len(plan.Segments) == 0 {
		t.Fatal("expected segments from unified cache")
	}
	if plan.Strategy != strategyMulti {
		t.Fatalf("strategy=%q", plan.Strategy)
	}
}

func TestPlanSegments_unifiedWithoutCache_singleLLMPath(t *testing.T) {
	raw := "InternVL是书生文档中的英雄。"
	ext := model.Extensions{
		extImportLLMUnifiedOK: true,
		"import_title_hint":   "InternVL 接口",
	}
	plan := PlanSegments(raw, false, nil, false, ext)
	if plan.Method != "unified_llm" {
		t.Fatalf("method=%q", plan.Method)
	}
	if plan.Segments[0].Title == "" && plan.DocAnalysis.ParentTitle != "InternVL 接口" {
		t.Fatalf("parent=%q", plan.DocAnalysis.ParentTitle)
	}
}

func TestPlanSegments_unifiedDoubleFail_singleSkeleton(t *testing.T) {
	raw := "A\n---\nB"
	ext := model.Extensions{
		extImportLLMUnifiedOK:      false,
		extImportLLMUnifiedFailCnt: 2,
	}
	plan := PlanSegments(raw, true, func(string) (string, error) {
		t.Fatal("legacy plan LLM should not run after unified double fail")
		return "", nil
	}, false, ext)
	if plan.Method != "single_skeleton" || plan.Strategy != strategySingle || len(plan.Segments) != 1 {
		t.Fatalf("unexpected plan: method=%q strategy=%q segs=%d", plan.Method, plan.Strategy, len(plan.Segments))
	}
}
