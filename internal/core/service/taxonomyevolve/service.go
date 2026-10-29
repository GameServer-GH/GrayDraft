package taxonomyevolve

import (
	"context"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/graydraft/core/config"
	"github.com/graydraft/core/model"
	"github.com/graydraft/core/service/answercache"
	"github.com/graydraft/core/service/cache"
	"github.com/graydraft/core/service/embedder"
	"github.com/graydraft/core/service/llmclient"
)

// Service implements blueprint, Loop A (incremental) and Loop B (compress) taxonomy evolution.
type Service struct {
	repo     taxonomyRepo
	llm      *llmclient.Client
	embedder *embedder.Client
	cfg      *config.Config
	resolver *Resolver
	cache    *cache.Manager
	answer   *answercache.Manager

	mu                sync.Mutex
	compress          map[string]time.Time
	blueprintMu       sync.Mutex
	blueprintInflight map[string]bool
	alignMu           sync.RWMutex
	alignCache        map[string]*alignVecCache // key: domain
}

// NewService wires taxonomy evolution.
func NewService(repo taxonomyRepo, llm *llmclient.Client, emb *embedder.Client, cfg *config.Config) *Service {
	resolver := NewResolver(repo)
	s := &Service{
		repo:              repo,
		llm:               llm,
		embedder:          emb,
		cfg:               cfg,
		resolver:          resolver,
		compress:          make(map[string]time.Time),
		blueprintInflight: make(map[string]bool),
		alignCache:        make(map[string]*alignVecCache),
	}
	resolver.setBlueprintTrigger(s.MaybeEnqueueBlueprint)
	return s
}

// WithCaches enables cache invalidation after taxonomy/backfill mutations.
func (s *Service) WithCaches(cacheMgr *cache.Manager, answerCache *answercache.Manager) *Service {
	if s == nil {
		return nil
	}
	s.cache = cacheMgr
	s.answer = answerCache
	return s
}

func (s *Service) clearAlignCache() {
	if s == nil {
		return
	}
	s.alignMu.Lock()
	s.alignCache = make(map[string]*alignVecCache)
	s.alignMu.Unlock()
}

// Resolver returns the workspace taxonomy resolver.
func (s *Service) Resolver() *Resolver {
	if s == nil {
		return NewResolver(nil)
	}
	return s.resolver
}

func (s *Service) enabled() bool {
	return s != nil && s.cfg != nil && s.cfg.TaxonomyEvolve.Enabled
}

// EnqueueRecordDelta runs Loop A asynchronously (non-blocking ingest path).
func (s *Service) EnqueueRecordDelta(ctx context.Context, kbType string, delta model.MappingDelta) {
	if !s.enabled() || kbType == "" || len(delta.KeyPhrases) == 0 {
		return
	}
	go func() {
		bg, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		defer cancel()
		if err := s.RecordDelta(bg, kbType, delta); err != nil {
			slog.Warn("taxonomy evolve record delta", "kb_type", kbType, "error", err)
		}
	}()
}

// RecordDelta ingests mapping delta into candidate buffer and may promote keywords.
func (s *Service) RecordDelta(ctx context.Context, kbType string, delta model.MappingDelta) error {
	if !s.enabled() || s.repo == nil || kbType == "" {
		return nil
	}
	phrases := cleanPhrases(delta.KeyPhrases)
	if len(phrases) == 0 {
		return nil
	}
	st, err := s.loadOrInitState(ctx, kbType, delta)
	if err != nil {
		return err
	}
	profile, _ := s.repo.GetContextProfile(ctx, kbType)
	effective := s.resolver.Effective(ctx, kbType, &profile, "")
	if st.KeywordHits == nil {
		st.KeywordHits = map[string]int{}
	}
	promoteN := s.cfg.TaxonomyEvolve.CandidatePromoteDocCount
	if promoteN < 1 {
		promoteN = 5
	}
	changed := false
	for _, phrase := range phrases {
		if effective.HasKeyword(phrase) {
			continue
		}
		dim, label := s.dimensionForDelta(ctx, delta, phrase, effective)
		if label == "" {
			if hint, _ := BestLabelSimilarity(phrase, labelDefsForDim(effective, dim)); hint != "" {
				label = hint
			} else {
				s.upsertCandidate(st, phrase, dim, "")
				changed = true
				continue
			}
		}
		if s.upsertCandidate(st, phrase, dim, label) {
			changed = true
		}
		for i := range st.Candidates {
			c := &st.Candidates[i]
			if c.Phrase != phrase || c.Label != label || c.Dimension != dim {
				continue
			}
			if c.DocCount >= promoteN {
				if s.tryPromote(ctx, st, effective, c) {
					changed = true
					st.Candidates = removeCandidate(st.Candidates, i)
				}
			}
			break
		}
	}
	if s.tryLabelBirth(ctx, kbType, st, effective) {
		changed = true
	}
	if !changed {
		return nil
	}
	if err := s.repo.SaveWorkspaceTaxonomy(ctx, st); err != nil {
		return err
	}
	s.resolver.Invalidate(kbType)
	s.clearAlignCache()
	if len(st.Candidates) >= s.compressThreshold() {
		s.maybeScheduleCompress(kbType)
	}
	return nil
}

