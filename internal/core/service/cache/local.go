package cache

import (
	"time"

	"github.com/dgraph-io/ristretto/v2"
	"github.com/graydraft/core/model"
)

// LocalL1 is an in-process Ristretto cache.
type LocalL1 struct {
	c *ristretto.Cache[string, *model.SearchResponse]
}

// NewLocalL1 creates L1 cache.
func NewLocalL1(maxItems int, ttl time.Duration) (*LocalL1, error) {
	if maxItems <= 0 {
		maxItems = 10000
	}
	if ttl <= 0 {
		ttl = 10 * time.Minute
	}
	rc, err := ristretto.NewCache(&ristretto.Config[string, *model.SearchResponse]{
		NumCounters: int64(maxItems * 10),
		MaxCost:     int64(maxItems),
		BufferItems: 64,
	})
	if err != nil {
		return nil, err
	}
	return &LocalL1{c: rc}, nil
}

// Get returns cached response.
func (l *LocalL1) Get(key string) (*model.SearchResponse, bool) {
	if l == nil || l.c == nil {
		return nil, false
	}
	v, ok := l.c.Get(key)
	return v, ok && v != nil
}

// Set stores response with TTL.
func (l *LocalL1) Set(key string, resp *model.SearchResponse, ttl time.Duration) {
	if l == nil || l.c == nil || resp == nil {
		return
	}
	_ = l.c.SetWithTTL(key, resp, 1, ttl)
	l.c.Wait()
}

// Clear drops all in-process retrieval cache entries.
func (l *LocalL1) Clear() {
	if l == nil || l.c == nil {
		return
	}
	l.c.Clear()
}
