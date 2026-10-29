package model

// Chunk is a retrieved knowledge slice.
type Chunk struct {
	ID           string    `json:"id"`
	DocID        string    `json:"doc_id"`
	KBType       string    `json:"kb_type"`
	Content      string    `json:"content"`
	Score        float64   `json:"score"`
	ChunkIndex   int       `json:"chunk_index"`
	QualityScore float64   `json:"quality_score,omitempty"`
	DocTitle     string     `json:"doc_title,omitempty"`
	DocExtensions Extensions `json:"doc_extensions,omitempty"` // 来自 kb_documents.extensions
	Extensions   Extensions `json:"extensions,omitempty"` // 来自 kb_documents.extensions
	VectorScore   float64 `json:"vector_score,omitempty"`
	RerankScore   float64 `json:"rerank_score,omitempty"`
	EntityMatched bool    `json:"entity_matched,omitempty"` // deprecated: use title_matched
	TitleMatched  bool    `json:"title_matched,omitempty"`
	Source        string  `json:"source"` // fulltext | vector | hybrid | structured
	MediaURL      string  `json:"media_url,omitempty"`      // 4.0 visual RAG
	MediaTimestamps []string `json:"media_timestamps,omitempty"`
}

// SearchRequest is the retrieval API input.
type SearchRequest struct {
	Query           string    `json:"query" binding:"required"`
	TenantID        string    `json:"tenant_id,omitempty"`
	KBID            string    `json:"kb_id,omitempty"`
	KBType          string    `json:"kb_type,omitempty"`
	KBTypes         []string  `json:"kb_types,omitempty"` // O-07: parallel multi-partition search
	TopK            int       `json:"top_k"`
	QueryEmbedding  []float32 `json:"query_embedding,omitempty"`
	UseGlobalIndex  bool      `json:"use_global_index"`
	FulltextWeight  *float64  `json:"fulltext_weight,omitempty"`
	VectorWeight    *float64  `json:"vector_weight,omitempty"`
	SkipCache       bool      `json:"skip_cache"`
	SkipClassifier  bool      `json:"skip_classifier"`
	IncludeCold     bool      `json:"include_cold"` // O-18: include cold-tier documents
	ExpandedTerms   []string  `json:"expanded_terms,omitempty"` // 3.5 alias alignment
	WorkspaceContext string                `json:"workspace_context,omitempty"` // 4.0 RAG query domain hint (flattened)
	ContextProfile   *ContextProfile         `json:"context_profile,omitempty"`   // structured workspace background
	Semantic4D       *QuerySemantic4D        `json:"semantic_4d,omitempty"`         // optional: server-side query 4D (normally auto)
}

// SearchResponse is the retrieval API output.
type SearchResponse struct {
	Query      string   `json:"query"`
	KBType     string   `json:"kb_type,omitempty"`
	KBTypes    []string `json:"kb_types,omitempty"`
	TopK       int     `json:"top_k"`
	Total      int     `json:"total"`
	Chunks     []Chunk `json:"chunks"`
	Strategy   string  `json:"strategy"`
	CacheHit       string  `json:"cache_hit,omitempty"` // l1 | l2_exact | l2_semantic | l3
	QueryIntent    string  `json:"query_intent,omitempty"`
	FulltextWeight float64 `json:"fulltext_weight,omitempty"`
	VectorWeight   float64 `json:"vector_weight,omitempty"`
	CatalogVer     int64   `json:"catalog_version,omitempty"`
	QuerySemantic4D *QuerySemantic4D `json:"query_semantic_4d,omitempty"`
	Semantic4DFilter bool            `json:"semantic_4d_filter_applied,omitempty"`
	RetrievalTrace   *RetrievalTrace `json:"retrieval_trace,omitempty"`
	CandidateStats   *CandidateStats  `json:"candidate_stats,omitempty"`
}
