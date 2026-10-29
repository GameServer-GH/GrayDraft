package middleware

import (
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/graydraft/core/config"
)

type dailyCounter struct {
	mu    sync.Mutex
	day   string
	count int
}

var chatBudget dailyCounter

// ChatBudget limits POST /api/v1/chat/stream per day per process (O-15 MVP).
func ChatBudget(cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		if cfg == nil || !cfg.Budget.Enabled || cfg.Budget.DailyChatLimit <= 0 {
			c.Next()
			return
		}
		if c.Request.Method != http.MethodPost || c.FullPath() != "/api/v1/chat/stream" {
			c.Next()
			return
		}
		today := time.Now().UTC().Format("2006-01-02")
		chatBudget.mu.Lock()
		if chatBudget.day != today {
			chatBudget.day = today
			chatBudget.count = 0
		}
		if chatBudget.count >= cfg.Budget.DailyChatLimit {
			chatBudget.mu.Unlock()
			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{
				"error":   "daily_chat_budget_exceeded",
				"limit":   cfg.Budget.DailyChatLimit,
				"message": "chat daily limit reached; try tomorrow or raise BUDGET_DAILY_CHAT_LIMIT",
			})
			return
		}
		chatBudget.count++
		chatBudget.mu.Unlock()
		c.Next()
	}
}
