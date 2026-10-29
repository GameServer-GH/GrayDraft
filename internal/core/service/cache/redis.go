package cache

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"time"

	"github.com/graydraft/core/config"
	"github.com/graydraft/core/model"
	"github.com/redis/go-redis/v9"
)

const (
	redisPrefix           = "knex:sr:"
	redisIndexPrefix      = "knex:sr:idx:"
	redisEntityPoolPrefix = "knex:sr:entity_pool:"
)

// RedisL2 provides exact + semantic similarity cache in Redis.
type RedisL2 struct {
	client *redis.Client
	cfg    *config.Config
}

// NewRedisL2 connects to Redis. Returns nil if disabled.
func NewRedisL2(cfg *config.Config) (*RedisL2, error) {
	if !cfg.Cache.Enabled || !cfg.Cache.RedisEnabled {
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
	return &RedisL2{client: rdb, cfg: cfg}, nil
}

type cacheEntry struct {
	Response  *model.SearchResponse `json:"response"`
	Query     string                `json:"query"`
	Embedding []float32             `json:"embedding,omitempty"`
}

// GetExact loads an exact key hit.
func (r *RedisL2) GetExact(ctx context.Context, key string) (*model.SearchResponse, bool) {
	if r == nil {
		return nil, false
	}
	data, err := r.client.Get(ctx, redisPrefix+key).Bytes()
	if err != nil {
		return nil, false
	}
	var entry cacheEntry
	if json.Unmarshal(data, &entry) != nil || entry.Response == nil {
		return nil, false
	}
	return entry.Response, true
}

// GetSemantic finds a similar cached query by embedding cosine similarity.
func (r *RedisL2) GetSemantic(
	ctx context.Context,
	indexKey string,
	embedding []float32,
	threshold float64,
) (*model.SearchResponse, bool) {
	if r == nil || len(embedding) == 0 {
		return nil, false
	}
	keys, err := r.client.LRange(ctx, redisIndexPrefix+indexKey, 0, int64(r.cfg.Cache.SemanticScanLimit)-1).Result()
	if err != nil || len(keys) == 0 {
		return nil, false
	}

	bestScore := threshold
	var best *model.SearchResponse
	for _, k := range keys {
		data, err := r.client.Get(ctx, redisPrefix+k).Bytes()
		if err != nil {
			continue
		}
		var entry cacheEntry
		if json.Unmarshal(data, &entry) != nil || entry.Response == nil || len(entry.Embedding) == 0 {
			continue
		}
		sim := cosineSimilarity(embedding, entry.Embedding)
		if sim >= bestScore {
			bestScore = sim
			resp := *entry.Response
			resp.Strategy = "cache_l2_semantic"
			resp.CacheHit = "l2_semantic"
			best = &resp
		}
	}
	return best, best != nil
}

// Set stores response and indexes for semantic lookup.
func (r *RedisL2) Set(
	ctx context.Context,
	key, indexKey string,
	query string,
	embedding []float32,
	resp *model.SearchResponse,
) error {
	if r == nil || resp == nil {
		return nil
	}
	ttl := time.Duration(r.cfg.Cache.TTLSeconds) * time.Second
	entry := cacheEntry{Response: resp, Query: query, Embedding: embedding}
	data, err := json.Marshal(entry)
	if err != nil {
		return err
	}
	pipe := r.client.Pipeline()
	pipe.Set(ctx, redisPrefix+key, data, ttl)
	pipe.LRem(ctx, redisIndexPrefix+indexKey, 0, key)
	pipe.LPush(ctx, redisIndexPrefix+indexKey, key)
	pipe.LTrim(ctx, redisIndexPrefix+indexKey, 0, int64(r.cfg.Cache.SemanticIndexMax)-1)
	pipe.Expire(ctx, redisIndexPrefix+indexKey, ttl)
	_, err = pipe.Exec(ctx)
	return err
}

// Ping checks Redis connectivity.
func (r *RedisL2) Ping(ctx context.Context) error {
	if r == nil {
		return fmt.Errorf("redis cache disabled")
	}
	return r.client.Ping(ctx).Err()
}

func cosineSimilarity(a, b []float32) float64 {
	n := min(len(a), len(b))
	if n == 0 {
		return 0
	}
	var dot, na, nb float64
	for i := 0; i < n; i++ {
		dot += float64(a[i]) * float64(b[i])
		na += float64(a[i]) * float64(a[i])
		nb += float64(b[i]) * float64(b[i])
	}
	if na == 0 || nb == 0 {
		return 0
	}
	return dot / (math.Sqrt(na) * math.Sqrt(nb))
}
