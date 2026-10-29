package retriever

import (
	"context"
	"strings"
	"sync"

	"github.com/graydraft/core/model"
	"github.com/graydraft/core/service/classifier"
)

// searchMultiKB runs parallel search per partition and merges with RRF (O-07).
func (h *Hybrid) searchMultiKB(ctx context.Context, req model.SearchRequest, topK, candidateK int, plan classifier.RoutePlan) (*model.SearchResponse, error) {
	types := dedupeKBTypes(req.KBTypes)
	if len(types) == 0 {
		return nil, nil
	}
	perKB := candidateK
	if perKB < 15 {
		perKB = 15
	}
	lists := make([][]model.Chunk, len(types))
	var wg sync.WaitGroup
	var mu sync.Mutex
	var firstErr error

	for i, kb := range types {
		wg.Add(1)
		go func(idx int, partition string) {
			defer wg.Done()
			sub := req
			sub.KBType = partition
			sub.KBTypes = nil
			sub.TopK = perKB
			resp, err := h.Search(ctx, sub)
			if err != nil {
				mu.Lock()
				if firstErr == nil {
					firstErr = err
				}
				mu.Unlock()
				return
			}
			if resp != nil {
				lists[idx] = resp.Chunks
			}
		}(i, kb)
	}
	wg.Wait()
	if firstErr != nil {
		return nil, firstErr
	}

	weights := make([]float64, len(lists))
	for i := range weights {
		weights[i] = 1
	}
	fused := rrfFuseWeightedN(h.constK, weights, lists)
	if len(fused) > topK {
		fused = fused[:topK]
	}
	for i := range fused {
		fused[i].Source = "multi_kb"
	}
	return &model.SearchResponse{
		Query:          req.Query,
		KBTypes:        types,
		TopK:           topK,
		Total:          len(fused),
		Chunks:         fused,
		Strategy:       "multi_kb_rrf+" + string(plan.Intent),
		QueryIntent:    string(plan.Intent),
		FulltextWeight: plan.FulltextWeight,
		VectorWeight:   plan.VectorWeight,
	}, nil
}

func dedupeKBTypes(in []string) []string {
	seen := make(map[string]struct{})
	var out []string
	for _, k := range in {
		k = strings.TrimSpace(k)
		if k == "" {
			continue
		}
		if _, ok := seen[k]; ok {
			continue
		}
		seen[k] = struct{}{}
		out = append(out, k)
	}
	return out
}
