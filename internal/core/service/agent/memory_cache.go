package agent

import (
	"context"
	"sync"
	"time"
)

// MemoryCache is intentionally small so Redis can be added without changing
// ContextBuilder. The in-process implementation is a safe fallback only.
type MemoryCache interface {
	Get(context.Context, string) ([]MemoryMessage, bool)
	Set(context.Context, string, []MemoryMessage, time.Duration)
}

type cachedMemory struct {
	base  ShortTermMemory
	cache MemoryCache
	key   string
}

func CachedMemory(base ShortTermMemory, cache MemoryCache, key string) ShortTermMemory {
	return &cachedMemory{base: base, cache: cache, key: key}
}
func (m *cachedMemory) Append(ctx context.Context, conversationID string, msg MemoryMessage) error {
	err := m.base.Append(ctx, conversationID, msg)
	if err == nil && m.cache != nil {
		m.cache.Set(ctx, m.key, nil, 0)
	}
	return err
}
func (m *cachedMemory) Recent(ctx context.Context, conversationID string, limit int) ([]MemoryMessage, error) {
	if m.cache != nil {
		if v, ok := m.cache.Get(ctx, m.key); ok && v != nil {
			return v, nil
		}
	}
	v, err := m.base.Recent(ctx, conversationID, limit)
	if err == nil && m.cache != nil {
		m.cache.Set(ctx, m.key, v, 30*time.Second)
	}
	return v, err
}

func (m *cachedMemory) LatestSummary(ctx context.Context, conversationID string) (string, error) {
	if p, ok := m.base.(SummaryProvider); ok {
		return p.LatestSummary(ctx, conversationID)
	}
	return "", nil
}

func (m *cachedMemory) Compact(ctx context.Context, conversationID string, keepRecent int) error {
	if p, ok := m.base.(MemoryCompactor); ok {
		return p.Compact(ctx, conversationID, keepRecent)
	}
	return nil
}

type cacheEntry struct {
	values  []MemoryMessage
	expires time.Time
}
type InProcessMemoryCache struct {
	mu   sync.RWMutex
	data map[string]cacheEntry
}

func NewInProcessMemoryCache() *InProcessMemoryCache {
	return &InProcessMemoryCache{data: map[string]cacheEntry{}}
}
func (c *InProcessMemoryCache) Get(_ context.Context, key string) ([]MemoryMessage, bool) {
	c.mu.RLock()
	e, ok := c.data[key]
	c.mu.RUnlock()
	if !ok || (!e.expires.IsZero() && time.Now().After(e.expires)) {
		return nil, false
	}
	return append([]MemoryMessage(nil), e.values...), true
}
func (c *InProcessMemoryCache) Set(_ context.Context, key string, v []MemoryMessage, ttl time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if v == nil {
		delete(c.data, key)
		return
	}
	e := cacheEntry{values: append([]MemoryMessage(nil), v...)}
	if ttl > 0 {
		e.expires = time.Now().Add(ttl)
	}
	c.data[key] = e
}
