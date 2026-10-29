package multimodal

// FileInput is one uploaded file for multimodal extraction.
type FileInput struct {
	Filename string
	MimeType string
	Data     []byte
}

// ExtractionResult is normalized text for smart-import (4.0).
type ExtractionResult struct {
	Text         string
	StructuredKV string
	Route        string // ocr | vlm | text | cache | phash_cache
	Filename     string
	MimeType     string
	MD5          string
	PHash        string
	MediaURL     string // graydraft media ref for RAG visual response
	Cached       bool
	Timestamps    []string
	ChainSummary  string // rolling summary for multi-image chain VLM (internal)
}

// AssetMeta is stored on document.extensions.multimodal.assets[].
type AssetMeta struct {
	Filename     string   `json:"filename"`
	MimeType     string   `json:"mime_type,omitempty"`
	MD5          string   `json:"md5"`
	PHash        string   `json:"phash,omitempty"`
	MediaURL     string   `json:"media_url"`
	Route        string   `json:"route"`
	Timestamps   []string `json:"timestamps,omitempty"`
	StructuredKV string   `json:"structured_kv,omitempty"`
}

// DocumentExtensions is the multimodal block on kb_documents.extensions.
type DocumentExtensions struct {
	WorkspaceID       string      `json:"workspace_id,omitempty"`
	StructuredKVText  string      `json:"structured_kv_text,omitempty"`
	Assets            []AssetMeta `json:"assets,omitempty"`
}
