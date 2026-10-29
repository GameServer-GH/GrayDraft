// Package answercache implements semantic answer cache shadow mode (V1-17).
// Shadow mode records semantic cache hits alongside fresh LLM answers for
// offline accuracy calibration without serving cached answers to users.
package answercache

import (
	"context"
	"encoding/json"
	"log/slog"
	"sync"
	"time"

	"github.com/graydraft/core/config"
)

// ShadowEntry records a semantic cache suggestion alongside the actual LLM answer.
type ShadowEntry struct {
	Timestamp       time.Time         `json:"timestamp"`
	Query           string            `json:"query"`
	KBType          string            `json:"kb_type"`
	CacheHit        bool              `json:"cache_hit"`
	CacheAnswer     string            `json:"cache_answer,omitempty"`
	CacheSimilarity float64           `json:"cache_similarity,omitempty"`
	LLMAnswer       string            `json:"llm_answer,omitempty"`
	LLMTokens       int               `json:"llm_tokens,omitempty"`
	AgreementScore  float64           `json:"agreement_score,omitempty"` // 0-1, filled later by batch eval
	Semantic4DFP    string            `json:"semantic4d_fingerprint,omitempty"`
}

// ShadowConfig controls shadow mode behavior.
type ShadowConfig struct {
	Enabled       bool    `json:"enabled"`
	SampleRate    float64 `json:"sample_rate"`    // 0.0-1.0, fraction of queries to shadow-track
	MinSimilarity float64 `json:"min_similarity"` // minimum cosine to record
	MaxBufferSize int     `json:"max_buffer_size"`
	FlushInterval time.Duration
}

// ShadowRecorder collects semantic cache hits in shadow mode.
type ShadowRecorder struct {
	cfg    ShadowConfig
	mu     sync.Mutex
	buffer []ShadowEntry
	logger *slog.Logger
}

// NewShadowRecorder creates a shadow mode recorder from config.
func NewShadowRecorder(cfg *config.Config) *ShadowRecorder {
	sc := ShadowConfig{
		Enabled:       cfg.AnswerCache.ShadowMode,
		SampleRate:    cfg.AnswerCache.ShadowSampleRate,
		MinSimilarity: cfg.Cache.SimilarityThreshold,
		MaxBufferSize: 1000,
		FlushInterval: 60 * time.Second,
	}
	if sc.SampleRate <= 0 {
		sc.SampleRate = 0.1
	}
	if sc.SampleRate > 1.0 {
		sc.SampleRate = 1.0
	}
	if sc.MaxBufferSize == 0 {
		sc.MaxBufferSize = 1000
	}
	return &ShadowRecorder{
		cfg:    sc,
		buffer: make([]ShadowEntry, 0, sc.MaxBufferSize),
		logger: slog.With("component", "answer_shadow"),
	}
}

// Record logs a shadow comparison when both cache and LLM answers are available.
func (sr *ShadowRecorder) Record(ctx context.Context, e ShadowEntry) {
	if sr == nil || !sr.cfg.Enabled {
		return
	}
	// Probabilistic sampling to limit overhead.
	if sr.cfg.SampleRate < 1.0 && !sr.shouldSample() {
		return
	}
	if e.CacheSimilarity < sr.cfg.MinSimilarity {
		return
	}

	sr.mu.Lock()
	sr.buffer = append(sr.buffer, e)
	needsFlush := len(sr.buffer) >= sr.cfg.MaxBufferSize
	sr.mu.Unlock()

	if needsFlush {
		sr.flush()
	}
}

// FlushNow forces a flush of accumulated shadow entries.
func (sr *ShadowRecorder) FlushNow() {
	sr.flush()
}

func (sr *ShadowRecorder) flush() {
	sr.mu.Lock()
	if len(sr.buffer) == 0 {
		sr.mu.Unlock()
		return
	}
	entries := sr.buffer
	sr.buffer = make([]ShadowEntry, 0, sr.cfg.MaxBufferSize)
	sr.mu.Unlock()

	// Log as structured JSON for offline analysis.
	for i := range entries {
		data, err := json.Marshal(entries[i])
		if err != nil {
			continue
		}
		sr.logger.Info("shadow_answer_cache",
			"query", entries[i].Query,
			"kb_type", entries[i].KBType,
			"cache_hit", entries[i].CacheHit,
			"cache_similarity", entries[i].CacheSimilarity,
			"llm_tokens", entries[i].LLMTokens,
			"payload", string(data),
		)
	}
}

func (sr *ShadowRecorder) shouldSample() bool {
	// Simple modulo-based sampling; for production use a hash-based approach.
	// For now use nanosecond parity.
	return time.Now().UnixNano()%100 < int64(sr.cfg.SampleRate*100)
}

// GetStats returns shadow mode buffer size.
func (sr *ShadowRecorder) GetStats() map[string]any {
	if sr == nil {
		return map[string]any{"enabled": false}
	}
	sr.mu.Lock()
	n := len(sr.buffer)
	sr.mu.Unlock()
	return map[string]any{
		"enabled":        sr.cfg.Enabled,
		"sample_rate":    sr.cfg.SampleRate,
		"buffer_size":    n,
		"max_buffer":     sr.cfg.MaxBufferSize,
		"min_similarity": sr.cfg.MinSimilarity,
	}
}
