package retriever

import (
	"testing"

	"github.com/graydraft/core/model"
)

func TestRRFFuseWeighted_prefersOverlap(t *testing.T) {
	ft := []model.Chunk{
		{ID: "a", KBType: "paper", Content: "one"},
		{ID: "b", KBType: "paper", Content: "two"},
	}
	vec := []model.Chunk{
		{ID: "a", KBType: "paper", Content: "one"},
		{ID: "c", KBType: "paper", Content: "three"},
	}
	out := rrfFuseWeighted(60, 1, 1, ft, vec)
	if len(out) != 3 {
		t.Fatalf("expected 3 chunks, got %d", len(out))
	}
	if out[0].ID != "a" {
		t.Fatalf("expected chunk a on top, got %s", out[0].ID)
	}
	if out[0].Score <= 0 {
		t.Fatalf("expected positive RRF score, got %f", out[0].Score)
	}
}
