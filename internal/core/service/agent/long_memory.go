package agent

import (
	"context"
	"fmt"
	"sort"
	"strings"
)

type MemoryItem struct {
	MemoryID    string         `json:"memory_id"`
	TenantID    string         `json:"tenant_id"`
	ActorID     string         `json:"actor_id"`
	Kind        string         `json:"kind"`
	Subject     string         `json:"subject"`
	Value       map[string]any `json:"value"`
	Confidence  float32        `json:"confidence"`
	Sensitivity string         `json:"sensitivity"`
	Status      string         `json:"status"`
}

type LongTermMemory interface {
	Write(context.Context, MemoryItem, bool) error
	Search(context.Context, string, string, string, int) ([]MemoryItem, error)
	Delete(context.Context, string, string) error
}
type LongMemoryAdmin interface {
	List(context.Context, string, string, string, int) ([]MemoryItem, error)
	ResolveConflict(context.Context, string, string, string) error
	Revoke(context.Context, string, string, string) error
	CleanupExpired(context.Context, string) error
}

type Embedder interface {
	EmbedQuery(context.Context, string) ([]float32, error)
}
type VectorLongTermMemory interface {
	SearchSimilar(context.Context, string, string, []float32, int) ([]MemoryItem, error)
	SetEmbedding(context.Context, string, []float32) error
}

// FuseMemoryResults combines relational and semantic recall with reciprocal
// rank fusion, then de-duplicates by kind+subject. Relational confidence is a
// tie-breaker so explicit facts remain stable when scores are equal.
func FuseMemoryResults(relational, semantic []MemoryItem, limit int) []MemoryItem {
	if limit <= 0 {
		limit = 8
	}
	type scored struct {
		item  MemoryItem
		score float64
	}
	merged := map[string]scored{}
	add := func(items []MemoryItem, weight float64) {
		for rank, item := range items {
			key := item.Kind + "\x00" + strings.ToLower(item.Subject)
			s := 1.0 / float64(60+rank+1) * weight
			if current, ok := merged[key]; ok {
				current.score += s
				if item.Confidence > current.item.Confidence {
					current.item = item
				}
				merged[key] = current
			} else {
				merged[key] = scored{item: item, score: s}
			}
		}
	}
	add(relational, 1.0)
	add(semantic, 1.0)
	out := make([]scored, 0, len(merged))
	for _, v := range merged {
		out = append(out, v)
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].score == out[j].score {
			return out[i].item.Confidence > out[j].item.Confidence
		}
		return out[i].score > out[j].score
	})
	if len(out) > limit {
		out = out[:limit]
	}
	result := make([]MemoryItem, len(out))
	for i := range out {
		result[i] = out[i].item
	}
	return result
}

// ValidateMemoryWrite checks an automatic workspace memory write.
// Per-item consent is not required. Sensitive values and low-confidence writes are refused.
// The consent argument remains so existing callers compile; it does not gate the write.
func ValidateMemoryWrite(item MemoryItem, consent bool) error {
	_ = consent
	if item.TenantID == "" || item.ActorID == "" || item.Kind == "" || strings.TrimSpace(item.Subject) == "" {
		return fmt.Errorf("memory: tenant, actor, kind and subject are required")
	}
	if item.Sensitivity == "sensitive" {
		return fmt.Errorf("memory: sensitive memory cannot be written automatically")
	}
	if item.Confidence < 0.75 {
		return fmt.Errorf("memory: confidence below write threshold")
	}
	return nil
}
