package model

// ChatSource is a KB citation attached to a chat answer (for feedback).
type ChatSource struct {
	ChunkID      string     `json:"chunk_id"`
	DocID        string     `json:"doc_id"`
	KBType       string     `json:"kb_type"`
	DocTitle     string     `json:"doc_title,omitempty"`
	QualityScore float64    `json:"quality_score,omitempty"`
	DocExtensions Extensions `json:"doc_extensions,omitempty"`
	Extensions   Extensions `json:"extensions,omitempty"`
}

// ChatFeedbackRequest records up/down vote on an agent answer.
type ChatFeedbackRequest struct {
	Vote       string   `json:"vote" binding:"required"` // up | down
	VoteType   string   `json:"vote_type,omitempty"`     // "quality" (default) | "coverage"
	Query      string   `json:"query,omitempty"`
	KBType     string   `json:"kb_type,omitempty"`
	ChunkIDs   []string `json:"chunk_ids,omitempty"`
	DocIDs     []string `json:"doc_ids,omitempty"`
	Confidence float64  `json:"confidence,omitempty"` // client-side answer confidence
}

// ChatFeedbackDocResult is per-document outcome after a vote.
type ChatFeedbackDocResult struct {
	DocID         string  `json:"doc_id"`
	OldQuality    float64 `json:"old_quality"`
	NewQuality    float64 `json:"new_quality"`
	Deleted       bool    `json:"deleted"`
	ChunksTouched int     `json:"chunks_touched"`
}

// ChatFeedbackResponse is returned after applying vote penalties or boosts.
type ChatFeedbackResponse struct {
	Vote          string                  `json:"vote"` // up | down
	Accepted      bool                    `json:"accepted"`
	Message       string                  `json:"message,omitempty"`
	ChunksUpdated int                     `json:"chunks_updated"`
	Docs          []ChatFeedbackDocResult `json:"docs"`
	DeletedDocIDs []string                `json:"deleted_doc_ids,omitempty"`
	MinQuality    float64                 `json:"min_quality_required"` // delete threshold
}
