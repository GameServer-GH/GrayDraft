package retriever

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/graydraft/core/config"
	"github.com/graydraft/core/model"
	"github.com/graydraft/core/observability"
	"github.com/graydraft/core/repository/postgres"
	"github.com/graydraft/core/service/cache"
	"github.com/graydraft/core/service/classifier"
	"github.com/graydraft/core/service/confidence"
	"github.com/graydraft/core/service/embedder"
	"github.com/graydraft/core/service/llmclient"
	"github.com/graydraft/core/service/reranker"
	"github.com/graydraft/core/service/taxonomyevolve"
	"github.com/graydraft/core/service/workspacecontext"
)

// Hybrid combines fulltext and vector recall with RRF fusion, Worker rerank, and tiered cache.
type Hybrid struct {
	repo       *postgres.ChunkRepo
	cfg        *config.Config
	constK     int
	embedder   *embedder.Client
	reranker   *reranker.Client
	cache      *cache.Manager
	classifier *classifier.Classifier
	llm        *llmclient.Client
}

// NewHybrid creates a hybrid retriever.
func NewHybrid(
	repo *postgres.ChunkRepo,
	cfg *config.Config,
	emb *embedder.Client,
	rr *reranker.Client,
	cm *cache.Manager,
	clf *classifier.Classifier,
	llm *llmclient.Client,
) *Hybrid {
	return &Hybrid{
		repo:       repo,
		cfg:        cfg,
		constK:     cfg.Retrieval.RRFConstantK,
		embedder:   emb,
		reranker:   rr,
		cache:      cm,
		classifier: clf,
		llm:        llm,
	}
}

