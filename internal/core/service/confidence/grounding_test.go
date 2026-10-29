package confidence

import (
	"testing"

	"github.com/graydraft/core/model"
)

func TestPassesHighQualityGrounding_quality_too_low(t *testing.T) {
	chunks := []model.Chunk{
		{QualityScore: 0.60, TitleMatched: false, EntityMatched: false},
	}
	if passesHighQualityGrounding("测试", chunks) {
		t.Fatal("low quality should not pass")
	}
}

func TestPassesHighQualityGrounding_title_matched(t *testing.T) {
	chunks := []model.Chunk{
		{QualityScore: 0.75, TitleMatched: true, EntityMatched: false},
	}
	if !passesHighQualityGrounding("InternVL 教程", chunks) {
		t.Fatal("title matched should pass")
	}
}

func TestPassesHighQualityGrounding_entity_matched(t *testing.T) {
	chunks := []model.Chunk{
		{QualityScore: 0.75, TitleMatched: false, EntityMatched: true},
	}
	if !passesHighQualityGrounding("InternVL", chunks) {
		t.Fatal("entity matched should pass")
	}
}

func TestPassesHighQualityGrounding_high_quality_grounded(t *testing.T) {
	chunks := []model.Chunk{
		{
			QualityScore: 0.85,
			TitleMatched: false,
			EntityMatched: false,
			Content:      "InternVL参数说明：温度 0.7，上下文 32768。",
			DocTitle:     "选型说明",
		},
	}
	if !passesHighQualityGrounding("InternVL参数说明", chunks) {
		t.Fatal("high quality with query grounding should pass")
	}
}

func TestPassesHighQualityGrounding_empty(t *testing.T) {
	if passesHighQualityGrounding("test", nil) {
		t.Fatal("empty chunks should not pass")
	}
}

func TestGroundingTerms_simple(t *testing.T) {
	terms := groundingTerms("InternVL参数说明")
	if len(terms) == 0 {
		t.Fatal("expected terms")
	}
}

func TestGroundingTerms_empty(t *testing.T) {
	terms := groundingTerms("")
	if len(terms) != 0 {
		t.Fatal("empty query should have no terms")
	}
}
