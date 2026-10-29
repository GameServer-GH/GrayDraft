package middleware

import (
	"github.com/gin-gonic/gin"
	"github.com/graydraft/core/observability/trace"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
)

// OtelTrace creates a server span per HTTP request when OTel is enabled.
func OtelTrace(enabled bool) gin.HandlerFunc {
	return func(c *gin.Context) {
		if !enabled {
			c.Next()
			return
		}
		ctx, span := trace.StartSpan(c.Request.Context(), c.FullPath(),
			attribute.String("http.method", c.Request.Method),
			attribute.String("http.route", c.FullPath()),
		)
		defer span.End()
		c.Request = c.Request.WithContext(ctx)
		c.Next()
		status := c.Writer.Status()
		span.SetAttributes(attribute.Int("http.status_code", status))
		if status >= 500 {
			span.SetStatus(codes.Error, c.Errors.String())
		} else {
			span.SetStatus(codes.Ok, "")
		}
	}
}
