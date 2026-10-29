package model

import "encoding/json"

// ChatRequest is the RAG chat API input.
type ChatRequest struct {
	Query     string   `json:"query" binding:"required"`
	TenantID  string   `json:"tenant_id,omitempty"`
	KBID      string   `json:"kb_id,omitempty"`
	KBType    string   `json:"kb_type,omitempty"`
	KBTypes   []string `json:"kb_types,omitempty"` // O-07 multi-partition
	TopK      int      `json:"top_k"`
	SessionID string   `json:"session_id,omitempty"`
	// ChatMode: kb_only（仅知识库检索片段）| hybrid（优先知识库，无有效检索时用 LLM 通用能力）
	ChatMode        string `json:"chat_mode,omitempty"`
	SkipCache       bool   `json:"skip_cache"`
	SkipAnswerCache bool   `json:"skip_answer_cache"`
	// LLMOverride 非空时使用调用方密钥（GrayDraft 工作区配置）；否则走 model_gateway
	LLMOverride *LLMOverride `json:"llm_override,omitempty"`
	// WorkspaceContext flattened background for retrieval embedding prefix.
	WorkspaceContext string `json:"workspace_context,omitempty"`
	// ContextProfile structured workspace background for chat system prompt.
	ContextProfile *ContextProfile `json:"context_profile,omitempty"`
	AgentProfile   string          `json:"-"`
	MaxAgentSteps  int             `json:"-"`
	MemoryMode     string          `json:"-"`
}

// ChatStreamEvent is one SSE payload from Core.
type ChatStreamEvent struct {
	Type                   string                  `json:"type"` // retrieval_grade | thinking | meta | chunk | done | error
	Delta                  string                  `json:"delta,omitempty"`
	Done                   bool                    `json:"done,omitempty"`
	RetrievalChunkPreviews []RetrievalChunkPreview `json:"retrieval_chunk_previews,omitempty"`
	Strategy               string                  `json:"strategy,omitempty"`
	CacheHit               string                  `json:"cache_hit,omitempty"`
	AnswerCacheHit         string                  `json:"answer_cache_hit,omitempty"`
	Total                  int                     `json:"total,omitempty"`
	ChunksUsed             int                     `json:"chunks_used,omitempty"`
	QueryIntent            string                  `json:"query_intent,omitempty"`
	FulltextWeight         float64                 `json:"fulltext_weight,omitempty"`
	VectorWeight           float64                 `json:"vector_weight,omitempty"`
	Message                string                  `json:"message,omitempty"`
	Confidence             float64                 `json:"confidence,omitempty"`
	AnswerConfidence       float64                 `json:"answer_confidence,omitempty"`
	ChatMode               string                  `json:"chat_mode,omitempty"`
	GateScore              float64                 `json:"gate_score,omitempty"`
	GateThreshold          float64                 `json:"gate_threshold,omitempty"`
	GateReason             string                  `json:"gate_reason,omitempty"`
	Sources                []ChatSource            `json:"sources,omitempty"`
	Citations              []ChatCitation          `json:"citations,omitempty"`
	QuerySemantic4D        *QuerySemantic4D        `json:"query_semantic_4d,omitempty"`
	Semantic4DFilter       bool                    `json:"semantic_4d_filter_applied,omitempty"`
	RetrievalTrace         *RetrievalTrace         `json:"retrieval_trace,omitempty"`
	// Q&A optimization v2 fields
	RetrievalGrade         string          `json:"retrieval_grade,omitempty"`
	CoverageScore          float64         `json:"coverage_score,omitempty"`
	GradeReason            string          `json:"grade_reason,omitempty"`
	MissingTopics          []string        `json:"missing_topics,omitempty"`
	ChunksSummary          []ChunkSummary  `json:"chunks_summary,omitempty"`
	SentenceLabels         []SentenceLabel `json:"sentence_labels,omitempty"`
	Warnings               []string        `json:"warnings,omitempty"`
	AnswerConfidenceDetail json.RawMessage `json:"answer_confidence_detail,omitempty"`
	AgentProfile           string          `json:"agent_profile,omitempty"`
}

// ChatCitation groups chunks under one document with business extensions.
type ChatCitation struct {
	DocID      string     `json:"doc_id"`
	DocTitle   string     `json:"doc_title,omitempty"`
	KBType     string     `json:"kb_type,omitempty"`
	Extensions Extensions `json:"extensions,omitempty"`
	ChunkIDs   []string   `json:"chunk_ids,omitempty"`
}

// RetrievalChunkPreview is a truncated view of a retrieved chunk for agent logging.
type RetrievalChunkPreview struct {
	Rank     int     `json:"rank"`
	DocTitle string  `json:"doc_title,omitempty"`
	ChunkID  string  `json:"chunk_id,omitempty"`
	Score    float64 `json:"score,omitempty"`
	Snippet  string  `json:"snippet,omitempty"`
}

// SentenceLabel describes one sentence's source attribution (kb vs supplement).
type SentenceLabel struct {
	Sentence string `json:"sentence"`
	Source   string `json:"source"` // "kb" / "supplement"
	Warning  string `json:"warning,omitempty"`
}

// ChunkSummary is a compact per-chunk view for grade SSE events.
type ChunkSummary struct {
	ChunkID    string `json:"chunk_id"`
	DocTitle   string `json:"doc_title"`
	GradeLabel string `json:"grade_label"` // "高可信" / "中可信" / "低可信"
}
