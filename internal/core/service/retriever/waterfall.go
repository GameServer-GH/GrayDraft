package retriever

import (
	"context"
	"sync"

	"github.com/graydraft/core/model"
	"github.com/graydraft/core/service/classifier"
)

const waterfallMinChunks = 1

// searchWithSem4DWaterfall runs tiered 4D pre-filters with optimistic concurrency.
//
// Tier 1 (_wf):     domain + entities + scene + style  (strict)
// Tier 2 (_wf_core): domain + entities (drop scene/style intent noise)
// Tier 3 (_wf_ent):  domain + shortest entity only (atomic ID)
// Tier 4 (_wf_dom):  domain only (semantic gate, no entity strictness)
// Tier 5 (_wf_soft): no SQL 4D WHERE — recall then BoostSemantic4D (soft filter)
//
// Optimization: tier 1 (strict) and tier 5 (soft) run concurrently.
// If tier 1 returns results, use them; otherwise fall back to tier 5,
// eliminating the worst-case 5-tier serial wait.
func (h *Hybrid) searchWithSem4DWaterfall(
	ctx context.Context,
	req model.SearchRequest,
	candidateK int,
	ftWeight, vecWeight float64,
	includeCold bool,
	sem4d *model.QuerySemantic4D,
	plan classifier.RoutePlan,
) ([]model.Chunk, string, *model.CandidateStats, error) {
	if sem4d == nil || !sem4d.Active() {
		chunks, stats, err := h.searchCandidates(ctx, req, candidateK, ftWeight, vecWeight, includeCold, nil, plan)
		return chunks, "", stats, err
	}

	// Optimistic concurrent: run strict (tier 1) and soft (tier 5) in parallel.
	type tierResult struct {
		chunks []model.Chunk
		suffix string
		stats  *model.CandidateStats
		err    error
	}
	var wg sync.WaitGroup
	strictCh := make(chan tierResult, 1)
	softCh := make(chan tierResult, 1)

	wg.Add(2)
	go func() {
		defer wg.Done()
		chunks, stats, err := h.searchCandidates(ctx, req, candidateK, ftWeight, vecWeight, includeCold, sem4d, plan)
		strictCh <- tierResult{chunks: chunks, suffix: "", stats: stats, err: err}
	}()
	go func() {
		defer wg.Done()
		chunks, stats, err := h.searchCandidates(ctx, req, candidateK, ftWeight, vecWeight, includeCold, nil, plan)
		softCh <- tierResult{chunks: chunks, suffix: "_wf_soft", stats: stats, err: err}
	}()

	// Wait for strict result first.
	strict := <-strictCh
	if strict.err != nil {
		// Strict failed; wait for soft.
		soft := <-softCh
		wg.Wait()
		if soft.err != nil {
			return nil, soft.suffix, soft.stats, soft.err
		}
		if len(soft.chunks) > 0 && sem4d.Active() {
			BoostSemantic4D(soft.chunks, *sem4d)
		}
		return soft.chunks, soft.suffix, soft.stats, nil
	}
	if len(strict.chunks) >= waterfallMinChunks {
		// Strict hit — drain soft channel and return.
		<-softCh
		wg.Wait()
		return strict.chunks, strict.suffix, strict.stats, nil
	}

	// Strict returned 0 results — use soft result.
	soft := <-softCh
	wg.Wait()
	if soft.err != nil {
		return nil, soft.suffix, soft.stats, soft.err
	}
	if len(soft.chunks) > 0 && sem4d.Active() {
		BoostSemantic4D(soft.chunks, *sem4d)
	}
	return soft.chunks, soft.suffix, soft.stats, nil
}

func sem4DFilterEqual(a, b *model.QuerySemantic4D) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}
	return a.CacheFingerprint() == b.CacheFingerprint()
}

func (h *Hybrid) searchCandidates(
	ctx context.Context,
	req model.SearchRequest,
	candidateK int,
	ftWeight, vecWeight float64,
	includeCold bool,
	sem4d *model.QuerySemantic4D,
	plan classifier.RoutePlan,
) ([]model.Chunk, *model.CandidateStats, error) {
	stats := &model.CandidateStats{}
	if plan.UseGlobalIndex && req.KBType == "" {
		if len(req.QueryEmbedding) > 0 {
			chunks, err := h.repo.SearchGlobalHybrid(ctx, req.Query, req.QueryEmbedding, candidateK, sem4d)
			stats.MergedUnique = len(chunks)
			stats.RRFKept = len(chunks)
			return chunks, stats, err
		}
		chunks, err := h.repo.SearchGlobalIndex(ctx, req.Query, candidateK, sem4d)
		stats.FulltextRaw = len(chunks)
		stats.MergedUnique = len(chunks)
		stats.RRFKept = len(chunks)
		return chunks, stats, err
	}
	hasVector := len(req.QueryEmbedding) > 0
	switch {
	case hasVector && ftWeight > 0 && vecWeight > 0:
		return h.searchHybrid(ctx, req, candidateK, ftWeight, vecWeight, includeCold, sem4d)
	case hasVector && vecWeight > 0:
		chunks, err := h.repo.SearchVector(ctx, req.QueryEmbedding, req.KBType, candidateK, includeCold, sem4d)
		for i := range chunks {
			chunks[i].VectorScore = chunks[i].Score
			chunks[i].Source = "vector"
		}
		stats.VectorRaw = len(chunks)
		stats.MergedUnique = len(chunks)
		stats.RRFKept = len(chunks)
		return chunks, stats, err
	default:
		chunks, err := h.repo.SearchFulltext(ctx, req.Query, req.KBType, candidateK, includeCold, req.ExpandedTerms, sem4d)
		stats.FulltextRaw = len(chunks)
		stats.MergedUnique = len(chunks)
		stats.RRFKept = len(chunks)
		return chunks, stats, err
	}
}