// Search runs adaptive hybrid retrieval with phased cache lookup:
// Phase 1: exact cache (L1 + L2 exact) — avoids embedding API call on hit.
// Phase 2: compute query embedding only when needed.
// Phase 3: semantic cache (L2 embedding similarity) with computed embedding.
func (h *Hybrid) Search(ctx context.Context, req model.SearchRequest) (resp *model.SearchResponse, err error) {
	start := time.Now()
	defer func() {
		rec := observability.RetrievalRecord{
			KBType:   req.KBType,
			Duration: time.Since(start),
			Err:      err,
		}
		if resp != nil {
			rec.CacheHit = resp.CacheHit
			rec.QueryIntent = resp.QueryIntent
			rec.Strategy = resp.Strategy
			rec.Chunks = resp.Total
		}
		observability.LogRetrieval(rec)
	}()

	topK := req.TopK
	if topK <= 0 {
		topK = h.cfg.Retrieval.DefaultTopK
	}
	if topK > h.cfg.Retrieval.MaxTopK {
		topK = h.cfg.Retrieval.MaxTopK
	}

	candidateK := h.cfg.Retrieval.RerankL1Pool
	if len(req.KBTypes) > 1 {
		plan := classifier.RoutePlan{
			FulltextWeight: h.cfg.Retrieval.FulltextWeight,
			VectorWeight:   h.cfg.Retrieval.VectorWeight,
		}
		if !req.SkipClassifier && h.classifier != nil {
			plan = h.classifier.ClassifyAndRoute(req.Query)
		}
		return h.searchMultiKB(ctx, req, topK, candidateK, plan)
	}
	if candidateK < topK*3 {
		candidateK = topK * 5
	}
	if candidateK < 25 {
		candidateK = 25
	}
	if candidateK > 100 {
		candidateK = 100
	}

	plan := classifier.RoutePlan{
		FulltextWeight: h.cfg.Retrieval.FulltextWeight,
		VectorWeight:   h.cfg.Retrieval.VectorWeight,
	}
	if !req.SkipClassifier && h.classifier != nil {
		plan = h.classifier.ClassifyAndRoute(req.Query)
		plan = classifier.ApplyUserOverrides(plan, req.FulltextWeight, req.VectorWeight, boolPtr(req.UseGlobalIndex))
	} else if req.FulltextWeight != nil || req.VectorWeight != nil {
		plan = classifier.ApplyUserOverrides(plan, req.FulltextWeight, req.VectorWeight, boolPtr(req.UseGlobalIndex))
	}
	if req.UseGlobalIndex {
		plan.UseGlobalIndex = true
	}

	ftWeight := plan.FulltextWeight
	vecWeight := plan.VectorWeight

	aliasIdx, _ := h.repo.LoadEntityAliasIndex(ctx, req.KBType)
	expandedTerms := req.ExpandedTerms
	if len(expandedTerms) == 0 && aliasIdx != nil {
		expandedTerms = aliasIdx.ExpandTerms(req.Query)
	}
	req.ExpandedTerms = expandedTerms

	profile := model.ContextProfile{}
	if req.ContextProfile != nil {
		profile = *req.ContextProfile
	}
	wcText := workspacecontext.ResolveWorkspaceContextText(profile, req.WorkspaceContext)
	tax := taxonomyevolve.Effective(ctx, req.KBType, &profile, wcText)

	sem4d := req.Semantic4D
	semanticFilterApplied := false
	var trace model.RetrievalTrace
	if sem4d == nil && h.cfg.Retrieval.Semantic4DFilterEnabled {
		taxSeg := taxonomyevolve.CacheKeySegment(ctx, req.KBType, &profile, wcText)
		sem4d = ptrQuery4D(ExtractQuery4DCached(ctx, req.Query, h.cfg, h.llm, taxSeg, tax))
	}
	if sem4d != nil && sem4d.Active() {
		clean := *sem4d
		trace.Query4DMethod = clean.Method
		sceneOrig, styleOrig := clean.Scene, clean.Style
		clean.Entities = model.FinalSanitizeWithMethod(clean.Entities, req.Query, clean.Method)
		if strings.HasPrefix(strings.TrimSpace(req.KBType), "gd_") {
			if pool := h.globalEntityPool(ctx, "gd_", 120); len(pool) > 0 {
				clean.Entities = augmentEntitiesFromGlobalPool(req.Query, clean.Entities, pool)
			}
		}
		entityAlign := "none"
		if aliasIdx != nil {
			before := len(clean.Entities)
			clean.Entities = aliasIdx.CanonicalizeEntities(clean.Entities)
			if before > 0 && len(clean.Entities) > 0 {
				entityAlign = "alias"
			}
		}
		if tax != nil && tax.FromBlueprint {
			before := append([]string(nil), clean.Entities...)
			clean.Entities = taxonomyevolve.AlignEntitiesEmbedding(ctx, clean.Entities, tax)
			if len(before) > 0 && len(clean.Entities) > 0 {
				if entityAlign == "alias" {
					entityAlign = "alias+embedding"
				} else {
					entityAlign = "embedding"
				}
			}
		}
		norm := model.NormalizeQuerySemantic4D(clean, tax)
		if tax != nil {
			if d := strings.TrimSpace(tax.Domain); d != "" && d != model.UniversalDomainKey && d != model.DefaultCatalog().DefaultDomainKey() {
				norm.Domain = d
			}
		}
		aligned := taxonomyevolve.AlignQuery4D(ctx, norm, tax)
		trace.SceneOriginal = sceneOrig
		trace.StyleOriginal = styleOrig
		trace.SceneAligned = aligned.Scene
		trace.StyleAligned = aligned.Style
		trace.EntitiesFinal = append([]string(nil), aligned.Entities...)
		trace.EntityAlign = entityAlign
		sem4d = &aligned
		semanticFilterApplied = true
		req.Semantic4D = sem4d
	}

	catalogMeta, _ := h.repo.GetCatalogMeta(ctx, req.KBType)
	sem4dFP := ""
	if semanticFilterApplied && sem4d != nil {
		sem4dFP = sem4d.CacheFingerprint()
	}
	cacheParams := cache.LookupParams{
		Query:                 req.Query,
		KBType:                req.KBType,
		TopK:                  topK,
		UseGlobalIndex:        plan.UseGlobalIndex,
		CatalogVersion:        catalogMeta.Version,
		CatalogUpdated:        catalogMeta.LatestDocUpdatedAt,
		QueryIntent:           string(plan.Intent),
		Semantic4DFingerprint: sem4dFP,
	}

	// Phase 1: exact cache lookup (L1 + L2 exact) — no embedding needed, skips API call on hit.
	if !req.SkipCache && h.cache != nil {
		if cached, tier, ok := h.cache.Lookup(ctx, cacheParams, nil); ok {
			if cached.Total > 0 || !semanticFilterApplied {
				resp = h.decorateCacheHit(cached, req, topK, catalogMeta, plan, ftWeight, vecWeight, tier)
				return resp, nil
			}
		}
	}

	// Phase 2: compute query embedding only when needed for vector search or semantic cache.
	embedQuery := req.Query
	if wcText != "" {
		embedQuery = wcText + "\n" + embedQuery
	}
	if len(req.QueryEmbedding) == 0 && h.cfg.Retrieval.AutoEmbed && h.embedder != nil {
		vec, err := h.embedder.EmbedQuery(ctx, embedQuery)
		if err != nil {
			slog.Warn("auto embed failed, fallback to fulltext", "error", err)
		} else {
			req.QueryEmbedding = vec
		}
	}

	// Phase 3: semantic cache lookup (L2 embedding similarity) — needs computed embedding.
	if !req.SkipCache && h.cache != nil && len(req.QueryEmbedding) > 0 {
		if cached, tier, ok := h.cache.Lookup(ctx, cacheParams, req.QueryEmbedding); ok {
			if cached.Total > 0 || !semanticFilterApplied {
				resp = h.decorateCacheHit(cached, req, topK, catalogMeta, plan, ftWeight, vecWeight, tier)
				return resp, nil
			}
			slog.Debug("cache skip empty sem4d", "query", req.Query)
		}
	}

	strategy := string(plan.Intent)
	var chunks []model.Chunk
	var wfSuffix string
	var rawStats *model.CandidateStats
	includeCold := req.IncludeCold

	if semanticFilterApplied {
		strategy = "hybrid_rrf+" + strategy + "+sem4d"
		chunks, wfSuffix, rawStats, err = h.searchWithSem4DWaterfall(ctx, req, candidateK, ftWeight, vecWeight, includeCold, sem4d, plan)
	} else {
		switch {
		case plan.UseGlobalIndex && req.KBType == "":
			strategy = "global_index+" + strategy
			if len(req.QueryEmbedding) > 0 {
				chunks, err = h.repo.SearchGlobalHybrid(ctx, req.Query, req.QueryEmbedding, candidateK, sem4d)
			} else {
				chunks, err = h.repo.SearchGlobalIndex(ctx, req.Query, candidateK, sem4d)
			}
		default:
			chunks, rawStats, err = h.searchCandidates(ctx, req, candidateK, ftWeight, vecWeight, includeCold, sem4d, plan)
			if len(req.QueryEmbedding) > 0 && ftWeight > 0 && vecWeight > 0 {
				strategy = "hybrid_rrf+" + strategy
			} else if len(req.QueryEmbedding) > 0 && vecWeight > 0 {
				strategy = "vector+" + strategy
			} else {
				strategy = "fulltext+" + strategy
			}
		}
	}
	if wfSuffix != "" {
		strategy += wfSuffix
	}
	if semanticFilterApplied {
		trace.WaterfallSuffix = wfSuffix
		trace.WaterfallTier = model.WaterfallTierFromSuffix(wfSuffix)
	}
	if err != nil {
		return nil, err
	}
	chunks = enrichChunksWithMedia(chunks)
	if plan.GlobalIndexBoost && req.KBType != "" {
		boostK := candidateK / 3
		if boostK < 10 {
			boostK = 10
		}
		globalChunks, gerr := h.repo.SearchGlobalIndex(ctx, req.Query, boostK, sem4d)
		if gerr == nil && len(globalChunks) > 0 {
			gw := h.cfg.Classifier.GlobalBoostWeight
			if gw <= 0 {
				gw = 0.35
			}
			chunks = rrfFuseWeighted(h.constK, 1.0, gw, chunks, globalChunks)
			strategy += "+global_boost"
		}
	}


	// V1-21: capture pre-rerank candidate count for diagnostics
	preRerankCount := len(chunks)
	if h.cfg.Retrieval.RerankEnabled && h.reranker != nil && len(chunks) > 0 {
		if shouldSkipRerank(chunks, topK, sem4d, h.cfg) {
			if rawStats != nil {
				rawStats.RerankSkipped = 1
			}
			if len(chunks) > topK {
				chunks = chunks[:topK]
			}
			strategy += "+rerank_skip_confident"
		} else {
			if rawStats != nil {
				rawStats.RerankCalls = 1
			}
			reranked, rerr := h.reranker.Rerank(ctx, req.Query, chunks, topK)
			if rerr != nil {
				if rawStats != nil {
					rawStats.RerankFailed = 1
				}
				slog.Warn("rerank degraded", "error", rerr)
				strategy += "+rerank_skip"
				if len(chunks) > topK {
					chunks = chunks[:topK]
				}
			} else {
				chunks = reranked
				strategy += "+rerank_l1_l2"
			}
		}
	} else if len(chunks) > topK {
		chunks = chunks[:topK]
	}

	h.annotateRetrievalScores(chunks, req.Query, aliasIdx)
	if sem4d != nil {
		BoostSemantic4D(chunks, *sem4d)
	}
	if !profile.IsEmpty() {
		h.applyEntityMatchFlags(chunks, req.Query, profile)
	}

	if chunks == nil {
		chunks = []model.Chunk{}
	}

	// Async: touch document access timestamps without blocking the response.
	touchCtx, cancelTouch := context.WithTimeout(context.WithoutCancel(ctx), 2*time.Second)
	go func() {
		defer cancelTouch()
		h.touchAccess(touchCtx, chunks)
	}()

	resp = &model.SearchResponse{
		Query:            req.Query,
		KBType:           req.KBType,
		TopK:             topK,
		Total:            len(chunks),
		Chunks:           chunks,
		Strategy:         strategy,
		QueryIntent:      string(plan.Intent),
		FulltextWeight:   ftWeight,
		VectorWeight:     vecWeight,
		CatalogVer:       catalogMeta.Version,
		QuerySemantic4D:  sem4d,
		Semantic4DFilter: semanticFilterApplied,
		RetrievalTrace:   copyRetrievalTrace(trace),

		// V1-21: per-stage counts visible in the returned candidate set.
		// Raw backend counts are captured from source labels; they are not
		// replaced with the configured candidateK.
		CandidateStats: &model.CandidateStats{
			FulltextRaw:   rawStatsValue(rawStats, func(s *model.CandidateStats) int { return s.FulltextRaw }),
			VectorRaw:     rawStatsValue(rawStats, func(s *model.CandidateStats) int { return s.VectorRaw }),
			StructuredRaw: rawStatsValue(rawStats, func(s *model.CandidateStats) int { return s.StructuredRaw }),
			MergedUnique:  rawStatsValue(rawStats, func(s *model.CandidateStats) int { return s.MergedUnique }),
			RRFKept:       rawStatsValue(rawStats, func(s *model.CandidateStats) int { return s.RRFKept }),
			RerankInput:   preRerankCount,
			RerankOutput:  len(chunks),
			WaterfallTier: trace.WaterfallTier,
			RerankCalls:   rawStatsValue(rawStats, func(s *model.CandidateStats) int { return s.RerankCalls }),
			RerankSkipped: rawStatsValue(rawStats, func(s *model.CandidateStats) int { return s.RerankSkipped }),
			RerankFailed:  rawStatsValue(rawStats, func(s *model.CandidateStats) int { return s.RerankFailed }),
		},
	}
	if semanticFilterApplied && trace.WaterfallTier > 0 {
		slog.Info("retrieval trace",
			"kb_type", req.KBType,
			"wf_tier", trace.WaterfallTier,
			"wf_suffix", trace.WaterfallSuffix,
			"entities", trace.EntitiesFinal,
			"scene", trace.SceneAligned,
			"entity_align", trace.EntityAlign,
			"q4d_method", trace.Query4DMethod,
		)
	}

	if !req.SkipCache && h.cache != nil && !(resp.Total == 0 && resp.Semantic4DFilter) {
		h.cache.Store(ctx, cacheParams, req.QueryEmbedding, resp)
	}

	return resp, nil
}

