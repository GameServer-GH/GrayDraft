package answercache

import (
	"context"
)

const redisKBIndexPrefix = "knex:ans:idxkb:"

// StatsByKBType counts answer cache entries tracked for a kb_type.
func (r *RedisL2) StatsByKBType(ctx context.Context, kbType string) (int, error) {
	if r == nil || kbType == "" {
		return 0, nil
	}
	n, err := r.client.SCard(ctx, redisKBIndexPrefix+kbType).Result()
	if err != nil {
		return 0, err
	}
	return int(n), nil
}

// PurgeByKBType removes answer cache entries for a kb_type.
func (r *RedisL2) PurgeByKBType(ctx context.Context, kbType string) (int, error) {
	if r == nil || kbType == "" {
		return 0, nil
	}
	idx := redisKBIndexPrefix + kbType
	members, err := r.client.SMembers(ctx, idx).Result()
	if err != nil {
		return 0, err
	}
	removed := 0
	if len(members) > 0 {
		delKeys := make([]string, 0, len(members)+1)
		for _, m := range members {
			delKeys = append(delKeys, redisPrefix+m)
		}
		delKeys = append(delKeys, idx)
		n, err := r.client.Del(ctx, delKeys...).Result()
		if err != nil {
			return removed, err
		}
		removed = int(n)
	} else {
		n, err := r.client.Del(ctx, idx).Result()
		if err != nil {
			return 0, err
		}
		removed = int(n)
	}
	return removed, nil
}
