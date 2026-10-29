package smartimport

import "testing"

func TestInferSubjectTitle_yasuoIntro(t *testing.T) {
	raw := `InternVL（InternVL）是《书生文档》中的一名多模态模型，用于理解课件页面、图表和截图。调用时需要设置图像分辨率、最大输入长度和视觉编码器。`
	got := inferSubjectTitle(raw)
	if got != "InternVL" {
		t.Fatalf("inferSubjectTitle=%q want InternVL", got)
	}
}

func TestAnalyze_yasuoIntroTitle(t *testing.T) {
	raw := `InternVL（InternVL）是《书生文档》中的一名多模态模型，用于理解课件页面、图表和截图。调用时需要设置图像分辨率、最大输入长度和视觉编码器。`
	llm := func(string) (string, error) {
		return `{"kb_type":"tutorial","title":"书生文档","confidence":0.9,"reason":"model mention"}`, nil
	}
	d, err := Analyze(raw, "", "", "gd_test_workspace", "", true, llm, nil)
	if err != nil {
		t.Fatal(err)
	}
	if d.Title == "书生文档" {
		t.Fatalf("title should not be generic franchise, got %q", d.Title)
	}
	if d.Title != "InternVL" {
		t.Fatalf("title=%q want InternVL", d.Title)
	}
}

func TestScanSkeleton_yasuoSubjectNotParentTitle(t *testing.T) {
	raw := `InternVL（InternVL）是《书生文档》中的一名多模态模型。`
	sk := ScanSkeleton(raw)
	if sk.Analysis.Subject != "书生文档" {
		t.Fatalf("subject=%q", sk.Analysis.Subject)
	}
	if sk.Analysis.ParentTitle != "InternVL" {
		t.Fatalf("parent_title=%q want InternVL", sk.Analysis.ParentTitle)
	}
}

func TestPlanSegments_yasuoSingleTitle(t *testing.T) {
	raw := `InternVL（InternVL）是《书生文档》中的一名多模态模型，用于理解课件页面和图表。`
	plan := PlanSegments(raw, false, nil, false, nil)
	if len(plan.Segments) != 1 {
		t.Fatalf("segments=%d", len(plan.Segments))
	}
	if plan.Segments[0].Title != "InternVL" {
		t.Fatalf("segment title=%q want InternVL", plan.Segments[0].Title)
	}
}

func TestIsGenericFranchiseTitle(t *testing.T) {
	if !isGenericFranchiseTitle("书生文档", "书生文档") {
		t.Fatal("expected generic")
	}
	if isGenericFranchiseTitle("InternVL", "书生文档") {
		t.Fatal("InternVL should not be generic")
	}
}