// decorateCacheHit fills metadata fields on a cache-hit response before returning it.
func (h *Hybrid) decorateCacheHit(
	cached *model.SearchResponse,
	req model.SearchRequest,
	topK int,
	catalogMeta postgres.CatalogMeta,
	plan classifier.RoutePlan,
	ftWeight, vecWeight float64,
	tier string,
) *model.SearchResponse {
	cached.Query = req.Query
	cached.KBType = req.KBType
	cached.TopK = topK
	cached.CatalogVer = catalogMeta.Version
	cached.QueryIntent = string(plan.Intent)
	cached.FulltextWeight = ftWeight
	cached.VectorWeight = vecWeight
	if cached.Strategy == "" || cached.CacheHit != "" {
		cached.Strategy = "cache_" + tier
	}
	slog.Debug("cache hit", "tier", tier, "intent", plan.Intent)
	return cached
}

func (h *Hybrid) globalEntityPool(ctx context.Context, scope string, limit int) []string {
	if h == nil || h.repo == nil {
		return nil
	}
	if h.cache != nil {
		if pool, ok := h.cache.GetEntityPool(ctx, scope); ok {
			if limit > 0 && len(pool) > limit {
				return pool[:limit]
			}
			return pool
		}
	}
	pool, err := h.repo.ListDistinctSemanticEntities(ctx, scope, limit)
	if err != nil || len(pool) == 0 {
		return nil
	}
	if h.cache != nil {
		ttl := time.Hour
		if h.cfg != nil && h.cfg.Cache.EntityPoolTTLSeconds > 0 {
			ttl = time.Duration(h.cfg.Cache.EntityPoolTTLSeconds) * time.Second
		}
		h.cache.SetEntityPool(ctx, scope, pool, ttl)
	}
	return pool
}

