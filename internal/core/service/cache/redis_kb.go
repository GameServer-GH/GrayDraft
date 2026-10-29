package cache

import (
	"context"
	"strings"
)

// StatsByKBType counts retrieval cache entries indexed for a kb_type.
func (r *RedisL2) StatsByKBType(ctx context.Context, kbType string) (int, error) {
	if r == nil || kbType == "" {
		return 0, nil
	}
	seen := make(map[string]struct{})
	pat := redisIndexPrefix + kbType + ":*"
	var cursor uint64
	for {
		keys, next, err := r.client.Scan(ctx, cursor, pat, 128).Result()
		if err != nil {
			return 0, err
		}
		for _, idxKey := range keys {
			members, err := r.client.LRange(ctx, idxKey, 0, -1).Result()
			if err != nil {
				continue
			}
			for _, m := range members {
				seen[m] = struct{}{}
			}
		}
		cursor = next
		if cursor == 0 {
			break
		}
	}
	return len(seen), nil
}

// PurgeByKBType removes retrieval cache keys for a kb_type.
func (r *RedisL2) PurgeByKBType(ctx context.Context, kbType string) (int, error) {
	if r == nil || kbType == "" {
		return 0, nil
	}
	pat := redisIndexPrefix + kbType + ":*"
	var cursor uint64
	removed := 0
	for {
		keys, next, err := r.client.Scan(ctx, cursor, pat, 64).Result()
		if err != nil {
			return removed, err
		}
		for _, idxKey := range keys {
			members, err := r.client.LRange(ctx, idxKey, 0, -1).Result()
			if err != nil {
				continue
			}
			if len(members) > 0 {
				delKeys := make([]string, 0, len(members)+1)
				for _, m := range members {
					delKeys = append(delKeys, redisPrefix+m)
				}
				delKeys = append(delKeys, idxKey)
				if n, err := r.client.Del(ctx, delKeys...).Result(); err == nil {
					removed += int(n)
				}
			} else {
				if n, err := r.client.Del(ctx, idxKey).Result(); err == nil {
					removed += int(n)
				}
			}
		}
		cursor = next
		if cursor == 0 {
			break
		}
	}
	return removed, nil
}

// FineGrainedPurge removes retrieval cache entries that contain any of the given entities or title terms.
// Falls back to full kb_type purge if the fine-grained lists are empty.
func (r *RedisL2) FineGrainedPurge(ctx context.Context, kbType string, entities []string, titleTerms []string) (int, error) {
	if r == nil || kbType == "" {
		return 0, nil
	}
	// If no fine-grained keys provided, fall back to full purge.
	if len(entities) == 0 && len(titleTerms) == 0 {
		return r.PurgeByKBType(ctx, kbType)
	}

	// Build a set of terms to match against cached query text in index entries.
	matchTerms := make(map[string]bool)
	for _, e := range entities {
		e = strings.ToLower(strings.TrimSpace(e))
		if e != "" {
			matchTerms[e] = true
		}
	}
	for _, t := range titleTerms {
		t = strings.ToLower(strings.TrimSpace(t))
		if t != "" {
			matchTerms[t] = true
		}
	}
	if len(matchTerms) == 0 {
		return r.PurgeByKBType(ctx, kbType)
	}

	pat := redisIndexPrefix + kbType + ":*"
	var cursor uint64
	removed := 0
	for {
		keys, next, err := r.client.Scan(ctx, cursor, pat, 64).Result()
		if err != nil {
			return removed, err
		}
		for _, idxKey := range keys {
			// Check if the index key itself contains any match term.
			// Index keys are: knex:sr:idx:{kbType}:{catalogVersion}:{timestamp}
			// The query text is embedded in the entry keys, not index keys.
			// For now, do a full scan of members; a more optimized approach would
			// use per-entity secondary indexes.
			members, err := r.client.LRange(ctx, idxKey, 0, -1).Result()
			if err != nil {
				continue
			}
			var delKeys []string
			for _, m := range members {
				low := strings.ToLower(m)
				matched := false
				for term := range matchTerms {
					if strings.Contains(low, term) {
						matched = true
						break
					}
				}
				if matched {
					delKeys = append(delKeys, redisPrefix+m)
				}
			}
			if len(delKeys) > 0 {
				n, err := r.client.Del(ctx, delKeys...).Result()
				if err == nil {
					removed += int(n)
				}
				// Also remove these members from the index.
				for _, dk := range delKeys {
					memberKey := strings.TrimPrefix(dk, redisPrefix)
					_ = r.client.LRem(ctx, idxKey, 0, memberKey).Err()
				}
			}
		}
		cursor = next
		if cursor == 0 {
			break
		}
	}
	return removed, nil
}
