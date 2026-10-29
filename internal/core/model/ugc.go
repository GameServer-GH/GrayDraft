package model

// UGCNotifyRequest is the ingest callback payload.
// Scope: provide (tenant_id + kb_id) for PaaS API, or legacy kb_type alone.
type UGCNotifyRequest struct {
	TenantID    string   `json:"tenant_id,omitempty"`
	KBID        string   `json:"kb_id,omitempty"`
	KBType      string   `json:"kb_type,omitempty"`
	Title       string   `json:"title" binding:"required"`
	Content     string   `json:"content" binding:"required"`
	SourceURL   string   `json:"source_url,omitempty"`
	DocID       string   `json:"doc_id,omitempty"`
	MimeType    string   `json:"mime_type,omitempty"`
	GenerateQA  *bool    `json:"generate_qa,omitempty"`
	Tags        []string     `json:"tags,omitempty"`
	Extensions  Extensions   `json:"extensions,omitempty"`
}

// UGCNotifyResponse is returned after the message is queued.
type UGCNotifyResponse struct {
	TraceID  string `json:"trace_id,omitempty"`
	TaskID   string `json:"task_id"`
	DocID    string `json:"doc_id,omitempty"`
	Accepted bool   `json:"accepted"`
	Channel  string `json:"channel"` // nats | sync_fallback
	Message  string `json:"message,omitempty"`
}

// UGCQueueMessage is published to NATS (Worker UGCMessage compatible).
type UGCQueueMessage struct {
	TraceID    string   `json:"trace_id,omitempty"`
	TaskID     string   `json:"task_id"`
	DocID      string   `json:"doc_id,omitempty"`
	KBType     string   `json:"kb_type"`
	Title      string   `json:"title"`
	Content    string   `json:"content"`
	SourceURL  string   `json:"source_url,omitempty"`
	MimeType   string   `json:"mime_type"`
	GenerateQA bool     `json:"generate_qa"`
	Tags       []string   `json:"tags,omitempty"`
	Extensions Extensions `json:"extensions,omitempty"`
}
