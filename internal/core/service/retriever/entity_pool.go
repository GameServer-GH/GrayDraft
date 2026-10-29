package retriever

import (
	"strings"

	"github.com/graydraft/core/model"
)

// augmentEntitiesFromGlobalPool adds pool entities mentioned in the query (cross-partition hints).
func augmentEntitiesFromGlobalPool(query string, entities []string, pool []string) []string {
	q := strings.ToLower(strings.TrimSpace(query))
	if q == "" || len(pool) == 0 {
		return entities
	}
	seen := map[string]bool{}
	for _, e := range entities {
		seen[strings.ToLower(strings.TrimSpace(e))] = true
	}
	out := append([]string(nil), entities...)
	for _, ent := range pool {
		ent = strings.TrimSpace(ent)
		if ent == "" || len([]rune(ent)) < 2 {
			continue
		}
		key := strings.ToLower(ent)
		if seen[key] {
			continue
		}
		if !strings.Contains(q, key) {
			continue
		}
		if model.LooksLikeConversationalFragment(ent) {
			continue
		}
		seen[key] = true
		out = append(out, ent)
		if len(out) >= 8 {
			break
		}
	}
	return out
}
