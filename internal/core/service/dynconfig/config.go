// Package dynconfig provides hot-reloadable configuration and rate limiting.
// V1-38: dynamic config refresh via file watch or Redis pub/sub.
// V1-39: per-workspace ingest and chat rate limiting.
package dynconfig

import (
	"context"
	"log/slog"
	"sync"
	"sync/atomic"
	"time"

	"github.com/graydraft/core/config"
)

// HotConfig holds values that can change at runtime without restart.
type HotConfig struct {
	// Retrieval budget.
	FulltextCandidates     int32
	VectorCandidates       int32
	StructuredCandidates   int32
	RRFCandidates          int32
	MaxContextChunks       int32
	MaxContextTokens       int32

	// Rate limits.
	IngestQPS              int32
	ChatConcurrency        int32
	EmbeddingTokensDaily   int64
	MaxTaskChunks          int32

	// Toggles.
	SemanticAnswerCacheEnabled int32 // 0=false, 1=true
	RateLimitEnabled          int32
}

// Manager loads initial config from static cfg and supports periodic refresh.
type Manager struct {
	cfg    *config.Config
	hot    atomic.Value // *HotConfig
	mu     sync.Mutex
	cancel context.CancelFunc
}

// New creates a dynconfig manager from static configuration.
func New(cfg *config.Config) *Manager {
	m := &Manager{cfg: cfg}
	m.storeFromStatic(cfg)
	if cfg.DynamicConfig.Enabled && cfg.DynamicConfig.RefreshSeconds > 0 {
		ctx, cancel := context.WithCancel(context.Background())
		m.cancel = cancel
		go m.refreshLoop(ctx, time.Duration(cfg.DynamicConfig.RefreshSeconds)*time.Second)
	}
	return m
}

// Hot returns the current hot configuration.
func (m *Manager) Hot() *HotConfig {
	v := m.hot.Load()
	if v == nil {
		return &HotConfig{}
	}
	return v.(*HotConfig)
}

// Shutdown stops the background refresh loop.
func (m *Manager) Shutdown() {
	if m.cancel != nil {
		m.cancel()
	}
}

// ReloadFromStatic re-reads from the static config pointer (used after file-watch update).
func (m *Manager) ReloadFromStatic(cfg *config.Config) {
	m.storeFromStatic(cfg)
}

func (m *Manager) storeFromStatic(cfg *config.Config) {
	hc := &HotConfig{
		FulltextCandidates:   40,
		VectorCandidates:     40,
		StructuredCandidates: 20,
		RRFCandidates:        30,
		MaxContextChunks:     clampInt32(cfg.Chat.MaxContextChunks, 3, 20),
		MaxContextTokens:     clampInt32(cfg.Chat.MaxContextTokens, 1000, 32000),

		IngestQPS:            clampInt32(cfg.RateLimit.PerWorkspaceIngestQPS, 1, 100),
		ChatConcurrency:      clampInt32(cfg.RateLimit.PerWorkspaceChatConcurrency, 1, 100),
		EmbeddingTokensDaily: cfg.RateLimit.PerWorkspaceEmbeddingTokensDaily,
		MaxTaskChunks:        clampInt32(cfg.RateLimit.MaxTaskChunks, 10, 20000),

		SemanticAnswerCacheEnabled: 0,
		RateLimitEnabled:           boolToInt32(cfg.RateLimit.Enabled),
	}
	m.hot.Store(hc)
	slog.Info("dynconfig loaded", "fulltext_candidates", hc.FulltextCandidates,
		"ingest_qps", hc.IngestQPS, "max_context_chunks", hc.MaxContextChunks)
}

func (m *Manager) refreshLoop(ctx context.Context, interval time.Duration) {
	t := time.NewTicker(interval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			m.storeFromStatic(m.cfg)
		}
	}
}

func clampInt32(v int, lo, hi int) int32 {
	if v <= 0 {
		return int32(lo)
	}
	if v < lo {
		return int32(lo)
	}
	if v > hi {
		return int32(hi)
	}
	return int32(v)
}

func boolToInt32(b bool) int32 {
	if b {
		return 1
	}
	return 0
}