func (s *Service) loadOrInitState(ctx context.Context, kbType string, delta model.MappingDelta) (*model.WorkspaceTaxonomyState, error) {
	st, err := s.repo.GetWorkspaceTaxonomy(ctx, kbType)
	if err != nil {
		return nil, err
	}
	if st != nil {
		return st, nil
	}
	profile, _ := s.repo.GetContextProfile(ctx, kbType)
	key := model.DefaultCatalog().ResolveKey(&profile, "", "")
	if d := strings.TrimSpace(delta.Scene); d != "" {
		if norm := model.DefaultCatalog().Get(key).NormalizeScene(d); norm != "" {
			_ = norm
		}
	}
	st = &model.WorkspaceTaxonomyState{
		KBType:     kbType,
		Domain:     key,
		Version:    1,
		Candidates: []model.KeywordCandidate{},
		KeywordHits: map[string]int{},
	}
	return st, nil
}

func (s *Service) dimensionForDelta(ctx context.Context, delta model.MappingDelta, phrase string, tax *model.DomainTaxonomy) (dim, label string) {
	if sc := strings.TrimSpace(delta.Scene); sc != "" {
		return "scene", AlignScene(ctx, sc, tax)
	}
	if st := strings.TrimSpace(delta.Style); st != "" {
		return "style", AlignStyle(ctx, st, tax)
	}
	if tax != nil {
		if l := AlignScene(ctx, phrase, tax); l != phrase {
			return "scene", l
		}
		if l := AlignStyle(ctx, phrase, tax); l != phrase {
			return "style", l
		}
	}
	return "scene", ""
}

func (s *Service) upsertCandidate(st *model.WorkspaceTaxonomyState, phrase, dim, label string) bool {
	now := time.Now().UTC()
	for i := range st.Candidates {
		c := &st.Candidates[i]
		if c.Phrase == phrase && c.Dimension == dim && c.Label == label {
			c.DocCount++
			c.LastSeen = now
			return true
		}
	}
	st.Candidates = append(st.Candidates, model.KeywordCandidate{
		Phrase: phrase, Dimension: dim, Label: label,
		DocCount: 1, FirstSeen: now, LastSeen: now,
	})
	return true
}

func (s *Service) tryPromote(ctx context.Context, st *model.WorkspaceTaxonomyState, tax *model.DomainTaxonomy, c *model.KeywordCandidate) bool {
	existing := keywordsForLabel(tax, c.Dimension, c.Label)
	overlay := overlayKeywordsForLabel(st, c.Dimension, c.Label)
	existing = append(existing, overlay...)

	_, ratio := BestKeywordMatch(c.Phrase, existing)
	high := s.cfg.TaxonomyEvolve.SimilarityMergeMin
	low := s.cfg.TaxonomyEvolve.SimilarityDistinctMax
	if high <= 0 {
		high = 0.85
	}
	if low <= 0 {
		low = 0.5
	}
	if ratio >= high {
		return true // duplicate — drop candidate
	}
	if ratio > low && ratio < high && s.cfg.TaxonomyEvolve.LLMMergeEnabled && s.llm != nil {
		action, _, err := resolveAmbiguousKeyword(ctx, s.llm, c.Phrase, c.Label, c.Dimension, existing)
		if err == nil && action == "ignore" {
			return true
		}
		if err == nil && action == "merge" {
			return true
		}
	}
	maxKW := s.cfg.TaxonomyEvolve.MaxKeywordsPerLabel
	if maxKW < 1 {
		maxKW = 10
	}
	if len(overlay) >= maxKW {
		return false
	}
	s.appendOverlayKeyword(st, c.Dimension, c.Label, c.Phrase)
	st.Version++
	return true
}

