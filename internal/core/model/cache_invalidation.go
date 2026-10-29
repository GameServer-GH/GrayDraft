// Package model defines cache invalidation event types (V1-16).
package model

// CacheInvalidationEvent is published by Workers after document ingestion.
// It carries fine-grained keys so cache managers only evict relevant entries.
type CacheInvalidationEvent struct {
	TraceID    string   `json:"trace_id,omitempty"`
	TaskID     string   `json:"task_id,omitempty"`
	DocID      string   `json:"doc_id,omitempty"`
	KBType     string   `json:"kb_type"`
	Readiness  string   `json:"readiness,omitempty"` // indexed | updated | deleted

	// Fine-grained invalidation keys.
	Entities    []string `json:"entities,omitempty"`      // entity names mentioned in doc
	TitleTerms  []string `json:"title_terms,omitempty"`   // significant words from title
	TaxonomyIDs []string `json:"taxonomy_ids,omitempty"`  // taxonomy node IDs affected
	DocIDs      []string `json:"doc_ids,omitempty"`       // explicit doc_id list

	// Full purge flag (set when precision isn't possible, e.g., bulk delete).
	PurgeAll bool `json:"purge_all,omitempty"`
}

// InvalidationTarget identifies what specific cache entries to flush.
type InvalidationTarget struct {
	KBType    string
	Entity    string
	TitleTerm string
}

// HasFineGrained checks if the invalidation event has enough detail for targeted eviction.
func (ev CacheInvalidationEvent) HasFineGrained() bool {
	return len(ev.Entities) > 0 || len(ev.TitleTerms) > 0 || len(ev.TaxonomyIDs) > 0 || len(ev.DocIDs) > 0
}
