package model

// CandidateStats tracks per-stage candidate counts for retrieval diagnostics (V1-21).
type CandidateStats struct {
	FulltextRaw    int `json:"fulltext_raw"`
	VectorRaw      int `json:"vector_raw"`
	StructuredRaw  int `json:"structured_raw"`
	MergedUnique   int `json:"merged_unique"`
	RRFKept        int `json:"rrf_kept"`
	RerankInput    int `json:"rerank_input"`
	RerankOutput   int `json:"rerank_output"`
	WaterfallTier  int `json:"waterfall_tier,omitempty"`
	RerankCalls    int `json:"rerank_calls,omitempty"`
	RerankSkipped  int `json:"rerank_skipped,omitempty"`
	RerankFailed   int `json:"rerank_failed,omitempty"`
}
