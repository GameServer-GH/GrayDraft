package nexus

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// Client proxies GrayDraft Core HTTP APIs for GrayDraft BFF.
type Client struct {
	cfg      Config
	http     *http.Client
	httpSlow *http.Client // smart-import / agent SSE 等长请求
	prefix   string
	tokens   *ServiceTokenProvider
}

// NewClient builds a client; returns nil if CoreURL is empty.
func NewClient(cfg Config) *Client {
	if !cfg.Enabled() {
		return nil
	}
	httpTimeout := cfg.HTTPTimeout
	if httpTimeout <= 0 {
		httpTimeout = 90 * time.Second
	}
	slowTimeout := cfg.SmartImportTimeout
	if slowTimeout <= 0 {
		slowTimeout = 5 * time.Minute
	}
	actor := cfg.AgentActorID
	if actor == "" {
		actor = "graydraft-bff"
	}
	return &Client{
		cfg:      cfg,
		http:     &http.Client{Timeout: httpTimeout},
		httpSlow: &http.Client{Timeout: slowTimeout},
		prefix:   cfg.KBTypePrefix,
		tokens:   NewServiceTokenProvider(cfg.JWTSecret, actor, cfg.AgentJWT, 12*time.Hour),
	}
}

func (c *Client) CoreURL() string {
	return c.base()
}

// ModelGatewayURL returns the configured model gateway base URL for multimodal/LLM routing.
func (c *Client) ModelGatewayURL() string {
	if c == nil {
		return ""
	}
	return strings.TrimRight(c.cfg.ModelGatewayURL, "/")
}

func (c *Client) base() string {
	if c == nil {
		return ""
	}
	return strings.TrimRight(c.cfg.CoreURL, "/")
}

// Health probes GET /api/health.
func (c *Client) Health(ctx context.Context) error {
	if c == nil {
		return fmt.Errorf("nexus client not configured")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.base()+"/api/health", nil)
	if err != nil {
		return err
	}
	applyTraceHeaders(req, ctx)
	res, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(res.Body)
		return fmt.Errorf("nexus health %d: %s", res.StatusCode, strings.TrimSpace(string(b)))
	}
	return nil
}

// KbTypeForWorkspace returns the partition name for a workspace key.
func (c *Client) KbTypeForWorkspace(normalizedWorkspaceKey string) string {
	return KbTypeFromWorkspaceKey(normalizedWorkspaceKey, c.prefix)
}

// EnsureKBType registers kb_type in GrayDraft (idempotent, legacy).
func (c *Client) EnsureKBType(ctx context.Context, kbType, description string) error {
	if c == nil {
		return fmt.Errorf("nexus client not configured")
	}
	body, _ := json.Marshal(map[string]string{
		"name":        kbType,
		"description": description,
	})
	err := c.postJSON(ctx, "/api/v1/kb/types/ensure", body, nil)
	if err != nil && strings.Contains(err.Error(), "404") {
		return fmt.Errorf("%w — GrayDraft Core 镜像过旧，缺少 GrayDraft 面板 API；请在 GrayDraft 仓库执行: docker compose -f deploy/docker-compose/docker-compose.dev.yml build core && docker compose -f deploy/docker-compose/docker-compose.dev.yml up -d core", err)
	}
	return err
}

// EnsureKnowledgeScope registers tenant + knowledge base (PaaS API, idempotent).
func (c *Client) EnsureKnowledgeScope(ctx context.Context, scope KnowledgeScope) error {
	if c == nil {
		return fmt.Errorf("nexus client not configured")
	}
	if err := c.postJSON(ctx, "/api/v1/tenants/ensure", mustJSON(map[string]string{
		"tenant_id": scope.TenantID,
	}), nil); err != nil {
		return err
	}
	path := fmt.Sprintf("/api/v1/tenants/%s/kbs/ensure", scope.TenantID)
	body := map[string]string{
		"kb_id":             scope.KBID,
		"storage_partition": scope.StoragePartition,
		"description":       fmt.Sprintf("GrayDraft workspace kb %s", scope.KBID),
	}
	return c.postJSON(ctx, path, mustJSON(body), nil)
}

