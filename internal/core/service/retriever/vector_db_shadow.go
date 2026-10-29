// Package retriever implements a shadow vector database comparison framework (V1-29).
// Supports running parallel queries against pgvector (primary) and alternative
// vector databases (Qdrant, Milvus, Vespa) for latency and recall comparison.
package retriever

import (
	"context"
	"encoding/json"
	"log/slog"
	"sync"
	"time"
)

// ShadowVectorDBType enumerates supported alternative vector stores.
type ShadowVectorDBType string

const (
	ShadowDBQdrant ShadowVectorDBType = "qdrant"
	ShadowDBMilvus ShadowVectorDBType = "milvus"
	ShadowDBVespa  ShadowVectorDBType = "vespa"
)

// ShadowVectorResult holds a single vector search result from an alternative store.
type ShadowVectorResult struct {
	ChunkID  string  `json:"chunk_id"`
	Score    float64 `json:"score"`
}

// ShadowQueryResult records a comparison between pgvector and a shadow DB.
type ShadowQueryResult struct {
	Timestamp   time.Time            `json:"timestamp"`
	QueryHash   string               `json:"query_hash"`
	KBType      string               `json:"kb_type"`
	TopK        int                  `json:"top_k"`
	DBType      ShadowVectorDBType   `json:"db_type"`

	// Latency.
	PgvectorLatencyMs float64 `json:"pgvector_latency_ms"`
	ShadowLatencyMs   float64 `json:"shadow_latency_ms"`

	// Overlap metrics.
	OverlapCount     int     `json:"overlap_count"`       // chunks in both result sets
	JaccardOverlap   float64 `json:"jaccard_overlap"`     // Jaccard similarity of result sets
	RankCorrelation  float64 `json:"rank_correlation"`    // Spearman correlation of scores

	// Per-result details (optional, for debugging).
	PgvectorResults []ShadowVectorResult `json:"pgvector_results,omitempty"`
	ShadowResults   []ShadowVectorResult `json:"shadow_results,omitempty"`
}

// ShadowVectorDBConfig configures a shadow vector database comparison.
type ShadowVectorDBConfig struct {
	Enabled    bool                `json:"enabled"`
	DBType     ShadowVectorDBType `json:"db_type"`
	Endpoint   string             `json:"endpoint"`
	Collection string             `json:"collection"`  // collection/index name
	APIKey     string             `json:"api_key,omitempty"`
	SampleRate float64            `json:"sample_rate"` // fraction of queries to compare
	MaxDim     int                `json:"max_dim"`     // embedding dimension
}

// ShadowVectorDBClient abstracts the interface for querying an alternative vector DB.
type ShadowVectorDBClient interface {
	// Search performs a vector similarity search.
	Search(ctx context.Context, collection string, vector []float32, topK int, filter map[string]any) ([]ShadowVectorResult, error)
	// Ping checks connectivity.
	Ping(ctx context.Context) error
}

// ShadowVectorComparator runs dual queries against pgvector and a shadow DB.
type ShadowVectorComparator struct {
	cfg     ShadowVectorDBConfig
	client  ShadowVectorDBClient
	mu      sync.Mutex
	buffer  []ShadowQueryResult
	logger  *slog.Logger
}

// NewShadowVectorComparator creates a shadow comparator for a specific DB type.
func NewShadowVectorComparator(cfg ShadowVectorDBConfig, client ShadowVectorDBClient) *ShadowVectorComparator {
	if cfg.SampleRate <= 0 {
		cfg.SampleRate = 0.05
	}
	if cfg.SampleRate > 1.0 {
		cfg.SampleRate = 1.0
	}
	return &ShadowVectorComparator{
		cfg:    cfg,
		client: client,
		buffer: make([]ShadowQueryResult, 0, 500),
		logger: slog.With("component", "shadow_vector_db"),
	}
}

// Enabled reports whether shadow comparison is active.
func (svc *ShadowVectorComparator) Enabled() bool {
	return svc != nil && svc.cfg.Enabled && svc.client != nil
}

