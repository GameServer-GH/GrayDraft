package reranker

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/graydraft/core/config"
	"github.com/graydraft/core/observability"
	"github.com/graydraft/core/model"
	"github.com/graydraft/core/service/cost"
)

// Client calls Worker L1/L2 rerank APIs.
type Client struct {
	baseURL string
	http    *http.Client
	l1TopN  int
	l2TopN  int
}

// NewClient creates a Worker rerank client. Returns nil if URL empty.
func NewClient(cfg *config.Config) *Client {
	if cfg.AIWorker.URL == "" {
		return nil
	}
	return &Client{
		baseURL: cfg.AIWorker.URL,
		http: &http.Client{
			Timeout: time.Duration(cfg.AIWorker.TimeoutSeconds) * time.Second,
		},
		l1TopN: cfg.Retrieval.RerankL1TopN,
		l2TopN: cfg.Retrieval.RerankL2TopN,
	}
}

type candidate struct {
	ID   string `json:"id"`
	Text string `json:"text"`
}

type rerankRequest struct {
	Query      string      `json:"query"`
	Candidates []candidate `json:"candidates"`
	TopN       int         `json:"top_n,omitempty"`
}

type scoredCandidate struct {
	ID    string  `json:"id"`
	Score float64 `json:"score"`
	Text  string  `json:"text,omitempty"`
}

type rerankResponse struct {
	Results []scoredCandidate `json:"results"`
	Model   string            `json:"model"`
}

// Rerank runs L1 then L2 rerank pipeline on candidates.
// Tries the /v1/rerank/combined endpoint first (single call, faster),
// falls back to sequential L1→L2 if combined is unavailable.
func (c *Client) Rerank(ctx context.Context, query string, chunks []model.Chunk, finalTopK int) ([]model.Chunk, error) {
	start := time.Now()
	if len(chunks) == 0 {
		return chunks, nil
	}
	recordCost := func(model string, out []model.Chunk, err error) {
		var inTok int64
		inTok += cost.EstimateTokens(query)
		for _, ch := range chunks {
			inTok += cost.EstimateTokens(BuildRerankPassage(ch))
		}
		status := "ok"
		if err != nil {
			status = "error"
		}
		cost.RecordAsync(ctx, cost.Event{
			Provider: "worker", Model: model, Operation: "rerank",
			InputTokens: inTok, ItemCount: len(chunks),
			LatencyMS: int(time.Since(start).Milliseconds()), Status: status,
		})
		_ = out
	}

	// Try combined endpoint first (single HTTP call for L1+L2).
	combinedTopN := finalTopK
	if combinedTopN <= 0 {
		combinedTopN = c.l2TopN
	}
	if combined, err := c.call(ctx, "/v1/rerank/combined", query, chunks, combinedTopN); err == nil {
		for i := range combined {
			combined[i].Source = "rerank_combined"
		}
		observability.RecordRerankLatency("worker", "combined", time.Since(start).Seconds())
		recordCost("combined", combined, nil)
		return combined, nil
	}

	// Fallback: sequential L1 → L2.
	l1Out, err := c.call(ctx, "/v1/rerank/l1", query, chunks, c.l1TopN)
	if err != nil {
		recordCost("l1", nil, err)
		return nil, err
	}
	observability.RecordRerankLatency("worker", "l2", time.Since(start).Seconds())
	if len(l1Out) == 0 {
		recordCost("l1", l1Out, nil)
		return []model.Chunk{}, nil
	}

	l2Top := c.l2TopN
	if finalTopK > 0 && finalTopK < l2Top {
		l2Top = finalTopK
	}
	l2Out, err := c.call(ctx, "/v1/rerank/l2", query, l1Out, l2Top)
	if err != nil {
		// Degrade: return L1 results truncated.
		if len(l1Out) > finalTopK && finalTopK > 0 {
			l1Out = l1Out[:finalTopK]
		}
		for i := range l1Out {
			l1Out[i].Source = "rerank_l1_degraded"
		}
		recordCost("l1_degraded", l1Out, err)
		return l1Out, nil
	}
	for i := range l2Out {
		l2Out[i].Source = "rerank_l2"
	}
	observability.RecordRerankLatency("worker", "l2", time.Since(start).Seconds())
	recordCost("l2", l2Out, nil)
	return l2Out, nil
}

func (c *Client) call(ctx context.Context, path, query string, chunks []model.Chunk, topN int) ([]model.Chunk, error) {
	cands := make([]candidate, len(chunks))
	byID := make(map[string]model.Chunk, len(chunks))
	for i, ch := range chunks {
		cands[i] = candidate{ID: ch.ID, Text: BuildRerankPassage(ch)}
		byID[ch.ID] = ch
	}
	body, _ := json.Marshal(rerankRequest{Query: query, Candidates: cands, TopN: topN})
	url := c.baseURL + path
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("worker rerank %s: %w", path, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		b, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("worker rerank %s status %d: %s", path, resp.StatusCode, string(b))
	}

	var parsed rerankResponse
	if err := json.NewDecoder(resp.Body).Decode(&parsed); err != nil {
		return nil, err
	}

	out := make([]model.Chunk, 0, len(parsed.Results))
	for _, r := range parsed.Results {
		ch, ok := byID[r.ID]
		if !ok {
			continue
		}
		ch.RerankScore = r.Score
		ch.Score = r.Score
		out = append(out, ch)
	}
	return out, nil
}
