package cache

import (
	"context"
	"log/slog"
	"time"

	"github.com/graydraft/core/config"
	"github.com/graydraft/core/model"
)

// Manager orchestrates L1 exact + L2 Redis exact/semantic cache lookup.
type Manager struct {
	cfg       *config.Config
	l1        *LocalL1
	l2        *RedisL2
	ttl       time.Duration
	threshold float64
}

// NewManager wires cache tiers from config.
func NewManager(cfg *config.Config) (*Manager, error) {
	if !cfg.Cache.Enabled {
		return &Manager{cfg: cfg}, nil
	}
	ttl := time.Duration(cfg.Cache.TTLSeconds) * time.Second
	l1, err := NewLocalL1(cfg.Cache.L1MaxItems, ttl)
	if err != nil {
		return nil, err
	}
	l2, err := NewRedisL2(cfg)
	if err != nil {
		slog.Warn("redis cache unavailable, L1 only", "error", err)
	}
	return &Manager{
		cfg:       cfg,
		l1:        l1,
		l2:        l2,
		ttl:       ttl,
		threshold: cfg.Cache.SimilarityThreshold,
	}, nil
}

// Lookup tries L1, L2 exact, and L2 semantic cache hits.
func (m *Manager) Lookup(ctx context.Context, p LookupParams, embedding []float32) (*model.SearchResponse, string, bool) {
	if m == nil || !m.cfg.Cache.Enabled {
		return nil, "", false
	}
	key := BuildExactKey(p)
	indexKey := IndexKey(p.KBType, p.CatalogVersion, p.CatalogUpdated, p.Semantic4DFingerprint)

	if resp, ok := m.l1.Get(key); ok {
		out := cloneResponse(resp)
		out.Strategy = "cache_l1"
		out.CacheHit = "l1"
		return out, "l1", true
	}

	if m.l2 != nil {
		if resp, ok := m.l2.GetExact(ctx, key); ok {
			out := cloneResponse(resp)
			out.Strategy = "cache_l2"
			out.CacheHit = "l2_exact"
			m.l1.Set(key, out, m.ttl)
			return out, "l2_exact", true
		}
		if len(embedding) > 0 {
			if resp, ok := m.l2.GetSemantic(ctx, indexKey, embedding, m.threshold); ok {
				m.l1.Set(key, resp, m.ttl)
				return resp, "l2_semantic", true
			}
		}
	}

	return nil, "", false
}

// Store writes result to L1 and L2 asynchronously-safe sync call.
func (m *Manager) Store(ctx context.Context, p LookupParams, embedding []float32, resp *model.SearchResponse) {
	if m == nil || !m.cfg.Cache.Enabled || resp == nil {
		return
	}
	key := BuildExactKey(p)
	indexKey := IndexKey(p.KBType, p.CatalogVersion, p.CatalogUpdated, p.Semantic4DFingerprint)
	stored := cloneResponse(resp)
	stored.CacheHit = ""

	m.l1.Set(key, stored, m.ttl)
	if m.l2 != nil {
		if err := m.l2.Set(ctx, key, indexKey, p.Query, embedding, stored); err != nil {
			slog.Warn("redis cache set failed", "error", err)
		}
	}
}

// Ping checks L2 Redis if configured.
func (m *Manager) Ping(ctx context.Context) error {
	if m == nil || m.l2 == nil {
		return nil
	}
	return m.l2.Ping(ctx)
}

// RedisStatsByKBType returns retrieval cache entry count in Redis for kb_type.
func (m *Manager) RedisStatsByKBType(ctx context.Context, kbType string) (int, error) {
	if m == nil || m.l2 == nil {
		return 0, nil
	}
	return m.l2.StatsByKBType(ctx, kbType)
}

// RedisPurgeByKBType clears Redis retrieval cache for kb_type and in-process L1.
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

func cloneResponse(r *model.SearchResponse) *model.SearchResponse {
	if r == nil {
		return nil
	}
	cp := *r
	if r.Chunks != nil {
		cp.Chunks = make([]model.Chunk, len(r.Chunks))
		copy(cp.Chunks, r.Chunks)
	}
	return &cp
}

// FineGrainedPurge removes cache entries matching specific entities or title terms.
// This is more precise than PurgeByKBType and avoids evicting unrelated queries.
func (m *Manager) FineGrainedPurge(ctx context.Context, kbType string, entities []string, titleTerms []string) (int, error) {
	if m == nil || !m.cfg.Cache.FineGrainedInvalidation {
		return m.RedisPurgeByKBType(ctx, kbType)
	}
	if m.l1 != nil {
		m.l1.Clear()
	}
	removed := 0
	if m.l2 != nil {
		n, err := m.l2.FineGrainedPurge(ctx, kbType, entities, titleTerms)
		if err != nil {
			return removed, err
		}
		removed += n
	}
	return removed, nil
}