func mustJSON(v any) []byte {
	b, _ := json.Marshal(v)
	return b
}

func (c *Client) postJSON(ctx context.Context, path string, body []byte, out any) error {
	if c == nil {
		return fmt.Errorf("nexus client not configured")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.base()+path, strings.NewReader(string(body)))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	applyTraceHeaders(req, ctx)
	res, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	raw, _ := io.ReadAll(res.Body)
	if res.StatusCode >= 300 {
		var errBody struct {
			Error string `json:"error"`
		}
		_ = json.Unmarshal(raw, &errBody)
		msg := errBody.Error
		if msg == "" {
			msg = string(raw)
		}
		return fmt.Errorf("nexus %s %d: %s", path, res.StatusCode, msg)
	}
	if out != nil && len(raw) > 0 {
		return json.Unmarshal(raw, out)
	}
	return nil
}

// DocumentListItem matches GrayDraft panel list row.
type DocumentListItem struct {
	ID           string         `json:"id"`
	Title        string         `json:"title"`
	Preview      string         `json:"preview"`
	IndexReady   bool           `json:"index_ready"`
	QualityScore float64        `json:"quality_score"`
	ChunkCount   int            `json:"chunk_count"`
	Extensions   map[string]any `json:"extensions,omitempty"`
	UpdatedAt    string         `json:"updated_at"`
}

// ListDocumentsResponse is GET /api/v1/kb/{kb_type}/documents.
type ListDocumentsResponse struct {
	KBType string             `json:"kb_type"`
	Total  int64              `json:"total"`
	Items  []DocumentListItem `json:"items"`
}

