package redis

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/graydraft/core/config"
	"github.com/graydraft/core/service/agent"
	redisgo "github.com/redis/go-redis/v9"
)

type MemoryCache struct{ client *redisgo.Client }

func NewMemoryCache(cfg *config.Config) (*MemoryCache, error) {
	c := redisgo.NewClient(&redisgo.Options{Addr: fmt.Sprintf("%s:%d", cfg.Redis.Host, cfg.Redis.Port), Password: cfg.Redis.Password, DB: cfg.Redis.DB})
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := c.Ping(ctx).Err(); err != nil {
		_ = c.Close()
		return nil, err
	}
	return &MemoryCache{client: c}, nil
}
func (m *MemoryCache) Get(ctx context.Context, key string) ([]agent.MemoryMessage, bool) {
	b, err := m.client.Get(ctx, "knex:agent:memory:"+key).Bytes()
	if err != nil {
		return nil, false
	}
	var out []agent.MemoryMessage
	if json.Unmarshal(b, &out) != nil {
		return nil, false
	}
	return out, true
}
func (m *MemoryCache) Set(ctx context.Context, key string, v []agent.MemoryMessage, ttl time.Duration) {
	k := "knex:agent:memory:" + key
	if v == nil {
		_ = m.client.Del(ctx, k).Err()
		return
	}
	b, err := json.Marshal(v)
	if err == nil {
		_ = m.client.Set(ctx, k, b, ttl).Err()
	}
}
func (m *MemoryCache) Close() error { return m.client.Close() }
