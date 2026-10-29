package answercache

// Entry is a cached RAG answer for replay over SSE.
type Entry struct {
	Answer         string  `json:"answer"`
	Strategy       string  `json:"strategy,omitempty"`
	ChunksUsed     int     `json:"chunks_used,omitempty"`
	QueryIntent    string  `json:"query_intent,omitempty"`
	FulltextWeight float64 `json:"fulltext_weight,omitempty"`
	VectorWeight   float64 `json:"vector_weight,omitempty"`
	RetrievalHit   string  `json:"retrieval_hit,omitempty"`
}
