package rerank

import (
	"testing"

	"github.com/graydraft/gateway/service/provider"
)

func TestOrderByHits(t *testing.T) {
	candidates := []Candidate{
		{ID: "a", Content: "无关"},
		{ID: "b", Content: "相关"},
		{ID: "c", Content: "次相关"},
	}
	got := orderByHits(candidates, []provider.RerankHit{{Index: 1, Score: 0.9}, {Index: 2, Score: 0.4}}, 2)
	if len(got) != 2 || got[0].ID != "b" || got[0].Score != 0.9 || got[0].Source != "text_rerank" {
		t.Fatalf("top=%+v", got)
	}
	if got[1].ID != "c" || got[1].Score != 0.4 {
		t.Fatalf("second=%+v", got[1])
	}
}

func TestOrderByHitsKeepsOriginalOrderOnTie(t *testing.T) {
	candidates := []Candidate{{ID: "a"}, {ID: "b"}}
	got := orderByHits(candidates, nil, 2)
	if got[0].ID != "a" || got[1].ID != "b" {
		t.Fatalf("order=%s %s", got[0].ID, got[1].ID)
	}
}
