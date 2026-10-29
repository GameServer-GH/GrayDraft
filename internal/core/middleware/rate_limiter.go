// Package middleware: per-workspace rate limiter (V1-14).
// Process-local sliding-window rate limiter for ingest and chat.
// Multi-instance deployment requires Redis backend — this is the MVP.
package middleware

import (
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/graydraft/core/service/dynconfig"
)

type wsBucket struct {
	mu      sync.Mutex
	window  int64 // unix second
	tokens  int
}

// RateLimiter provides per-workspace QPS limiting.
type RateLimiter struct {
	dynCfg *dynconfig.Manager
	buckets sync.Map // workspace -> *wsBucket
}

// NewRateLimiter creates a process-local rate limiter.
func NewRateLimiter(dynCfg *dynconfig.Manager) *RateLimiter {
	return &RateLimiter{dynCfg: dynCfg}
}

func workspaceFromRequest(c *gin.Context) string {
	if ws := c.GetHeader("X-Workspace"); ws != "" {
		return ws
	}
	kb := c.Query("kb_type")
	if kb == "" {
		var req struct{ KBType string `json:"kb_type"` }
		_ = c.ShouldBindJSON(&req)
		if c.Request.Body != nil {
			kb = req.KBType
		}
	}
	if kb != "" {
		return kb
	}
	return "_default"
}

// IngestRateLimit returns middleware for ingest endpoints.
func (rl *RateLimiter) IngestRateLimit() gin.HandlerFunc {
	return func(c *gin.Context) {
		if rl == nil || rl.dynCfg == nil {
			c.Next()
			return
		}
		hc := rl.dynCfg.Hot()
		if hc.RateLimitEnabled == 0 {
			c.Next()
			return
		}
		qps := int(hc.IngestQPS)
		if qps <= 0 {
			c.Next()
			return
		}
		ws := workspaceFromRequest(c)
		if rl.tryAcquire(ws, qps) {
			c.Next()
			return
		}
		c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{
			"error": "ingest_rate_limit_exceeded",
			"workspace": ws,
			"limit": qps,
			"retry_after": "1s",
		})
	}
}

func (rl *RateLimiter) tryAcquire(ws string, qps int) bool {
	now := time.Now().Unix()
	val, _ := rl.buckets.LoadOrStore(ws, &wsBucket{window: now, tokens: qps})
	b := val.(*wsBucket)
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.window != now {
		b.window = now
		b.tokens = qps
	}
	if b.tokens > 0 {
		b.tokens--
		return true
	}
	return false
}

// Remaining returns tokens left for the given workspace in current window.
func (rl *RateLimiter) Remaining(ws string) int {
	val, ok := rl.buckets.Load(ws)
	if !ok {
		return int(rl.dynCfg.Hot().IngestQPS)
	}
	b := val.(*wsBucket)
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.tokens
}

func itoa(i int) string { return strconv.Itoa(i) }
