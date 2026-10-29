package model

// SmartImportRequest is agent-assisted bulk import input.
type SmartImportRequest struct {
	TenantID string `json:"tenant_id,omitempty"`
	KBID     string `json:"kb_id,omitempty"`
	RawText  string `json:"raw_text"`
	Filename string `json:"filename,omitempty"`
	MimeType string `json:"mime_type,omitempty"`
	// Attachments are binary files (base64) processed by multimodal 4.0 before analyze.
	Attachments []SmartImportAttachment `json:"attachments,omitempty"`
	// WorkspacePromptOverride is stacked on system base prompt for VLM/OCR (4.0).
	WorkspacePromptOverride string `json:"workspace_prompt_override,omitempty"`
	// WorkspaceContext is flattened background for retrieval-style hints (optional).
	WorkspaceContext string `json:"workspace_context,omitempty"`
	// ContextProfile is structured workspace background (import / storage).
	ContextProfile *ContextProfile `json:"context_profile,omitempty"`
	SourceURL      string          `json:"source_url,omitempty"`
	HintKBType     string          `json:"hint_kb_type,omitempty"` // legacy; defaults to resolved partition when empty
	GenerateQA     *bool           `json:"generate_qa,omitempty"`
	UseLLM         bool            `json:"use_llm,omitempty"`
	Split          bool            `json:"split,omitempty"` // 拆分为多条知识
	Submit         bool            `json:"submit,omitempty"`
	MinConfidence  *float64        `json:"min_confidence,omitempty"` // 覆盖配置阈值
	Extensions     Extensions      `json:"extensions,omitempty"`     // 写入文档的业务扩展字段
	LLMOverride    *LLMOverride    `json:"llm_override,omitempty"`   // 工作区指定模型；空则走 gateway
	// TargetDocID 非空时强制更新该文档（GrayDraft 选中知识卡片）；空则强制新建
	TargetDocID string `json:"target_doc_id,omitempty"`
	// SegmentsOverride 用户调整切割线后回传（O-06）
	SegmentsOverride []SmartImportSegmentOverride `json:"segments_override,omitempty"`
}

// SmartImportAttachment is one uploaded file for multimodal extraction.
// DataURL (preferred): GrayDraft downloads the file from this URL directly (e.g., OSS).
// DataBase64 (fallback): caller-encoded base64 content.
// Priority: DataURL > DataBase64.
type SmartImportAttachment struct {
	Filename   string `json:"filename"`
	MimeType   string `json:"mime_type,omitempty"`
	DataURL    string `json:"data_url,omitempty"`
	DataBase64 string `json:"data_base64,omitempty"`
}

// SmartImportSegmentOverride adjusts one segment line range before analyze/submit.
type SmartImportSegmentOverride struct {
	Index     int    `json:"index"`
	LineRange [2]int `json:"line_range"` // 1-based inclusive
}

// SimilarDocument is a semantic neighbor suggested during import (O-05).
type SimilarDocument struct {
	DocID      string  `json:"doc_id"`
	Title      string  `json:"title"`
	KBType     string  `json:"kb_type"`
	Similarity float64 `json:"similarity"`
}

// SmartImportPreview is the analyzed import plan for one segment.
type SmartImportPreview struct {
	KBType          string  `json:"kb_type"`
	Title           string  `json:"title"`
	Content         string  `json:"content"`
	ContentHash     string  `json:"content_hash"`
	MimeType        string  `json:"mime_type"`
	Action          string  `json:"action"` // create | update | unchanged
	ExistingDocID   string  `json:"existing_doc_id,omitempty"`
	ExistingVersion int64   `json:"existing_kb_version,omitempty"`
	Confidence      float64 `json:"confidence"`
	ImportAllowed   bool    `json:"import_allowed"`
	Reason          string  `json:"reason"`
	Method          string  `json:"method"` // rules | llm | rules+llm
}

// SmartImportDocAnalysis is macro-level document understanding (2.0).
type SmartImportDocAnalysis struct {
	Subject string `json:"subject,omitempty"`
	Version     string `json:"version,omitempty"`
	DocType     string `json:"doc_type,omitempty"`
	ParentTitle string `json:"parent_title,omitempty"`
}

// SmartImportItem is one split segment analysis result.
type SmartImportItem struct {
	Index              int                `json:"index"`
	Preview            SmartImportPreview `json:"preview"`
	ImportAllowed      bool               `json:"import_allowed"`
	LineRange          [2]int             `json:"line_range,omitempty"` // 1-based inclusive
	ContextEnhancement string             `json:"context_enhancement,omitempty"`
}

// SmartImportSkipped explains why a segment was not submitted.
type SmartImportSkipped struct {
	Index      int     `json:"index"`
	Reason     string  `json:"reason"`
	Confidence float64 `json:"confidence"`
}

// SmartImportResponse always includes confidence fields (even on failure).
type SmartImportResponse struct {
	Confidence            float64 `json:"confidence"`
	MinConfidenceRequired float64 `json:"min_confidence_required"`
	ImportAllowed         bool    `json:"import_allowed"`
	Split                 bool    `json:"split"`
	Message               string  `json:"message,omitempty"`

	DocAnalysis       *SmartImportDocAnalysis `json:"doc_analysis,omitempty"`
	SplittingStrategy string                  `json:"splitting_strategy,omitempty"` // single | multi_segment
	SplitMethod       string                  `json:"split_method,omitempty"`       // rules | llm | rules+llm

	Preview *SmartImportPreview  `json:"preview,omitempty"`
	Items   []SmartImportItem    `json:"items,omitempty"`
	Tasks   []UGCNotifyResponse  `json:"tasks,omitempty"`
	Skipped []SmartImportSkipped `json:"skipped,omitempty"`

	Task      *UGCNotifyResponse    `json:"task,omitempty"` // single-mode submit
	Readiness *SmartImportReadiness `json:"readiness,omitempty"`

	SimilarDocuments []SimilarDocument `json:"similar_documents,omitempty"` // O-05

	MultimodalExtractions []SmartImportMultimodalExtraction `json:"multimodal_extractions,omitempty"`
}

// SmartImportReadiness tells Agent whether submitted content is already queryable.
type SmartImportReadiness struct {
	Queryable bool     `json:"queryable"`
	Status    string   `json:"status"` // preview | queued | ready | no_task
	Message   string   `json:"message,omitempty"`
	TaskIDs   []string `json:"task_ids,omitempty"`
	DocIDs    []string `json:"doc_ids,omitempty"`
}

// SmartImportMultimodalExtraction summarizes per-file 4.0 processing.
type SmartImportMultimodalExtraction struct {
	Filename string `json:"filename"`
	Route    string `json:"route"`
	Cached   bool   `json:"cached"`
	MD5      string `json:"md5,omitempty"`
}
