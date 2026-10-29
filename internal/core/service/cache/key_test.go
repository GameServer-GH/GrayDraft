package cache

import (
	"testing"
	"time"
)

func TestBuildExactKey_versionAware(t *testing.T) {
	updated := time.Unix(1000, 0)
	p1 := LookupParams{Query: "Hello", KBType: "tutorial", TopK: 10, CatalogVersion: 1, CatalogUpdated: updated}
	p2 := LookupParams{Query: "hello", KBType: "paper", TopK: 10, CatalogVersion: 2, CatalogUpdated: updated}
	if BuildExactKey(p1) == BuildExactKey(p2) {
		t.Fatal("expected different keys for different catalog versions")
	}
}

func TestBuildExactKey_semantic4DFingerprint(t *testing.T) {
	updated := time.Unix(1000, 0)
	base := LookupParams{
		Query: "模型发布", KBType: "tutorial", TopK: 10,
		CatalogVersion: 1, CatalogUpdated: updated, QueryIntent: "factual",
	}
	with4D := base
	with4D.Semantic4DFingerprint = "教程资料|模型|版本发布|时间表"
	if BuildExactKey(base) == BuildExactKey(with4D) {
		t.Fatal("expected different keys when 4D fingerprint differs")
	}
}

func TestCosineSimilarity_identical(t *testing.T) {
	a := []float32{1, 0, 0}
	b := []float32{1, 0, 0}
	if cosineSimilarity(a, b) < 0.99 {
		t.Fatalf("expected ~1.0, got %f", cosineSimilarity(a, b))
	}
}