func ptrQuery4D(q model.QuerySemantic4D) *model.QuerySemantic4D {
	if !q.Active() {
		return nil
	}
	return &q
}

func (h *Hybrid) searchHybrid(
	ctx context.Context,
	req model.SearchRequest,
	candidateK int,
	ftWeight, vecWeight float64,
	includeCold bool,
	sem4d *model.QuerySemantic4D,
) ([]model.Chunk, *model.CandidateStats, error) {
	stats := &model.CandidateStats{}
	var (
		ftChunks  []model.Chunk
		vecChunks []model.Chunk
		stChunks  []model.Chunk
		ftErr     error
		vecErr    error
		stErr     error
		wg        sync.WaitGroup
	)

	stWeight := h.cfg.Retrieval.StructuredVectorWeight
	if stWeight <= 0 {
		stWeight = h.cfg.Multimodal.StructuredVectorWeight
	}

	// Run all three search legs concurrently.
	numTasks := 2
	if stWeight > 0 && len(req.QueryEmbedding) > 0 {
		numTasks = 3
	}
	wg.Add(numTasks)
	go func() {
		defer wg.Done()
		if ftWeight > 0 {
			ftChunks, ftErr = h.repo.SearchFulltext(ctx, req.Query, req.KBType, candidateK, includeCold, req.ExpandedTerms, sem4d)
		}
	}()
	go func() {
		defer wg.Done()
		if vecWeight > 0 {
			vecChunks, vecErr = h.repo.SearchVector(ctx, req.QueryEmbedding, req.KBType, candidateK, includeCold, sem4d)
			for i := range vecChunks {
				vecChunks[i].VectorScore = vecChunks[i].Score
				vecChunks[i].Source = "vector"
			}
		}
	}()
	if stWeight > 0 && len(req.QueryEmbedding) > 0 {
		go func() {
			defer wg.Done()
			stChunks, stErr = h.repo.SearchVectorStructured(ctx, req.QueryEmbedding, req.KBType, candidateK, includeCold, sem4d)
		}()
	}
	wg.Wait()

	if ftErr != nil {
		return nil, stats, fmt.Errorf("fulltext: %w", ftErr)
	}
	if vecErr != nil {
		return nil, stats, fmt.Errorf("vector: %w", vecErr)
	}
	if stErr != nil {
		slog.Warn("structured vector search failed", "error", stErr)
	}

	var fused []model.Chunk
	stats.FulltextRaw = len(ftChunks)
	stats.VectorRaw = len(vecChunks)
	stats.StructuredRaw = len(stChunks)
	if len(stChunks) > 0 && stWeight > 0 {
		fused = rrfFuseWeightedN(h.constK, []float64{ftWeight, vecWeight, stWeight}, [][]model.Chunk{ftChunks, vecChunks, stChunks})
	} else {
		fused = rrfFuseWeighted(h.constK, ftWeight, vecWeight, ftChunks, vecChunks)
	}
	for i := range fused {
		if fused[i].Source == "" {
			fused[i].Source = "hybrid"
		}
	}
	stats.MergedUnique = len(fused)
	stats.RRFKept = len(fused)
	return fused, stats, nil
}

