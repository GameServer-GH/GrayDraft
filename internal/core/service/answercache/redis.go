package answercache

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/graydraft/core/config"
	"github.com/redis/go-redis/v9"
)

const redisPrefix = "knex:ans:"

// RedisL2 stores answer cache entries in Redis.
type RedisL2 struct {
	client *redis.Client
	ttl    time.Duration
}

// NewRedisL2 connects to Redis. Returns nil if disabled.
func NewRedisL2(cfg *config.Config) (*RedisL2, error) {
	if !cfg.AnswerCache.Enabled || !cfg.AnswerCache.RedisEnabled {
		return nil, nil
	}
	rdb := redis.NewClient(&redis.Options{
		Addr:     fmt.Sprintf("%s:%d", cfg.Redis.Host, cfg.Redis.Port),
		Password: cfg.Redis.Password,
		DB:       cfg.Redis.DB,
	})
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if err := rdb.Ping(ctx).Err(); err != nil {
		return nil, err
	}
	ttl := time.Duration(cfg.AnswerCache.TTLSeconds) * time.Second
	if ttl <= 0 {
		ttl = 30 * time.Minute
	}
	return &RedisL2{client: rdb, ttl: ttl}, nil
}

func (r *RedisL2) Get(ctx context.Context, key string) (*Entry, bool) {
	if r == nil {
		return nil, false
	}
	data, err := r.client.Get(ctx, redisPrefix+key).Bytes()
	if err != nil {
		return nil, false
	}
	var entry Entry
	if json.Unmarshal(data, &entry) != nil || entry.Answer == "" {
		return nil, false
	}
	return &entry, true
}

func (r *RedisL2) Set(ctx context.Context, key, kbType string, entry *Entry) error {
	if r == nil || entry == nil {
		return nil
	}
	data, err := json.Marshal(entry)
	if err != nil {
		return err
	}
	pipe := r.client.Pipeline()
	pipe.Set(ctx, redisPrefix+key, data, r.ttl)
	if kbType != "" {
		idx := redisKBIndexPrefix + kbType
		pipe.SAdd(ctx, idx, key)
		pipe.Expire(ctx, idx, r.ttl*2)
	}
	_, err = pipe.Exec(ctx)
	return err
}

func (r *RedisL2) Ping(ctx context.Context) error {
	if r == nil {
		return fmt.Errorf("answer redis cache disabled")
	}
	return r.client.Ping(ctx).Err()
}
