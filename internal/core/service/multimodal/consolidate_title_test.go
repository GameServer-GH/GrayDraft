package multimodal

import (
	"strings"
	"testing"
)

func TestInferConsolidatedTitle_spawnGuide(t *testing.T) {
	results := []ExtractionResult{
		{Filename: "S2接口说明版本发布_1.jpg", Text: legacyJSON1},
		{Filename: "S2接口说明版本发布_2.jpg", Text: legacyJSON2},
	}
	note := "梳理附近中模型发布时间和抓取建议"
	got := InferConsolidatedTitle(note, results)
	if got == "" {
		t.Fatal("expected non-empty title")
	}
	if IsGenericConsolidatedTitle(got) {
		t.Fatalf("generic title %q", got)
	}
	if got != "S2接口说明模型发布时间表" {
		t.Fatalf("title=%q want InternVL 接口发布时间表", got)
	}
}

func TestIsGenericConsolidatedTitle(t *testing.T) {
	for _, bad := range []string{"多图要点汇总", "要点汇总", "多图汇总"} {
		if !IsGenericConsolidatedTitle(bad) {
			t.Fatalf("expected generic: %q", bad)
		}
	}
	if IsGenericConsolidatedTitle("InternVL 接口发布时间表") {
		t.Fatal("specific title should not be generic")
	}
}

func TestFinalizeIngestText_infersTitleWhenLLMEmpty(t *testing.T) {
	results := []ExtractionResult{
		{Filename: "S2接口说明版本发布_1.jpg", Text: legacyJSON1},
		{Filename: "S2接口说明版本发布_2.jpg", Text: legacyJSON2},
	}
	note := "梳理附近中模型发布时间和抓取建议"
	final, title, ok := FinalizeIngestText(note, results, "")
	if !ok {
		t.Fatal("expected consolidation")
	}
	if IsGenericConsolidatedTitle(title) {
		t.Fatalf("title=%q", title)
	}
	if !strings.Contains(final, "InternVL") || !strings.Contains(final, "视觉编码") {
		t.Fatalf("missing content: %s", final)
	}
}
