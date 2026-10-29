package model

// RetrievalTrace exposes semantic retrieval path for debugging and acceptance (e.g. 莫特斐).
type RetrievalTrace struct {
	WaterfallTier   int      `json:"waterfall_tier,omitempty"`   // 1–5 when 4D waterfall used; 0 = not applied
	WaterfallSuffix string   `json:"waterfall_suffix,omitempty"` // "", _wf_core, _wf_ent, _wf_dom, _wf_soft
	Query4DMethod   string   `json:"query_4d_method,omitempty"`  // rules | llm
	SceneOriginal   string   `json:"scene_original,omitempty"`
	SceneAligned    string   `json:"scene_aligned,omitempty"`
	StyleOriginal   string   `json:"style_original,omitempty"`
	StyleAligned    string   `json:"style_aligned,omitempty"`
	EntityAlign     string   `json:"entity_align,omitempty"` // alias | embedding | rules | none
	EntitiesFinal   []string `json:"entities_final,omitempty"`
	// Q&A optimization v2
	RetrievalGrade string `json:"retrieval_grade,omitempty"` // A/B/C/D
}

// RetrievalDiagnosis is a compact debug view for Agent import/chat no-result cases.
type RetrievalDiagnosis struct {
	Query            string                `json:"query"`
	KBType           string                `json:"kb_type,omitempty"`
	Strategy         string                `json:"strategy,omitempty"`
	CacheHit         string                `json:"cache_hit,omitempty"`
	Total            int                   `json:"total"`
	QueryIntent      string                `json:"query_intent,omitempty"`
	QuerySemantic4D  *QuerySemantic4D      `json:"query_semantic_4d,omitempty"`
	Semantic4DFilter bool                  `json:"semantic_4d_filter_applied,omitempty"`
	RetrievalTrace   *RetrievalTrace       `json:"retrieval_trace,omitempty"`
	CandidateStats   *CandidateStats  `json:"candidate_stats,omitempty"`
	Gate             RetrievalGateDebug    `json:"gate"`
	TopChunks        []RetrievalChunkDebug `json:"top_chunks"`
}

type RetrievalGateDebug struct {
	Useful    bool    `json:"useful"`
	Score     float64 `json:"score"`
	Threshold float64 `json:"threshold"`
	Reason    string  `json:"reason"`
}

type RetrievalChunkDebug struct {
	Rank           int      `json:"rank"`
	ChunkID        string   `json:"chunk_id"`
	DocID          string   `json:"doc_id"`
	DocTitle       string   `json:"doc_title,omitempty"`
	Score          float64  `json:"score,omitempty"`
	RerankScore    float64  `json:"rerank_score,omitempty"`
	VectorScore    float64  `json:"vector_score,omitempty"`
	QualityScore   float64  `json:"quality_score,omitempty"`
	TitleMatched   bool     `json:"title_matched,omitempty"`
	EntityMatched  bool     `json:"entity_matched,omitempty"`
	Source         string   `json:"source,omitempty"`
	LineRange      any      `json:"line_range,omitempty"`
	SemanticTags4D any      `json:"semantic_tags_4d,omitempty"`
	SearchAnchors  []string `json:"search_anchors,omitempty"`
	Snippet        string   `json:"snippet,omitempty"`
}

// WaterfallTierFromSuffix maps waterfall SQL suffix to tier number (1 = strictest).
func WaterfallTierFromSuffix(suffix string) int {
	switch suffix {
	case "":
		return 1
	case "_wf_core":
		return 2
	case "_wf_ent":
		return 3
	case "_wf_dom":
		return 4
	case "_wf_soft":
		return 5
	default:
		return 0
	}
}
