package nexus

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
)

// GetWorkspaceContext loads GET /api/v1/kb/:kb_type/context.
func (c *Client) GetWorkspaceContext(ctx context.Context, kbType string) (ContextProfile, error) {
	if c == nil {
		return ContextProfile{}, fmt.Errorf("nexus client not configured")
	}
	kbType = strings.TrimSpace(kbType)
	if kbType == "" {
		return ContextProfile{}, fmt.Errorf("kb_type required")
	}
	var out ContextProfile
	url := fmt.Sprintf("%s/api/v1/kb/%s/context", c.base(), kbType)
	if err := c.getJSON(ctx, url, &out); err != nil {
		return ContextProfile{}, err
	}
	return out, nil
}

// EnrichWorkspaceContext calls POST /api/v1/kb/:kb_type/context/enrich and returns the saved profile.
func (c *Client) EnrichWorkspaceContext(ctx context.Context, kbType, description string) (ContextProfile, error) {
	if c == nil {
		return ContextProfile{}, fmt.Errorf("nexus client not configured")
	}
	kbType = strings.TrimSpace(kbType)
	if kbType == "" {
		return ContextProfile{}, fmt.Errorf("kb_type required")
	}
	body, err := json.Marshal(map[string]string{"description": strings.TrimSpace(description)})
	if err != nil {
		return ContextProfile{}, err
	}
	var out struct {
		Profile ContextProfile `json:"profile"`
		Warning string         `json:"warning"`
		Error   string         `json:"error"`
	}
	path := fmt.Sprintf("/api/v1/kb/%s/context/enrich", kbType)
	if err := c.postJSON(ctx, path, body, &out); err != nil {
		return ContextProfile{}, err
	}
	if out.Profile.Description == "" && len(out.Profile.Entities) == 0 {
		return ContextProfile{}, fmt.Errorf("empty enrich response")
	}
	return out.Profile, nil
}

// PutWorkspaceContext stores PUT /api/v1/kb/:kb_type/context.
func (c *Client) PutWorkspaceContext(ctx context.Context, kbType string, profile ContextProfile) error {
	if c == nil {
		return fmt.Errorf("nexus client not configured")
	}
	kbType = strings.TrimSpace(kbType)
	if kbType == "" {
		return fmt.Errorf("kb_type required")
	}
	profile.KBType = kbType
	body, err := json.Marshal(profile)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPut, c.base()+"/api/v1/kb/"+kbType+"/context", strings.NewReader(string(body)))
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
	if res.StatusCode >= 300 {
		return fmt.Errorf("nexus put workspace context %d", res.StatusCode)
	}
	return nil
}
