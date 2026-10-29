package smartimport

import (
	"testing"

	"github.com/graydraft/core/model"
)

func TestDeriveSegmentSemantic4D_mergesEntities(t *testing.T) {
	doc := model.Extensions{
		"semantic_tags_4d": map[string]any{
			"domain": "教程资料", "scene": "版本发布", "entities": []any{"模型"},
		},
	}
	seg := PlannedSegment{Title: "S2InternVL", Tags: []string{"05.22"}, SemanticSummary: "出没时段"}
	got := DeriveSegmentSemantic4D(doc, seg)
	if got == nil {
		t.Fatal("expected segment 4d")
	}
	ents, _ := got["entities"].([]string)
	if len(ents) < 2 {
		t.Fatalf("expected merged entities, got %v", ents)
	}
}