// ListDocuments returns documents for a tenant knowledge base (PaaS path).
func (c *Client) ListDocuments(ctx context.Context, scope KnowledgeScope, limit, offset int) (*ListDocumentsResponse, error) {
	url := fmt.Sprintf("%s/api/v1/tenants/%s/kbs/%s/documents?limit=%d&offset=%d",
		c.base(), scope.TenantID, scope.KBID, limit, offset)
	var out ListDocumentsResponse
	if err := c.getJSON(ctx, url, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// GetDocument loads one document as generic JSON (for BFF passthrough).
func (c *Client) GetDocument(ctx context.Context, scope KnowledgeScope, docID string) (json.RawMessage, error) {
	url := fmt.Sprintf("%s/api/v1/tenants/%s/kbs/%s/documents/%s",
		c.base(), scope.TenantID, scope.KBID, docID)
	var out json.RawMessage
	if err := c.getJSON(ctx, url, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// DeleteDocument removes a document.
func (c *Client) DeleteDocument(ctx context.Context, scope KnowledgeScope, docID string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodDelete,
		fmt.Sprintf("%s/api/v1/tenants/%s/kbs/%s/documents/%s",
			c.base(), scope.TenantID, scope.KBID, docID), nil)
	if err != nil {
		return err
	}
	applyTraceHeaders(req, ctx)
	res, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if res.StatusCode == http.StatusNoContent {
		return nil
	}
	raw, _ := io.ReadAll(res.Body)
	if res.StatusCode == http.StatusNotFound {
		return fmt.Errorf("document not found")
	}
	return fmt.Errorf("nexus delete %d: %s", res.StatusCode, strings.TrimSpace(string(raw)))
}

// UGCNotifyResponse is POST /api/v1/ugc/notify (202).
type UGCNotifyResponse struct {
	TraceID  string `json:"trace_id,omitempty"`
	TaskID   string `json:"task_id"`
	DocID    string `json:"doc_id,omitempty"`
	Accepted bool   `json:"accepted"`
	Channel  string `json:"channel"`
	Message  string `json:"message,omitempty"`
}

// UGCNotifyInput is POST /api/v1/ugc/notify body (scope fields set by client).
type UGCNotifyInput struct {
	Title      string
	Content    string
	SourceURL  string
	DocID      string
	Extensions map[string]any
}

// UGCNotify queues document ingestion.
func (c *Client) UGCNotify(ctx context.Context, scope KnowledgeScope, in UGCNotifyInput) (*UGCNotifyResponse, error) {
	genQA := true
	bodyMap := map[string]any{
		"tenant_id":   scope.TenantID,
		"kb_id":       scope.KBID,
		"title":       in.Title,
		"content":     in.Content,
		"source_url":  in.SourceURL,
		"generate_qa": genQA,
		"doc_id":      strings.TrimSpace(in.DocID),
	}
	if len(in.Extensions) > 0 {
		bodyMap["extensions"] = in.Extensions
	}
	body, _ := json.Marshal(bodyMap)
	var out UGCNotifyResponse
	if err := c.postJSON(ctx, "/api/v1/ugc/notify", body, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// IngestTaskStatus is GET /api/v1/ugc/tasks/:task_id.
type IngestTaskStatus map[string]any

// SmartImport proxies POST /api/v1/ugc/smart-import (kb_type injected by BFF).
type SmartImportSegmentOverride struct {
	Index     int    `json:"index"`
	LineRange [2]int `json:"line_range"`
}

// SmartImportAttachment is a base64 file for multimodal 4.0 processing.
// DataURL is preferred: GrayDraft downloads the file from this URL directly (e.g., OSS).
// DataBase64 is the fallback: caller-encoded base64 content.
type SmartImportAttachment struct {
	Filename   string `json:"filename"`
	MimeType   string `json:"mime_type,omitempty"`
	DataURL    string `json:"data_url,omitempty"`
	DataBase64 string `json:"data_base64,omitempty"`
}

type SmartImportBody struct {
	RawText                 string                       `json:"raw_text"`
	Filename                string                       `json:"filename,omitempty"`
	SourceURL               string                       `json:"source_url,omitempty"`
	MimeType                string                       `json:"mime_type,omitempty"`
	Attachments             []SmartImportAttachment      `json:"attachments,omitempty"`
	WorkspacePromptOverride string                       `json:"workspace_prompt_override,omitempty"`
	WorkspaceContext        string                       `json:"workspace_context,omitempty"`
	ContextProfile          *ContextProfile              `json:"context_profile,omitempty"`
	Split                   bool                         `json:"split,omitempty"`
	Submit                  bool                         `json:"submit,omitempty"`
	UseLLM                  bool                         `json:"use_llm,omitempty"`
	GenerateQA              *bool                        `json:"generate_qa,omitempty"`
	LLMOverride             *LLMOverride                 `json:"llm_override,omitempty"`
	TargetDocID             string                       `json:"target_doc_id,omitempty"`
	Extensions              map[string]any               `json:"extensions,omitempty"`
	SegmentsOverride        []SmartImportSegmentOverride `json:"segments_override,omitempty"`
}

// SmartImport calls Core smart-import for a tenant knowledge base.
func (c *Client) SmartImport(ctx context.Context, scope KnowledgeScope, normalizedWorkspaceKey string, body SmartImportBody) (json.RawMessage, error) {
	partition := WorkspaceStoragePartition(scope, normalizedWorkspaceKey, c.prefix)
	bodyMap := map[string]any{
		"tenant_id":    scope.TenantID,
		"kb_id":        scope.KBID,
		"raw_text":     body.RawText,
		"filename":     body.Filename,
		"split":        body.Split,
		"submit":       body.Submit,
		"use_llm":      body.UseLLM,
		"generate_qa":  true,
		"hint_kb_type": partition,
	}
	if strings.TrimSpace(body.Filename) != "" {
		bodyMap["filename"] = strings.TrimSpace(body.Filename)
	}
	if strings.TrimSpace(body.SourceURL) != "" {
		bodyMap["source_url"] = strings.TrimSpace(body.SourceURL)
	}
	if strings.TrimSpace(body.MimeType) != "" {
		bodyMap["mime_type"] = strings.TrimSpace(body.MimeType)
	}
	if len(body.Extensions) > 0 {
		bodyMap["extensions"] = body.Extensions
	}
	if body.GenerateQA != nil {
		bodyMap["generate_qa"] = *body.GenerateQA
	}
	if body.LLMOverride != nil {
		bodyMap["llm_override"] = body.LLMOverride
	}
	if strings.TrimSpace(body.TargetDocID) != "" {
		bodyMap["target_doc_id"] = strings.TrimSpace(body.TargetDocID)
	}
	if len(body.SegmentsOverride) > 0 {
		bodyMap["segments_override"] = body.SegmentsOverride
	}
	if len(body.Attachments) > 0 {
		bodyMap["attachments"] = body.Attachments
	}
	if strings.TrimSpace(body.WorkspacePromptOverride) != "" {
		bodyMap["workspace_prompt_override"] = strings.TrimSpace(body.WorkspacePromptOverride)
	}
	if strings.TrimSpace(body.WorkspaceContext) != "" {
		bodyMap["workspace_context"] = strings.TrimSpace(body.WorkspaceContext)
	}
	if body.ContextProfile != nil {
		bodyMap["context_profile"] = body.ContextProfile
	}
	raw, err := json.Marshal(bodyMap)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.base()+"/api/v1/ugc/smart-import", strings.NewReader(string(raw)))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	applyTraceHeaders(req, ctx)
	res, err := c.httpSlow.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	b, _ := io.ReadAll(res.Body)
	if res.StatusCode >= 300 {
		var errBody struct {
			Error   string `json:"error"`
			Message string `json:"message"`
		}
		_ = json.Unmarshal(b, &errBody)
		msg := errBody.Message
		if msg == "" {
			msg = errBody.Error
		}
		if msg == "" {
			msg = string(b)
		}
		return json.RawMessage(b), fmt.Errorf("nexus smart-import %d: %s", res.StatusCode, msg)
	}
	return json.RawMessage(b), nil
}

// MultimodalPromptDefaults is GET /api/v1/multimodal/prompt-defaults.
type MultimodalPromptDefaults struct {
	BasePrompt string `json:"base_prompt"`
	Enabled    bool   `json:"enabled"`
}

// GetMultimodalPromptDefaults returns system base prompt for workspace UI.
func (c *Client) GetMultimodalPromptDefaults(ctx context.Context) (*MultimodalPromptDefaults, error) {
	if c == nil {
		return nil, fmt.Errorf("nexus client not configured")
	}
	var out MultimodalPromptDefaults
	if err := c.getJSON(ctx, c.base()+"/api/v1/multimodal/prompt-defaults", &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// GetIngestTask polls async ingest task state.
func (c *Client) GetIngestTask(ctx context.Context, taskID string) (IngestTaskStatus, error) {
	url := fmt.Sprintf("%s/api/v1/ugc/tasks/%s", c.base(), taskID)
	var out IngestTaskStatus
	if err := c.getJSON(ctx, url, &out); err != nil {
		return nil, err
	}
	return out, nil
}

func (c *Client) getJSON(ctx context.Context, url string, out any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	applyTraceHeaders(req, ctx)
	res, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	raw, _ := io.ReadAll(res.Body)
	if res.StatusCode == http.StatusNotFound {
		return fmt.Errorf("not found")
	}
	if res.StatusCode >= 300 {
		var errBody struct {
			Error string `json:"error"`
		}
		_ = json.Unmarshal(raw, &errBody)
		msg := errBody.Error
		if msg == "" {
			msg = string(raw)
		}
		return fmt.Errorf("nexus GET %d: %s", res.StatusCode, msg)
	}
	if out != nil && len(raw) > 0 {
		return json.Unmarshal(raw, out)
	}
	return nil
}
