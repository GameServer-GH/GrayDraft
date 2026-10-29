package taxonomyevolve

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/graydraft/core/config"
)

// WorkerClient calls AI Worker maintenance endpoints from Core.
type WorkerClient struct {
	baseURL string
	http    *http.Client
}

// NewWorkerClient creates a worker HTTP client when ai_worker.url is set.
func NewWorkerClient(cfg *config.Config) *WorkerClient {
	if cfg == nil || cfg.AIWorker.URL == "" {
		return nil
	}
	timeout := time.Duration(cfg.AIWorker.TimeoutSeconds) * time.Second
	if timeout < 60*time.Second {
		timeout = 120 * time.Second
	}
	return &WorkerClient{
		baseURL: cfg.AIWorker.URL,
		http:    &http.Client{Timeout: timeout},
	}
}

type shadowBackfillRequest struct {
	KBType          string `json:"kb_type"`
	TaxonomyVersion int    `json:"taxonomy_version"`
	Limit           int    `json:"limit"`
}

type shadowBackfillResponse struct {
	Updated int    `json:"updated"`
	Skipped int    `json:"skipped"`
	Message string `json:"message,omitempty"`
}

// ShadowBackfill posts to Worker /v1/taxonomy/shadow-backfill.
func (c *WorkerClient) ShadowBackfill(ctx context.Context, kbType string, taxonomyVersion, limit int) (*shadowBackfillResponse, error) {
	if c == nil {
		return nil, fmt.Errorf("worker client not configured")
	}
	body, err := json.Marshal(shadowBackfillRequest{
		KBType:          kbType,
		TaxonomyVersion: taxonomyVersion,
		Limit:           limit,
	})
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/v1/taxonomy/shadow-backfill", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("worker shadow-backfill status %d: %s", resp.StatusCode, string(b))
	}
	var parsed shadowBackfillResponse
	if err := json.Unmarshal(b, &parsed); err != nil {
		return nil, err
	}
	return &parsed, nil
}
