package middleware

import (
	"github.com/gin-gonic/gin"
	"github.com/graydraft/core/observability/trace"
)

// RequestID injects trace/request id into context, response headers, and Gin context.
func RequestID() gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.GetHeader(trace.HeaderTraceID)
		if id == "" {
			id = c.GetHeader(trace.HeaderRequestID)
		}
		id = trace.Resolve(id)

		ctx := trace.WithContext(c.Request.Context(), id)
		c.Request = c.Request.WithContext(ctx)
		c.Set(trace.HeaderTraceID, id)
		c.Writer.Header().Set(trace.HeaderTraceID, id)
		c.Writer.Header().Set(trace.HeaderRequestID, id)
		c.Next()
	}
}
