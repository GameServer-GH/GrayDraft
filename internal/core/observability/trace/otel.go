// Package trace: OpenTelemetry integration (V1-35).
// Wraps the existing trace context with OTel span creation
// for Core -> Gateway -> Worker -> DB/Redis spans.
package trace

import (
	"context"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

const serviceName = "graydraft-core"

var tracer trace.Tracer

func init() {
	tracer = otel.Tracer(serviceName)
}

// StartSpan creates a child span from context and returns new ctx.
func StartSpan(ctx context.Context, name string, attrs ...attribute.KeyValue) (context.Context, trace.Span) {
	return tracer.Start(ctx, name, trace.WithAttributes(attrs...))
}

// SpanFromContext returns the active span (or noop).
func SpanFromContext(ctx context.Context) trace.Span {
	return trace.SpanFromContext(ctx)
}

// RecordSpanSuccess marks span ok and sets optional attrs.
func RecordSpanSuccess(span trace.Span, attrs ...attribute.KeyValue) {
	if span == nil {
		return
	}
	span.SetAttributes(attrs...)
	span.SetStatus(codes.Ok, "")
	span.End()
}

// RecordSpanError marks span as error.
func RecordSpanError(span trace.Span, err error, attrs ...attribute.KeyValue) {
	if span == nil {
		return
	}
	span.SetAttributes(attrs...)
	span.SetStatus(codes.Error, err.Error())
	span.End()
}

// SpanAttrs builds common attributes.
func SpanAttrs(kbType, workspace string) []attribute.KeyValue {
	var attrs []attribute.KeyValue
	if kbType != "" {
		attrs = append(attrs, attribute.String("kb_type", kbType))
	}
	if workspace != "" {
		attrs = append(attrs, attribute.String("workspace", workspace))
	}
	return attrs
}
