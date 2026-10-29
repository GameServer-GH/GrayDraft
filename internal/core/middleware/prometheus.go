package middleware

import (
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/graydraft/core/observability"
)

// Prometheus records HTTP request metrics for registered routes.
func Prometheus() gin.HandlerFunc {
	return func(c *gin.Context) {
		if !observability.Enabled() {
			c.Next()
			return
		}
		start := time.Now()
		c.Next()
		route := c.FullPath()
		if route == "" {
			route = c.Request.URL.Path
		}
		if route == "/metrics" {
			return
		}
		observability.RecordHTTP(
			c.Request.Method,
			route,
			strconv.Itoa(c.Writer.Status()),
			time.Since(start),
		)
	}
}