func (s *Service) appendOverlayKeyword(st *model.WorkspaceTaxonomyState, dim, label, phrase string) {
	defs := &st.Scenes
	if dim == "style" {
		defs = &st.Styles
	}
	for i := range *defs {
		if (*defs)[i].Label == label {
			(*defs)[i].Keywords = uniqueAppend((*defs)[i].Keywords, phrase)
			return
		}
	}
	*defs = append(*defs, model.LabelDefinition{Label: label, Keywords: []string{phrase}})
}

func (s *Service) compressThreshold() int {
	if s.cfg == nil || s.cfg.TaxonomyEvolve.CompressCandidateThreshold < 1 {
		return 20
	}
	return s.cfg.TaxonomyEvolve.CompressCandidateThreshold
}

func (s *Service) maybeScheduleCompress(kbType string) {
	s.mu.Lock()
	if last, ok := s.compress[kbType]; ok && time.Since(last) < 24*time.Hour {
		s.mu.Unlock()
		return
	}
	s.compress[kbType] = time.Now()
	s.mu.Unlock()
	go func() {
		bg, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
		defer cancel()
		if err := s.Compress(bg, kbType); err != nil {
			slog.Warn("taxonomy compress", "kb_type", kbType, "error", err)
		}
	}()
}

// Compress runs Loop B: LLM merges candidates into a slim overlay.
func (s *Service) Compress(ctx context.Context, kbType string) error {
	if !s.enabled() || s.repo == nil || !s.cfg.TaxonomyEvolve.CompressEnabled {
		return nil
	}
	st, err := s.repo.GetWorkspaceTaxonomy(ctx, kbType)
	if err != nil || st == nil || len(st.Candidates) == 0 {
		return err
	}
	profile, _ := s.repo.GetContextProfile(ctx, kbType)
	effective := s.resolver.Effective(ctx, kbType, &profile, "")
	if s.llm == nil {
		return nil
	}
	maxKW := s.cfg.TaxonomyEvolve.MaxKeywordsPerLabel
	if maxKW < 1 {
		maxKW = 10
	}
	scenes, styles, err := compressMappingLLM(ctx, s.llm, effective, st.Candidates, maxKW)
	if err != nil {
		return err
	}
	st.Scenes = trimDefs(scenes, maxKW)
	st.Styles = trimDefs(styles, maxKW)
	st.Candidates = nil
	st.Version++
	if err := s.repo.SaveWorkspaceTaxonomy(ctx, st); err != nil {
		return err
	}
	s.resolver.Invalidate(kbType)
	s.clearAlignCache()
	slog.Info("taxonomy compressed", "kb_type", kbType, "version", st.Version)
	return nil
}

func trimDefs(defs []model.LabelDefinition, maxKW int) []model.LabelDefinition {
	out := make([]model.LabelDefinition, 0, len(defs))
	for _, d := range defs {
		if d.Label == "" {
			continue
		}
		kw := d.Keywords
		if len(kw) > maxKW {
			kw = kw[:maxKW]
		}
		out = append(out, model.LabelDefinition{Label: d.Label, Keywords: kw})
	}
	return out
}

func keywordsForLabel(tax *model.DomainTaxonomy, dim, label string) []string {
	if tax == nil {
		return nil
	}
	defs := tax.Scenes
	if dim == "style" {
		defs = tax.Styles
	}
	for _, d := range defs {
		if d.Label == label {
			return append([]string(nil), d.Keywords...)
		}
	}
	return nil
}

func overlayKeywordsForLabel(st *model.WorkspaceTaxonomyState, dim, label string) []string {
	if st == nil {
		return nil
	}
	return st.OverlayKeywords(dim, label)
}

func cleanPhrases(in []string) []string {
	var out []string
	seen := map[string]bool{}
	for _, p := range in {
		p = strings.TrimSpace(p)
		if p == "" || len([]rune(p)) > 32 || seen[p] {
			continue
		}
		seen[p] = true
		out = append(out, p)
		if len(out) >= 3 {
			break
		}
	}
	return out
}

func uniqueAppend(dst []string, v string) []string {
	for _, s := range dst {
		if s == v {
			return dst
		}
	}
	return append(dst, v)
}

func removeCandidate(cands []model.KeywordCandidate, idx int) []model.KeywordCandidate {
	if idx < 0 || idx >= len(cands) {
		return cands
	}
	return append(cands[:idx], cands[idx+1:]...)
}