func rrfFuseWeighted(constK int, w1, w2 float64, list1, list2 []model.Chunk) []model.Chunk {
	return rrfFuseWeightedN(constK, []float64{w1, w2}, [][]model.Chunk{list1, list2})
}

func rrfFuseWeightedN(constK int, weights []float64, lists [][]model.Chunk) []model.Chunk {
	scores := make(map[string]float64)
	chunks := make(map[string]model.Chunk)

	for li, list := range lists {
		w := 1.0
		if li < len(weights) {
			w = weights[li]
		}
		for i, c := range list {
			key := c.ID + ":" + c.KBType
			scores[key] += w / float64(constK+i+1)
			if prev, ok := chunks[key]; ok {
				if c.VectorScore > prev.VectorScore {
					prev.VectorScore = c.VectorScore
				}
				chunks[key] = prev
			} else {
				chunks[key] = c
			}
		}
	}

	type ranked struct {
		key   string
		score float64
	}
	var order []ranked
	for k, s := range scores {
		order = append(order, ranked{k, s})
	}
	for i := 0; i < len(order); i++ {
		for j := i + 1; j < len(order); j++ {
			if order[j].score > order[i].score {
				order[i], order[j] = order[j], order[i]
			}
		}
	}

	out := make([]model.Chunk, 0, len(order))
	for _, r := range order {
		c := chunks[r.key]
		c.Score = r.score
		out = append(out, c)
	}
	return out
}

