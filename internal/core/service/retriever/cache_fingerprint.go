package retriever

import (
	"context"

	"github.com/graydraft/core/config"
	"github.com/graydraft/core/service/llmclient"
	"github.com/graydraft/core/service/taxonomyevolve"
)

// QueryCache4DFingerprint returns the retrieval/answer cache segment for active query 4D filters.
func QueryCache4DFingerprint(ctx context.Context, query string, cfg *config.Config, llm *llmclient.Client, kbType string) string {
	if cfg == nil || !cfg.Retrieval.Semantic4DFilterEnabled {
		return ""
	}
	tax := taxonomyevolve.Effective(ctx, kbType, nil, "")
	seg := taxonomyevolve.CacheKeySegment(ctx, kbType, nil, "")
	q := ExtractQuery4DCached(ctx, query, cfg, llm, seg, tax)
	if !q.Active() {
		return ""
	}
	return q.CacheFingerprint()
}

// QueryCache4DFingerprint is a convenience wrapper on Hybrid.
func (h *Hybrid) QueryCache4DFingerprint(ctx context.Context, query string) string {
	if h == nil {
		return ""
	}
	return QueryCache4DFingerprint(ctx, query, h.cfg, h.llm, "")
}