// Compare runs a shadow query against the alternative DB and records metrics.
// pgvectorResults are the results from the primary pgvector query.
// queryEmbedding is the normalized query vector.
func (svc *ShadowVectorComparator) Compare(
	ctx context.Context,
	queryHash string,
	kbType string,
	topK int,
	queryEmbedding []float32,
	pgvectorResults []ShadowVectorResult,
	pgvectorLatency time.Duration,
) {
	if svc == nil || !svc.cfg.Enabled || svc.client == nil {
		return
	}
	// Probabilistic sampling.
	if svc.cfg.SampleRate < 1.0 {
		if time.Now().UnixNano()%100 >= int64(svc.cfg.SampleRate*100) {
			return
		}
	}

	shadowStart := time.Now()
	shadowResults, err := svc.client.Search(ctx, svc.cfg.Collection, queryEmbedding, topK, nil)
	shadowLatency := time.Since(shadowStart)

	result := ShadowQueryResult{
		Timestamp:        time.Now().UTC(),
		QueryHash:        queryHash,
		KBType:           kbType,
		TopK:             topK,
		DBType:           svc.cfg.DBType,
		PgvectorLatencyMs: float64(pgvectorLatency.Microseconds()) / 1000.0,
		ShadowLatencyMs:   float64(shadowLatency.Microseconds()) / 1000.0,
		PgvectorResults:   pgvectorResults,
	}

	if err != nil {
		svc.logger.Warn("shadow vector query failed",
			"db_type", svc.cfg.DBType,
			"error", err,
		)
		// Still log for observability.
		svc.logResult(result)
		return
	}

	result.ShadowResults = shadowResults
	result.OverlapCount, result.JaccardOverlap = computeOverlap(pgvectorResults, shadowResults)
	result.RankCorrelation = computeRankCorrelation(pgvectorResults, shadowResults)

	svc.logResult(result)
}

func (svc *ShadowVectorComparator) logResult(r ShadowQueryResult) {
	svc.mu.Lock()
	svc.buffer = append(svc.buffer, r)
	needsFlush := len(svc.buffer) >= 500
	svc.mu.Unlock()

	if needsFlush {
		svc.flush()
	}

	data, _ := json.Marshal(r)
	svc.logger.Info("shadow_vector_compare",
		"db_type", r.DBType,
		"kb_type", r.KBType,
		"pgvector_ms", r.PgvectorLatencyMs,
		"shadow_ms", r.ShadowLatencyMs,
		"overlap", r.OverlapCount,
		"jaccard", r.JaccardOverlap,
		"payload", string(data),
	)
}

func (svc *ShadowVectorComparator) flush() {
	svc.mu.Lock()
	svc.buffer = svc.buffer[:0]
	svc.mu.Unlock()
}

// computeOverlap returns overlap count and Jaccard similarity between two result sets.
func computeOverlap(a, b []ShadowVectorResult) (overlap int, jaccard float64) {
	aSet := make(map[string]bool, len(a))
	for _, r := range a {
		aSet[r.ChunkID] = true
	}
	bSet := make(map[string]bool, len(b))
	for _, r := range b {
		bSet[r.ChunkID] = true
		// Count overlap.
		if aSet[r.ChunkID] {
			overlap++
		}
	}
	union := len(aSet)
	for id := range bSet {
		if !aSet[id] {
			union++
		}
	}
	if union > 0 {
		jaccard = float64(overlap) / float64(union)
	}
	return overlap, jaccard
}

// computeRankCorrelation returns a simple rank overlap score (0-1).
func computeRankCorrelation(a, b []ShadowVectorResult) float64 {
	rankMap := make(map[string]int, len(a))
	for i, r := range a {
		rankMap[r.ChunkID] = i
	}
	if len(b) == 0 {
		return 0
	}
	matches := 0
	for i, r := range b {
		if _, ok := rankMap[r.ChunkID]; ok {
			// Simple binary match at any rank position.
			matches++
			_ = i // rank position in shadow - could compute Kendall's tau
		}
	}
	return float64(matches) / float64(len(b))
}

// GetStats returns shadow comparison statistics.
func (svc *ShadowVectorComparator) GetStats() map[string]any {
	if svc == nil {
		return map[string]any{"enabled": false}
	}
	return map[string]any{
		"enabled":     svc.cfg.Enabled,
		"db_type":     svc.cfg.DBType,
		"endpoint":    svc.cfg.Endpoint,
		"sample_rate": svc.cfg.SampleRate,
	}
}
