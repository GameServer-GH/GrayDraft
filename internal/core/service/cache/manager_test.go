package cache

import (
	"context"
	"testing"
	"time"

	"github.com/graydraft/core/config"
	"github.com/graydraft/core/model"
)

func TestLookupDoesNotReplayPreviousQueryAsL3(t *testing.T) {
	cfg := &config.Config{}
	cfg.Cache.Enabled = true
	cfg.Cache.L3Enabled = true
	cfg.Cache.TTLSeconds = 60
	cfg.Cache.L1MaxItems = 100
	cfg.Cache.SemanticIndexMax = 10
	cfg.Cache.SemanticScanLimit = 10
	m, err := NewManager(cfg)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Unix(1000, 0)
	prev := LookupParams{
		Query:          "番茄有哪些吃法",
		KBType:         "gd_test",
		TopK:           5,
		CatalogVersion: 1,
		CatalogUpdated: now,
		QueryIntent:    "fact",
	}
	m.Store(context.Background(), prev, nil, &model.SearchResponse{
		Query:  prev.Query,
		KBType: prev.KBType,
		Total:  1,
		Chunks: []model.Chunk{{ID: "old"}},
	})
	current := prev
	current.Query = "番茄周几吃，一次吃多少"
	if resp, tier, ok := m.Lookup(context.Background(), current, nil); ok {
		t.Fatalf("unexpected cache replay tier=%s resp=%+v", tier, resp)
	}
}
