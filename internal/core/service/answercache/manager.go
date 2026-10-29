package answercache

import (
	"context"
	"log/slog"
	"time"

	"github.com/graydraft/core/config"
)

// Manager provides L1 → L2 answer cache for Chat.
type Manager struct {
	cfg      *config.Config
	l1       *LocalL1
	l2       *RedisL2
	ttl      time.Duration
	halfTTL  time.Duration
	shadow   *ShadowRecorder  // V1-17: shadow mode
	prefetch *PrefetchManager // V1-19: session prefetch
}

// NewManager wires answer cache tiers from config.
func NewManager(cfg *config.Config) (*Manager, error) {
	if !cfg.AnswerCache.Enabled {
		return &Manager{cfg: cfg}, nil
	}
	ttl := time.Duration(cfg.AnswerCache.TTLSeconds) * time.Second
	if ttl <= 0 {
		ttl = 30 * time.Minute
	}
	halfTTL := ttl / 2
	l1, err := NewLocalL1(cfg.AnswerCache.L1MaxItems, ttl)
	if err != nil {
		return nil, err
	}
	l2, err := NewRedisL2(cfg)
	if err != nil {
		slog.Warn("answer cache redis unavailable, L1 only", "error", err)
	}
	m := &Manager{cfg: cfg, l1: l1, l2: l2, ttl: ttl, halfTTL: halfTTL}
	if cfg.AnswerCache.ShadowMode {
		m.shadow = NewShadowRecorder(cfg)
	}
	// V1-19: session prefetch (disabled by default, enabled after shadow calibration)
	if cfg.AnswerCache.PrefetchEnabled {
		m.prefetch = NewPrefetchManager(PrefetchConfig{
			Enabled:        true,
			MaxPerSession:  5,
			MaxConcurrency: 3,
			TTL:            5 * time.Minute,
			LowPriorityTTL: 2 * time.Minute,
		})
	}
	return m, nil
}

// Lookup tries L1 then L2 exact match.
func (m *Manager) Lookup(ctx context.Context, p LookupParams) (*Entry, string, bool) {
	if m == nil || !m.cfg.AnswerCache.Enabled {
		return nil, "", false
	}
	key := BuildKey(p)

	if m.l1 != nil {
		if entry, ok := m.l1.Get(key); ok && entry != nil && entry.Answer != "" {
			return cloneEntry(entry), "l1", true
		}
	}
	if m.l2 != nil {
		if entry, ok := m.l2.Get(ctx, key); ok {
			out := cloneEntry(entry)
			m.l1.Set(key, out, m.ttl)
			return out, "l2", true
		}
	}
	return nil, "", false
}

// Store writes answer to L1 and L2.
func (m *Manager) Store(ctx context.Context, p LookupParams, entry *Entry) {
	m.StoreWithTTL(ctx, p, entry, m.ttl)
}

// StoreWithTTL writes answer with a custom TTL (for half-TTL scenarios like grade C).
func (m *Manager) StoreWithTTL(ctx context.Context, p LookupParams, entry *Entry, ttl time.Duration) {
	if m == nil || !m.cfg.AnswerCache.Enabled || entry == nil || entry.Answer == "" {
		return
	}
	key := BuildKey(p)
	stored := cloneEntry(entry)
	if m.l1 != nil {
		m.l1.Set(key, stored, ttl)
	}
	if m.l2 != nil {
		if err := m.l2.Set(ctx, key, p.KBType, stored); err != nil {
			slog.Warn("answer cache redis set failed", "error", err)
		}
	}
}

// StoreHalfTTL stores with half the default TTL (for grade C / partial hits).
func (m *Manager) StoreHalfTTL(ctx context.Context, p LookupParams, entry *Entry) {
	m.StoreWithTTL(ctx, p, entry, m.halfTTL)
}

// EvictLowerGrades removes B/C grade entries for the same query when an A-grade entry is written.
// TODO: Implement when RedisL2 supports key deletion. Currently relies on TTL expiry.
func (m *Manager) EvictLowerGrades(ctx context.Context, p LookupParams) {
	// No-op: RedisL2 doesn't have a Delete method yet.
	// Lower grades will naturally expire via their shorter TTLs.
}

// RedisStatsByKBType returns tracked answer cache entry count in Redis.
func (m *Manager) RedisStatsByKBType(ctx context.Context, kbType string) (int, error) {
	if m == nil || m.l2 == nil {
		return 0, nil
	}
	return m.l2.StatsByKBType(ctx, kbType)
}

// RedisPurgeByKBType clears Redis answer cache for kb_type and in-process L1.
func (m *Manager) RedisPurgeByKBType(ctx context.Context, kbType string) (int, error) {
	if m == nil {
		return 0, nil
	}
	if m.l1 != nil {
		m.l1.Clear()
	}
	if m.l2 == nil {
		return 0, nil
	}
	return m.l2.PurgeByKBType(ctx, kbType)
}

// ShadowModeEnabled reports whether shadow mode is active.
func (m *Manager) ShadowModeEnabled() bool {
	return m != nil && m.shadow != nil && m.cfg.AnswerCache.ShadowMode
}

// RecordShadow logs a shadow comparison between cache hit and LLM answer.
func (m *Manager) RecordShadow(ctx context.Context, query, kbType string, cacheHit bool, cacheSimilarity float64, cacheAnswer, llmAnswer string, llmTokens int, semantic4DFP string) {
	if m == nil || m.shadow == nil {
		return
	}
	m.shadow.Record(ctx, ShadowEntry{
		Timestamp:       time.Now().UTC(),
		Query:           query,
		KBType:          kbType,
		CacheHit:        cacheHit,
		CacheAnswer:     cacheAnswer,
		CacheSimilarity: cacheSimilarity,
		LLMAnswer:       llmAnswer,
		LLMTokens:       llmTokens,
		Semantic4DFP:    semantic4DFP,
	})
}

// PrefetchManager returns the session prefetch manager (may be nil).
func (m *Manager) PrefetchManager() *PrefetchManager {
	if m == nil {
		return nil
	}
	return m.prefetch
}

// PrefetchAsync triggers a background prefetch for a session.
func (m *Manager) PrefetchAsync(ctx context.Context, sessionID string, query string, kbType string) {
	if m == nil || m.prefetch == nil {
		return
	}
	m.prefetch.PrefetchAsync(ctx, sessionID, query, kbType)
}

// Ping checks L2 Redis if configured.
func (m *Manager) Ping(ctx context.Context) error {
	if m == nil || m.l2 == nil {
		return nil
	}
	return m.l2.Ping(ctx)
}

// Enabled reports whether answer cache is on.
func (m *Manager) Enabled() bool {
	return m != nil && m.cfg.AnswerCache.Enabled
}

func cloneEntry(e *Entry) *Entry {
	if e == nil {
		return nil
	}
	cp := *e
	return &cp
}
