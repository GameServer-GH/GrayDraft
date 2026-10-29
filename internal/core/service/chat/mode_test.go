package chat

import (
	"testing"

	"github.com/graydraft/core/model"
	"github.com/graydraft/core/service/confidence"
)

func TestHasUsefulRetrieval(t *testing.T) {
	chunks := []model.Chunk{{RerankScore: 0.5, VectorScore: 0.2, Score: 0.5}}
	if !HasUsefulRetrieval(chunks, 1, "test", "fact") {
		t.Fatal("expected useful by composite confidence")
	}
	if HasUsefulRetrieval(nil, 0, "", "") {
		t.Fatal("empty should not be useful")
	}
}

func TestEvaluateRetrievalGrade(t *testing.T) {
	// Empty → GradeD
	gd := EvaluateRetrievalGrade(nil, 0, "", "", nil)
	if gd.Grade != confidence.GradeD {
		t.Fatalf("empty chunks: expected GradeD, got %s", gd.Grade)
	}

	// Low scores → GradeC (has results, so not D)
	gd = EvaluateRetrievalGrade([]model.Chunk{
		{RerankScore: 0.05, VectorScore: 0.02, Score: 0.05, QualityScore: 0.1},
	}, 1, "test", "fact", nil)
	if gd.Grade != confidence.GradeC {
		t.Fatalf("low scores: expected GradeC, got %s (reason: %s, score: %.3f)", gd.Grade, gd.Reason, gd.Score)
	}
	if !gd.Grade.IsUseful() {
		t.Fatal("GradeC should be useful (sparse context injection)")
	}

	// Strong signals → GradeA or B
	gd = EvaluateRetrievalGrade([]model.Chunk{
		{RerankScore: 0.85, VectorScore: 0.8, Score: 0.85, QualityScore: 0.82, TitleMatched: true, EntityMatched: true},
	}, 1, "test", "fact", nil)
	if gd.Grade != confidence.GradeA && gd.Grade != confidence.GradeB {
		t.Fatalf("strong signals: expected GradeA/B, got %s", gd.Grade)
	}
}
