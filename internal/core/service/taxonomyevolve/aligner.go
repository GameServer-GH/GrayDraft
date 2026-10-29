package taxonomyevolve

import (
	"context"
	"strings"
	"github.com/graydraft/core/model"
)

type alignVecCache struct {
	version int
	domain  string
	vecs    map[string][]float32 // key: dimension|label
}

// AlignScene maps free-text scene to a canonical label in taxonomy.
func AlignScene(ctx context.Context, raw string, tax *model.DomainTaxonomy) string {
	return alignDimension(ctx, "scene", raw, tax)
}

// AlignStyle maps free-text style to a canonical label in taxonomy.
func AlignStyle(ctx context.Context, raw string, tax *model.DomainTaxonomy) string {
	return alignDimension(ctx, "style", raw, tax)
}

// AlignQuery4D normalizes scene/style on a query 4D struct.
func AlignQuery4D(ctx context.Context, q model.QuerySemantic4D, tax *model.DomainTaxonomy) model.QuerySemantic4D {
	if tax == nil {
		return q
	}
	if s := strings.TrimSpace(q.Scene); s != "" {
		q.Scene = AlignScene(ctx, s, tax)
	}
	if s := strings.TrimSpace(q.Style); s != "" {
		q.Style = AlignStyle(ctx, s, tax)
	}
	return q
}

func alignDimension(ctx context.Context, dimension, raw string, tax *model.DomainTaxonomy) string {
	raw = strings.TrimSpace(raw)
	if raw == "" || tax == nil {
		return raw
	}
	defs := tax.Scenes
	if dimension == "style" {
		defs = tax.Styles
	}
	if len(defs) == 0 {
		return raw
	}
	for _, d := range defs {
		if d.Label == raw {
			return d.Label
		}
	}
	if tax.FromBlueprint {
		if s := defaultSvc.Load(); s != nil {
			if label, ok := s.alignByEmbedding(ctx, dimension, raw, tax); ok {
				return label
			}
		}
		if label, ratio := BestLabelSimilarity(raw, defs); ratio >= 0.72 {
			return label
		}
		return raw
	}
	if norm := model.GenericNormalize(raw, defs); norm != "" && norm != raw {
		return norm
	}
	if label, ratio := BestLabelSimilarity(raw, defs); ratio >= 0.72 {
		return label
	}
	if s := defaultSvc.Load(); s != nil {
		if label, ok := s.alignByEmbedding(ctx, dimension, raw, tax); ok {
			return label
		}
	}
	return raw
}

// BestLabelSimilarity scores raw against label strings and their keywords.
func BestLabelSimilarity(raw string, defs []model.LabelDefinition) (label string, ratio float64) {
	for _, d := range defs {
		if d.Label == "" {
			continue
		}
		if r := SimilarityRatio(raw, d.Label); r > ratio {
			ratio = r
			label = d.Label
		}
		for _, kw := range d.Keywords {
			if r := SimilarityRatio(raw, kw); r > ratio {
				ratio = r
				label = d.Label
			}
		}
	}
	return label, ratio
}

func (s *Service) alignByEmbedding(ctx context.Context, dimension, raw string, tax *model.DomainTaxonomy) (string, bool) {
	if s == nil || s.embedder == nil || tax == nil {
		return "", false
	}
	minSim := 0.88
	if s.cfg != nil && s.cfg.TaxonomyEvolve.EmbedAlignMin > 0 {
		minSim = s.cfg.TaxonomyEvolve.EmbedAlignMin
	}
	defs := tax.Scenes
	if dimension == "style" {
		defs = tax.Styles
	}
	if len(defs) == 0 {
		return "", false
	}
	s.alignMu.Lock()
	cache := s.alignCache[tax.Domain]
	if cache == nil || cache.version != taxVersionKey(tax) {
		cache = &alignVecCache{version: taxVersionKey(tax), domain: tax.Domain, vecs: map[string][]float32{}}
		s.alignCache[tax.Domain] = cache
	}
	s.alignMu.Unlock()

	qVec, err := s.embedder.EmbedQuery(ctx, raw)
	if err != nil {
		return "", false
	}
	best := ""
	bestSim := 0.0
	for _, d := range defs {
		if d.Label == "" {
			continue
		}
		key := dimension + "|" + d.Label
		lVec, ok := cache.vecs[key]
		if !ok {
			lVec, err = s.embedder.EmbedQuery(ctx, d.Label)
			if err != nil {
				continue
			}
			s.alignMu.Lock()
			cache.vecs[key] = lVec
			s.alignMu.Unlock()
		}
		if sim := CosineSimilarity(qVec, lVec); sim > bestSim {
			bestSim = sim
			best = d.Label
		}
	}
	if bestSim >= minSim {
		return best, true
	}
	return "", false
}

func taxVersionKey(tax *model.DomainTaxonomy) int {
	if tax == nil {
		return 0
	}
	return len(tax.Scenes)*1000 + len(tax.Styles)
}

// NoteUnaligned records a phrase that could not be aligned (for evolution / birth).
func (s *Service) NoteUnaligned(ctx context.Context, kbType, dimension, phrase, hintLabel string, tax *model.DomainTaxonomy) {
	if !s.enabled() || s.repo == nil || kbType == "" || phrase == "" {
		return
	}
	st, err := s.loadOrInitState(ctx, kbType, model.MappingDelta{})
	if err != nil || st == nil {
		return
	}
	if tax != nil && tax.HasKeyword(phrase) {
		return
	}
	label := hintLabel
	if label == "" {
		label, _ = BestLabelSimilarity(phrase, labelDefsForDim(tax, dimension))
	}
	s.upsertCandidate(st, phrase, dimension, label)
	_ = s.repo.SaveWorkspaceTaxonomy(ctx, st)
}

func labelDefsForDim(tax *model.DomainTaxonomy, dimension string) []model.LabelDefinition {
	if tax == nil {
		return nil
	}
	if dimension == "style" {
		return tax.Styles
	}
	return tax.Scenes
}