func boolPtr(v bool) *bool {
	if v {
		return &v
	}
	return nil
}

func rawStatsValue(s *model.CandidateStats, f func(*model.CandidateStats) int) int {
	if s == nil {
		return 0
	}
	return f(s)
}

func (h *Hybrid) touchAccess(ctx context.Context, chunks []model.Chunk) {
	if len(chunks) == 0 {
		return
	}
	seen := make(map[string]struct{})
	var docIDs []string
	for _, c := range chunks {
		if c.DocID == "" {
			continue
		}
		if _, ok := seen[c.DocID]; ok {
			continue
		}
		seen[c.DocID] = struct{}{}
		docIDs = append(docIDs, c.DocID)
	}
	h.repo.TouchDocumentAccess(ctx, docIDs)
}

func (h *Hybrid) annotateRetrievalScores(chunks []model.Chunk, query string, _ *classifier.AliasIndex) {
	for i := range chunks {
		if chunks[i].RerankScore <= 0 {
			chunks[i].RerankScore = chunks[i].Score
		}
		ext := map[string]any(chunks[i].Extensions)
		hit := confidence.TitleHit(query, chunks[i].DocTitle, ext)
		chunks[i].TitleMatched = hit
		chunks[i].EntityMatched = hit
	}
}

func (h *Hybrid) applyEntityMatchFlags(chunks []model.Chunk, query string, profile model.ContextProfile) {
	if profile.IsEmpty() || len(chunks) == 0 {
		return
	}
	q := strings.ToLower(strings.TrimSpace(query))
	for i := range chunks {
		body := strings.ToLower(chunks[i].Content + " " + chunks[i].DocTitle)
		for _, ent := range profile.Entities {
			el := strings.ToLower(strings.TrimSpace(ent))
			if el == "" {
				continue
			}
			if strings.Contains(q, el) && strings.Contains(body, el) {
				chunks[i].EntityMatched = true
				if chunks[i].Score > 0 {
					chunks[i].Score = min(1.0, chunks[i].Score+0.05)
				}
				break
			}
		}
	}
}

