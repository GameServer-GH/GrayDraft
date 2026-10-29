// Package model defines large file manifest types for async parsing (V1-10).
package model

// LargeFileManifest describes a file stored in OSS that the Worker should download and parse.
type LargeFileManifest struct {
	TraceID     string `json:"trace_id,omitempty"`
	TaskID      string `json:"task_id"`
	DocID       string `json:"doc_id,omitempty"`
	KBType      string `json:"kb_type"`
	Title       string `json:"title"`
	SourceURL   string `json:"source_url,omitempty"`
	MimeType    string `json:"mime_type"`
	GenerateQA  bool   `json:"generate_qa"`
	Tags        []string       `json:"tags,omitempty"`
	Extensions  map[string]any `json:"extensions,omitempty"`

	// OSS reference.
	StorageProvider string `json:"storage_provider"` // s3, gcs, azure, oss
	Bucket          string `json:"bucket"`
	ObjectKey       string `json:"object_key"`
	PresignedURL    string `json:"presigned_url"`
	PresignedExpiry int64  `json:"presigned_expiry,omitempty"` // unix timestamp

	// File metadata.
	FileSize int64  `json:"file_size,omitempty"`
	FileHash string `json:"file_hash,omitempty"` // sha256
}

// LargeFileStatus tracks async parse progress.
type LargeFileStatus struct {
	TaskID         string `json:"task_id"`
	Status         string `json:"status"` // downloading | parsing | chunking | done | failed
	ProgressBytes  int64  `json:"progress_bytes,omitempty"`
	TotalBytes     int64  `json:"total_bytes,omitempty"`
	ErrorMessage   string `json:"error_message,omitempty"`
}
