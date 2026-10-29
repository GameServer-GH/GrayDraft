package retriever

import (
	"strings"

	"github.com/graydraft/core/config"
	"github.com/graydraft/core/model"
)

// shouldSkipRerank skips expensive L2 rerank when top recall is already confident and unambiguous.
func shouldSkipRerank(chunks []model.Chunk, topK int, sem4d *model.QuerySemantic4D, cfg *config.Config) bool {
	if cfg == nil || len(chunks) == 0 || topK <= 0 {
		return false
	}
	minScore := cfg.Retrieval.RerankSkipMinScore
	if minScore <= 0 {
		minScore = 0.78
	}
	top := chunks[0]
	if top.Score < minScore {
		return false
	}
	if sem4d != nil && len(sem4d.Entities) > 0 {
		if !chunkMatchesQueryEntities(top, sem4d.Entities) {
			return false
		}
	}
	if len(chunks) > 1 {
		gap := top.Score - chunks[1].Score
		if gap < 0.06 {
			return false
		}
	}
	return true
}

func chunkMatchesQueryEntities(ch model.Chunk, queryEnts []string) bool {
	ext, _ := ch.Extensions["semantic_tags_4d"].(map[string]any)
	if ext == nil {
		return true
	}
	raw, _ := ext["entities"].([]any)
	if len(raw) == 0 {
		return true
	}
	for _, qe := range queryEnts {
		qe = strings.TrimSpace(qe)
		if qe == "" {
			continue
		}
		for _, item := range raw {
			ent, _ := item.(string)
			ent = strings.TrimSpace(ent)
			if ent == "" {
				continue
			}
			if strings.Contains(ent, qe) || strings.Contains(qe, ent) {
				return true
			}
		}
	}
	return false
}
