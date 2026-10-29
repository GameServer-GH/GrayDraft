// Package trace propagates correlation IDs from BFF to GrayDraft and logs.
package trace

import (
	"context"

	"github.com/google/uuid"
)

const (
	HeaderTraceID   = "X-Trace-Id"
	HeaderRequestID = "X-Request-ID"
	HeaderTaskID    = "X-Task-Id"
)

type ctxKey struct{}

func WithContext(ctx context.Context, traceID string) context.Context {
	if traceID == "" {
		return ctx
	}
	return context.WithValue(ctx, ctxKey{}, traceID)
}

func FromContext(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	v, _ := ctx.Value(ctxKey{}).(string)
	return v
}

func NewID() string {
	return uuid.New().String()
}

func Resolve(incoming string) string {
	if incoming != "" {
		return incoming
	}
	return NewID()
}
