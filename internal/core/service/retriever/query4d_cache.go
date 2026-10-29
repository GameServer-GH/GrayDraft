package retriever

import (
	"context"
	"strings"
	"sync"
	"time"

	"github.com/graydraft/core/config"
	"github.com/graydraft/core/model"
	"github.com/graydraft/core/service/llmclient"
)

const query4DCacheMaxEntries = 512

type query4DCacheEntry struct {
	value     model.QuerySemantic4D
	expiresAt time.Time
}

var (
	query4DCacheMu sync.RWMutex
	query4DCache   = map[string]query4DCacheEntry{}
)

func query4DCacheKey(query string, cfg *config.Config, taxonomyCacheSeg string) string {
	norm := strings.ToLower(strings.TrimSpace(query))
	llmOn := cfg != nil && cfg.Retrieval.Semantic4DLLMEnabled
	seg := strings.TrimSpace(taxonomyCacheSeg)
	if seg == "" {
		seg = "_"
	}
	return norm + "|llm:" + boolString(llmOn) + "|tax:" + seg
}

func boolString(b bool) string {
	if b {
		return "1"
	}
	return "0"
}

func getQuery4DFromCache(key string) (model.QuerySemantic4D, bool) {
	query4DCacheMu.RLock()
	defer query4DCacheMu.RUnlock()
	e, ok := query4DCache[key]
	if !ok || time.Now().After(e.expiresAt) {
		return model.QuerySemantic4D{}, false
	}
	return e.value, true
}

func setQuery4DCache(key string, v model.QuerySemantic4D, ttl time.Duration) {
	query4DCacheMu.Lock()
	defer query4DCacheMu.Unlock()
	if len(query4DCache) >= query4DCacheMaxEntries {
		now := time.Now()
		for k, e := range query4DCache {
			if now.After(e.expiresAt) {
				delete(query4DCache, k)
			}
		}
		if len(query4DCache) >= query4DCacheMaxEntries {
			for k := range query4DCache {
				delete(query4DCache, k)
				break
			}
		}
	}
	query4DCache[key] = query4DCacheEntry{value: v, expiresAt: time.Now().Add(ttl)}
}

// ExtractQuery4DCached wraps ExtractQuery4D with a short-lived in-process cache (per normalized query + taxonomy).
func ExtractQuery4DCached(ctx context.Context, query string, cfg *config.Config, llm *llmclient.Client, taxonomyCacheSeg string, tax *model.DomainTaxonomy) model.QuerySemantic4D {
	key := query4DCacheKey(query, cfg, taxonomyCacheSeg)
	if v, ok := getQuery4DFromCache(key); ok {
		if v.Method != "" && !strings.Contains(v.Method, "cache") {
			v.Method = v.Method + "+cache"
		} else if v.Method == "" {
			v.Method = "cache"
		}
		return v
	}
	out := ExtractQuery4D(ctx, query, cfg, llm, tax)
	ttl := 10 * time.Minute
	if cfg != nil && cfg.Cache.TTLSeconds > 0 {
		ttl = time.Duration(cfg.Cache.TTLSeconds) * time.Second
		if ttl > 30*time.Minute {
			ttl = 30 * time.Minute
		}
	}
	setQuery4DCache(key, out, ttl)
	return out
}
