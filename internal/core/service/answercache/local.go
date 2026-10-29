package answercache

import (
	"time"

	"github.com/dgraph-io/ristretto/v2"
)

// LocalL1 is an in-process answer cache.
type LocalL1 struct {
	c *ristretto.Cache[string, *Entry]
}

// NewLocalL1 creates L1 answer cache.
func NewLocalL1(maxItems int, ttl time.Duration) (*LocalL1, error) {
	if maxItems <= 0 {
		maxItems = 5000
	}
	rc, err := ristretto.NewCache(&ristretto.Config[string, *Entry]{
		NumCounters: int64(maxItems * 10),
		MaxCost:     int64(maxItems),
		BufferItems: 64,
	})
	if err != nil {
		return nil, err
	}
	_ = ttl
	return &LocalL1{c: rc}, nil
}

func (l *LocalL1) Get(key string) (*Entry, bool) {
	if l == nil || l.c == nil {
		return nil, false
	}
	v, ok := l.c.Get(key)
	return v, ok
}

func (l *LocalL1) Set(key string, entry *Entry, ttl time.Duration) {
	if l == nil || l.c == nil || entry == nil {
		return
	}
	l.c.SetWithTTL(key, entry, 1, ttl)
}

// Clear drops all in-process answer cache entries.
func (l *LocalL1) Clear() {
	if l == nil || l.c == nil {
		return
	}
	l.c.Clear()
}
