package agent

import (
	"context"
	"sync"
	"time"
)

type ToolSpecStore interface {
	ActiveForTenant(context.Context, string) ([]ToolSpec, error)
}
type TenantToolCache struct {
	mu    sync.RWMutex
	ttl   time.Duration
	items map[string]cacheTools
	store ToolSpecStore
}
type cacheTools struct {
	specs   []ToolSpec
	expires time.Time
}

func NewTenantToolCache(store ToolSpecStore, ttl time.Duration) *TenantToolCache {
	if ttl <= 0 {
		ttl = time.Minute
	}
	return &TenantToolCache{store: store, ttl: ttl, items: map[string]cacheTools{}}
}
func (c *TenantToolCache) ForTenant(ctx context.Context, tenant string) ([]ToolSpec, error) {
	c.mu.RLock()
	v, ok := c.items[tenant]
	c.mu.RUnlock()
	if ok && time.Now().Before(v.expires) {
		return v.specs, nil
	}
	specs, err := c.store.ActiveForTenant(ctx, tenant)
	if err != nil {
		return nil, err
	}
	c.mu.Lock()
	c.items[tenant] = cacheTools{specs: specs, expires: time.Now().Add(c.ttl)}
	c.mu.Unlock()
	return specs, nil
}
func (c *TenantToolCache) Invalidate(tenant string) {
	c.mu.Lock()
	delete(c.items, tenant)
	c.mu.Unlock()
}
