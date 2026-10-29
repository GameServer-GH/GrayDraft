package taxonomyevolve

import "testing"

func TestSimilarityRatio_identical(t *testing.T) {
	if SimilarityRatio("组合", "组合") != 1 {
		t.Fatal("expected 1")
	}
}

func TestSimilarityRatio_substring(t *testing.T) {
	r := SimilarityRatio("磁化机制", "磁化")
	if r < 0.5 {
		t.Fatalf("expected high similarity, got %f", r)
	}
}

func TestBestKeywordMatch(t *testing.T) {
	_, r := BestKeywordMatch("共鸣回路", []string{"培养", "共鸣", "术语"})
	if r <= 0 {
		t.Fatal("expected match")
	}
}
