package nexus

import (
	"context"
	"encoding/json"
	"fmt"
)

// RetrievalSearchRequest is POST /api/v1/retrieval/search body (scope set by BFF).
type RetrievalSearchRequest struct {
	Query            string          `json:"query"`
	TenantID         string          `json:"tenant_id,omitempty"`
	KBID             string          `json:"kb_id,omitempty"`
	KBType           string          `json:"kb_type,omitempty"`
	TopK             int             `json:"top_k,omitempty"`
	SkipCache        bool            `json:"skip_cache,omitempty"`
	WorkspaceContext string          `json:"workspace_context,omitempty"`
	ContextProfile   *ContextProfile `json:"context_profile,omitempty"`
}

// RetrievalSearch proxies GrayDraft hybrid retrieval.
func (c *Client) RetrievalSearch(ctx context.Context, req RetrievalSearchRequest) (json.RawMessage, error) {
	if c == nil {
		return nil, fmt.Errorf("nexus client not configured")
	}
	body, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}
	var out json.RawMessage
	if err := c.postJSON(ctx, "/api/v1/retrieval/search", body, &out); err != nil {
		return nil, err
	}
	return out, nil
}
