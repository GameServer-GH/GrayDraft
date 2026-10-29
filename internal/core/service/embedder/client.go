package embedder

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"

	"github.com/graydraft/core/config"
	"github.com/graydraft/core/observability"
	"github.com/graydraft/core/service/cost"
)

// Client calls the model gateway embedding API.
type Client struct {
	baseURL string
	model   string
	http    *http.Client

	// In-memory embedding cache to avoid redundant API calls for identical queries.
	embCache   map[string]embCacheEntry
	embCacheMu sync.RWMutex
}

type embCacheEntry struct {
	vec    []float32
	expiry time.Time
}

const embCacheMaxSize = 2000
const embCacheTTL = 10 * time.Minute

// NewClient creates a gateway embedding client. Returns nil if URL empty.
func NewClient(cfg *config.Config) *Client {
	if cfg.ModelGateway.URL == "" {
		return nil
	}
	return &Client{
		baseURL:  cfg.ModelGateway.URL,
		model:    cfg.ModelGateway.EmbeddingModel,
		embCache: make(map[string]embCacheEntry),
		http: &http.Client{
			Timeout: time.Duration(cfg.ModelGateway.TimeoutSeconds) * time.Second,
		},
	}
}

type embedRequest struct {
	Texts []string `json:"texts"`
	Model string   `json:"model,omitempty"`
}

type embedResponse struct {
	Vectors [][]float32 `json:"vectors"`
	Model   string      `json:"model"`
}

func embCacheKey(query, model string) string {
	h := sha256.Sum256([]byte(query + "|" + model))
	return hex.EncodeToString(h[:16])
}

func (c *Client) embCacheGet(key string) ([]float32, bool) {
	c.embCacheMu.RLock()
	e, ok := c.embCache[key]
	c.embCacheMu.RUnlock()
	if !ok || time.Now().After(e.expiry) {
		return nil, false
	}
	return e.vec, true
}

func (c *Client) embCacheSet(key string, vec []float32) {
	c.embCacheMu.Lock()
	defer c.embCacheMu.Unlock()
	// Evict oldest entries if over capacity.
	if len(c.embCache) >= embCacheMaxSize {
		for k := range c.embCache {
			delete(c.embCache, k)
			break
		}
	}
	c.embCache[key] = embCacheEntry{vec: vec, expiry: time.Now().Add(embCacheTTL)}
}

// EmbedQuery returns the embedding vector for a single query, with local cache.
func (c *Client) EmbedQuery(ctx context.Context, query string) ([]float32, error) {
	key := embCacheKey(query, c.model)
	if vec, ok := c.embCacheGet(key); ok {
		return vec, nil
	}
	vecs, err := c.EmbedTexts(ctx, []string{query})
	if err != nil {
		return nil, err
	}
	if len(vecs) == 0 {
		return nil, fmt.Errorf("empty embedding response")
	}
	c.embCacheSet(key, vecs[0])
	return vecs[0], nil
}

// EmbedTexts embeds multiple texts.
func (c *Client) EmbedTexts(ctx context.Context, texts []string) ([][]float32, error) {
	start := time.Now()
	model := c.model
	if model == "" {
		model = "default"
	}
	body, err := json.Marshal(embedRequest{Texts: texts, Model: model})
	if err != nil {
		return nil, err
	}
	url := c.baseURL + "/v1/embeddings"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("gateway embed: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		b, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("gateway embed status %d: %s", resp.StatusCode, string(b))
	}

	var parsed embedResponse
	if err := json.NewDecoder(resp.Body).Decode(&parsed); err != nil {
		return nil, err
	}
	observability.RecordEmbeddingLatency("gateway", model, time.Since(start).Seconds())
	var inTok int64
	for _, t := range texts {
		inTok += cost.EstimateTokens(t)
	}
	observability.RecordModelTokenUsage("", "gateway", model, float64(inTok))
	cost.RecordAsync(ctx, cost.Event{
		Provider: "gateway", Model: model, Operation: "embedding",
		InputTokens: inTok, ItemCount: len(texts),
		LatencyMS: int(time.Since(start).Milliseconds()), Status: "ok",
	})
	return parsed.Vectors, nil
}
