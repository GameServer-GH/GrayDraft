package ugc

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/graydraft/core/config"
	"github.com/graydraft/core/model"
	"github.com/graydraft/core/observability/trace"
)

// WorkerClient sync-calls Worker ingest when NATS is unavailable.
type WorkerClient struct {
	baseURL string
	http    *http.Client
}

// NewWorkerClient creates a Worker HTTP client.
func NewWorkerClient(cfg *config.Config) *WorkerClient {
	if cfg.AIWorker.URL == "" {
		return nil
	}
	return &WorkerClient{
		baseURL: cfg.AIWorker.URL,
		http: &http.Client{
			Timeout: time.Duration(cfg.AIWorker.TimeoutSeconds) * time.Second,
		},
	}
}

type ingestResponse struct {
	DocID      string `json:"doc_id"`
	KBType     string `json:"kb_type"`
	ChunkCount int    `json:"chunk_count"`
	IndexReady bool   `json:"index_ready"`
}

// IngestUGC posts to Worker /v1/ingest/ugc synchronously.
func (c *WorkerClient) IngestUGC(ctx context.Context, msg model.UGCQueueMessage) (*ingestResponse, error) {
	if c == nil {
		return nil, fmt.Errorf("worker client not configured")
	}
	body, err := json.Marshal(msg)
	if err != nil {
		return nil, err
	}
	url := c.baseURL + "/v1/ingest/ugc"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	if msg.TraceID != "" {
		req.Header.Set(trace.HeaderTraceID, msg.TraceID)
		req.Header.Set(trace.HeaderRequestID, msg.TraceID)
	}
	if msg.TaskID != "" {
		req.Header.Set(trace.HeaderTaskID, msg.TaskID)
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("worker ingest: %w", err)
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("worker ingest status %d: %s", resp.StatusCode, string(b))
	}

	var parsed ingestResponse
	if err := json.Unmarshal(b, &parsed); err != nil {
		return nil, err
	}
	return &parsed, nil
}
