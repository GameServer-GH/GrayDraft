package answercache

import (
	"testing"
	"time"
)

func TestBuildKey_versionBump(t *testing.T) {
	updated := time.Unix(1000, 0)
	p1 := LookupParams{Query: "Hello", KBType: "paper", TopK: 5, CatalogVersion: 1, CatalogUpdated: updated}
	p2 := LookupParams{Query: "hello", KBType: "paper", TopK: 5, CatalogVersion: 2, CatalogUpdated: updated}
	if BuildKey(p1) == BuildKey(p2) {
		t.Fatal("expected different keys after catalog version bump")
	}
}

func TestBuildKey_sameQuery(t *testing.T) {
	updated := time.Unix(1000, 0)
	p1 := LookupParams{Query: " 混合检索 ", KBType: "tutorial", TopK: 5, CatalogVersion: 3, CatalogUpdated: updated}
	p2 := LookupParams{Query: "混合检索", KBType: "tutorial", TopK: 5, CatalogVersion: 3, CatalogUpdated: updated}
	if BuildKey(p1) != BuildKey(p2) {
		t.Fatal("expected same key for normalized query")
	}
}
