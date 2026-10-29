package taxonomyevolve

import (
	"context"
	"strings"

	"github.com/graydraft/core/model"
)

// AlignEntitiesEmbedding maps entities to nearest taxonomy corpus term via embedding (Blueprint path).
func AlignEntitiesEmbedding(ctx context.Context, entities []string, tax *model.DomainTaxonomy) []string {
	if len(entities) == 0 || tax == nil {
		return entities
	}
	s := defaultSvc.Load()
	if s == nil {
		return entities
	}
	return s.alignEntitiesEmbedding(ctx, entities, tax)
}

func (s *Service) alignEntitiesEmbedding(ctx context.Context, entities []string, tax *model.DomainTaxonomy) []string {
	corpus := entityCorpusFromTaxonomy(tax)
	if len(corpus) == 0 {
		return entities
	}
	seen := map[string]bool{}
	out := make([]string, 0, len(entities))
	for _, e := range entities {
		canon := strings.TrimSpace(s.alignEntityToCorpus(ctx, e, corpus))
		if canon == "" {
			continue
		}
		key := strings.ToLower(canon)
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, canon)
	}
	if len(out) == 0 {
		return entities
	}
	return out
}

func entityCorpusFromTaxonomy(tax *model.DomainTaxonomy) []string {
	if tax == nil {
		return nil
	}
	seen := map[string]bool{}
	var out []string
	add := func(s string) {
		s = strings.TrimSpace(s)
		if s == "" || seen[s] {
			return
		}
		if len([]rune(s)) < 2 || len([]rune(s)) > 24 {
			return
		}
		seen[s] = true
		out = append(out, s)
	}
	for _, k := range tax.DetectKeywords {
		add(k)
	}
	for _, defs := range [][]model.LabelDefinition{tax.Scenes, tax.Styles} {
		for _, d := range defs {
			add(d.Label)
			for _, kw := range d.Keywords {
				add(kw)
			}
		}
	}
	return out
}

func (s *Service) alignEntityToCorpus(ctx context.Context, entity string, corpus []string) string {
	entity = strings.TrimSpace(entity)
	if entity == "" {
		return entity
	}
	for _, c := range corpus {
		if c == entity {
			return entity
		}
		if strings.Contains(entity, c) && len([]rune(c)) >= 2 {
			return c
		}
	}
	if s == nil || s.embedder == nil {
		return entity
	}
	minSim := 0.88
	if s.cfg != nil && s.cfg.TaxonomyEvolve.EmbedAlignMin > 0 {
		minSim = s.cfg.TaxonomyEvolve.EmbedAlignMin
	}
	qVec, err := s.embedder.EmbedQuery(ctx, entity)
	if err != nil {
		return entity
	}
	best := entity
	bestSim := minSim
	for _, c := range corpus {
		lVec, err := s.embedder.EmbedQuery(ctx, c)
		if err != nil {
			continue
		}
		sim := CosineSimilarity(qVec, lVec)
		if sim >= bestSim {
			bestSim = sim
			best = c
		}
	}
	if best != entity && (bestSim >= minSim) {
		if len([]rune(best)) <= len([]rune(entity)) || strings.Contains(strings.ToLower(entity), strings.ToLower(best)) {
			return best
		}
	}
	return entity
}
