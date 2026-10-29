package nexus

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

func (c *Client) ListMemories(ctx context.Context, tenantID string) (json.RawMessage, error) {
	_, raw, err := c.agentRaw(ctx, http.MethodGet, "/api/v2/memories?status=active", tenantID, nil)
	if err != nil {
		return nil, err
	}
	return raw, nil
}

func (c *Client) RevokeMemory(ctx context.Context, tenantID, memoryID string) error {
	path := "/api/v2/memories/" + url.PathEscape(strings.TrimSpace(memoryID))
	_, _, err := c.agentRaw(ctx, http.MethodDelete, path, tenantID, nil)
	return err
}

func (c *Client) GetProductThread(ctx context.Context, tenantID, threadKey string) (json.RawMessage, error) {
	path := "/api/v2/threads?thread_key=" + url.QueryEscape(threadKey)
	_, raw, err := c.agentRaw(ctx, http.MethodGet, path, tenantID, nil)
	if err != nil {
		return nil, err
	}
	return raw, nil
}

func (c *Client) PutProductThread(ctx context.Context, tenantID, threadKey string, payload json.RawMessage) error {
	body, err := json.Marshal(map[string]any{"thread_key": threadKey, "payload": payload})
	if err != nil {
		return err
	}
	_, _, err = c.agentRaw(ctx, http.MethodPut, "/api/v2/threads", tenantID, body)
	return err
}

func (c *Client) DeleteProductThreads(ctx context.Context, tenantID, threadKey string) error {
	path := "/api/v2/threads"
	if strings.TrimSpace(threadKey) != "" {
		path += "?thread_key=" + url.QueryEscape(threadKey)
	}
	_, _, err := c.agentRaw(ctx, http.MethodDelete, path, tenantID, nil)
	return err
}

func (c *Client) GetMediaMeta(ctx context.Context, md5 string) (json.RawMessage, error) {
	if c == nil {
		return nil, fmt.Errorf("nexus client not configured")
	}
	var out json.RawMessage
	if err := c.getJSON(ctx, c.base()+"/api/v1/multimodal/media/"+url.PathEscape(md5), &out); err != nil {
		return nil, err
	}
	return out, nil
}

func (c *Client) agentRaw(ctx context.Context, method, path, tenantID string, body []byte) (int, []byte, error) {
	if c == nil {
		return 0, nil, fmt.Errorf("nexus client not configured")
	}
	var rdr io.Reader
	if body != nil {
		rdr = bytes.NewReader(body)
	}
	req, err := http.NewRequestWithContext(ctx, method, c.base()+path, rdr)
	if err != nil {
		return 0, nil, err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if err := c.applyAgentHeaders(req, ChatStreamRequest{TenantID: tenantID}); err != nil {
		return 0, nil, err
	}
	res, err := c.http.Do(req)
	if err != nil {
		return 0, nil, err
	}
	defer res.Body.Close()
	raw, _ := io.ReadAll(res.Body)
	if res.StatusCode >= 300 {
		msg := strings.TrimSpace(string(raw))
		if msg == "" {
			msg = res.Status
		}
		return res.StatusCode, raw, fmt.Errorf("nexus: %s", msg)
	}
	return res.StatusCode, raw, nil
}
