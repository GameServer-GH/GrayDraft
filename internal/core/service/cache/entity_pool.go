package cache

import (
	"context"
	"strings"
	"time"
)

// GetEntityPool loads a Redis-backed hot entity pool.
func (m *Manager) GetEntityPool(ctx context.Context, scope string) ([]string, bool) {
	if m == nil || m.l2 == nil {
		return nil, false
	}
	return m.l2.GetEntityPool(ctx, scope)
}

// SetEntityPool stores hot entities with an independent TTL.
func (m *Manager) SetEntityPool(ctx context.Context, scope string, entities []string, ttl time.Duration) {
	if m == nil || m.l2 == nil || len(entities) == 0 {
		return
	}
	_ = m.l2.SetEntityPool(ctx, scope, entities, ttl)
}

func (r *RedisL2) GetEntityPool(ctx context.Context, scope string) ([]string, bool) {
	if r == nil {
		return nil, false
	}
	vals, err := r.client.LRange(ctx, entityPoolKey(scope), 0, -1).Result()
	if err != nil || len(vals) == 0 {
		return nil, false
	}
	return vals, true
}

func (r *RedisL2) SetEntityPool(ctx context.Context, scope string, entities []string, ttl time.Duration) error {
	if r == nil {
		return nil
	}
	if ttl <= 0 {
		ttl = time.Hour
	}
	key := entityPoolKey(scope)
	pipe := r.client.Pipeline()
	pipe.Del(ctx, key)
	if len(entities) > 0 {
		args := make([]any, 0, len(entities))
		for _, e := range entities {
			if e = strings.TrimSpace(e); e != "" {
				args = append(args, e)
			}
		}
		if len(args) > 0 {
			pipe.RPush(ctx, key, args...)
		}
	}
	pipe.Expire(ctx, key, ttl)
	_, err := pipe.Exec(ctx)
	return err
}

func entityPoolKey(scope string) string {
	scope = strings.TrimSpace(scope)
	if scope == "" {
		scope = "gd_"
	}
	return redisEntityPoolPrefix + scope
}
