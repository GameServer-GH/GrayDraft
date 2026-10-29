// Package trace propagates correlation IDs across HTTP, NATS, and logs.
package trace

import (
	"context"
	"log/slog"

	"github.com/google/uuid"
)

const (
	// HeaderTraceID is the primary cross-service correlation header (O-02).
	HeaderTraceID = "X-Trace-Id"
	// HeaderRequestID is kept in sync with trace id for legacy clients.
	HeaderRequestID = "X-Request-ID"
	// HeaderTaskID is set on NATS messages after UGC task creation.
	HeaderTaskID = "X-Task-Id"
)

type ctxKey struct{}

// WithContext stores trace id on context.
func WithContext(ctx context.Context, traceID string) context.Context {
	if traceID == "" {
		return ctx
	}
	return context.WithValue(ctx, ctxKey{}, traceID)
}

// FromContext returns trace id or empty string.
func FromContext(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	v, _ := ctx.Value(ctxKey{}).(string)
	return v
}

// NewID generates a new trace id.
func NewID() string {
	return uuid.New().String()
}

// Resolve picks incoming trace id or generates a new one.
func Resolve(incoming string) string {
	if incoming != "" {
		return incoming
	}
	return NewID()
}

// Logger returns slog with trace_id attribute when present.
func Logger(ctx context.Context) *slog.Logger {
	id := FromContext(ctx)
	if id == "" {
		return slog.Default()
	}
	return slog.Default().With("trace_id", id)
}

// LogAttrs returns slog attributes for trace_id.
func LogAttrs(ctx context.Context) []any {
	id := FromContext(ctx)
	if id == "" {
		return nil
	}
	return []any{"trace_id", id}
}