func copyRetrievalTrace(t model.RetrievalTrace) *model.RetrievalTrace {
	if t.WaterfallTier == 0 && t.Query4DMethod == "" && len(t.EntitiesFinal) == 0 {
		return nil
	}
	cp := t
	if len(t.EntitiesFinal) > 0 {
		cp.EntitiesFinal = append([]string(nil), t.EntitiesFinal...)
	}
	return &cp
}

// Diagnose runs normal retrieval and returns a compact no-result/debug report.
func (h *Hybrid) Diagnose(ctx context.Context, req model.SearchRequest) (*model.RetrievalDiagnosis, error) {
	resp, err := h.Search(ctx, req)
	if err != nil {
		return nil, err
	}
	gate := confidence.EvaluateRetrievalGate(req.Query, resp.QueryIntent, resp.Chunks)
	out := &model.RetrievalDiagnosis{
		Query:            req.Query,
		KBType:           resp.KBType,
		Strategy:         resp.Strategy,
		CacheHit:         resp.CacheHit,
		Total:            resp.Total,
		QueryIntent:      resp.QueryIntent,
		QuerySemantic4D:  resp.QuerySemantic4D,
		Semantic4DFilter: resp.Semantic4DFilter,
		RetrievalTrace:   resp.RetrievalTrace,
		CandidateStats:   resp.CandidateStats,
		Gate: model.RetrievalGateDebug{
			Useful:    gate.Useful,
			Score:     gate.Score,
			Threshold: gate.Threshold,
			Reason:    gate.Reason,
		},
	}
	for i, c := range resp.Chunks {
		if i >= 5 {
			break
		}
		out.TopChunks = append(out.TopChunks, model.RetrievalChunkDebug{
			Rank:           i + 1,
			ChunkID:        c.ID,
			DocID:          c.DocID,
			DocTitle:       c.DocTitle,
			Score:          c.Score,
			RerankScore:    c.RerankScore,
			VectorScore:    c.VectorScore,
			QualityScore:   c.QualityScore,
			TitleMatched:   c.TitleMatched,
			EntityMatched:  c.EntityMatched,
			Source:         c.Source,
			LineRange:      c.Extensions["line_range"],
			SemanticTags4D: c.Extensions["semantic_tags_4d"],
			SearchAnchors:  debugStringList(c.Extensions["search_anchors"]),
			Snippet:        debugSnippet(c.Content, 240),
		})
	}
	return out, nil
}

func debugSnippet(s string, maxRunes int) string {
	s = strings.TrimSpace(s)
	r := []rune(s)
	if len(r) <= maxRunes {
		return s
	}
	return string(r[:maxRunes]) + "..."
}

func debugStringList(v any) []string {
	switch t := v.(type) {
	case []string:
		return append([]string(nil), t...)
	case []any:
		var out []string
		for _, item := range t {
			if s, ok := item.(string); ok && strings.TrimSpace(s) != "" {
				out = append(out, strings.TrimSpace(s))
			}
		}
		return out
	default:
		return nil
	}
}
