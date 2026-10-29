package retriever

import (
	"context"
	"testing"

	"github.com/graydraft/core/config"
	"github.com/graydraft/core/model"
)

func TestExtractQuery4DCached_hit(t *testing.T) {
	cfg := &config.Config{}
	cfg.Retrieval.Semantic4DLLMEnabled = false
	q := "模型版本发布时间表"
	tax := model.DefaultCatalog().Get(model.UniversalDomainKey)
	first := ExtractQuery4DCached(context.Background(), q, cfg, nil, "universal|v0", tax)
	second := ExtractQuery4DCached(context.Background(), q, cfg, nil, "universal|v0", tax)
	if !second.Active() {
		t.Fatal("expected active 4d")
	}
	if first.CacheFingerprint() != second.CacheFingerprint() {
		t.Fatal("fingerprint mismatch")
	}
	if second.Method != "rules+cache" && second.Method != "cache" {
		t.Fatalf("expected cache method suffix, got %q", second.Method)
	}
}
