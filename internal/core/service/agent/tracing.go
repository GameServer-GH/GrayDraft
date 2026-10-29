package agent

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"time"

	"github.com/graydraft/core/observability/trace"
	"go.opentelemetry.io/otel/attribute"
	oteltrace "go.opentelemetry.io/otel/trace"
)

// SpanKind classifies agent runtime spans for trace timelines.
type SpanKind string

const (
	SpanKindRun       SpanKind = "run"
	SpanKindStep      SpanKind = "step"
	SpanKindTool      SpanKind = "tool"
	SpanKindLLM       SpanKind = "llm"
	SpanKindRetrieval SpanKind = "retrieval"
)

type traceCtxKey struct{}

// TraceContext binds correlation identifiers to an Agent Run execution tree.
type TraceContext struct {
	TraceID       string
	RunID         string
	TenantID      string
	PolicyVersion string
	activeSpan    string
}

// WithTraceContext stores agent trace metadata on ctx and propagates trace_id.
func WithTraceContext(ctx context.Context, tc TraceContext) context.Context {
	if tc.TraceID == "" {
		tc.TraceID = trace.FromContext(ctx)
	}
	if tc.TraceID == "" {
		tc.TraceID = trace.NewID()
	}
	ctx = trace.WithContext(ctx, tc.TraceID)
	return context.WithValue(ctx, traceCtxKey{}, tc)
}

// TraceFromContext returns the agent trace context when present.
func TraceFromContext(ctx context.Context) (TraceContext, bool) {
	v, ok := ctx.Value(traceCtxKey{}).(TraceContext)
	return v, ok
}

func newSpanID() string {
	var b [8]byte
	_, _ = rand.Read(b[:])
	return hex.EncodeToString(b[:])
}

// SpanHandle tracks one logical span in the Agent Run tree.
type SpanHandle struct {
	TraceID      string
	SpanID       string
	ParentSpanID string
	Name         string
	Kind         SpanKind
	StepID       int
	CallID       string
	ToolName     string
	started      time.Time
	otelSpan     oteltrace.Span
}

// StartAgentSpan opens an OTel child span and records parent/child linkage for events.
func StartAgentSpan(ctx context.Context, name string, kind SpanKind, stepID int, callID, toolName string) (context.Context, *SpanHandle) {
	tc, _ := TraceFromContext(ctx)
	parent := tc.activeSpan
	spanID := newSpanID()

	attrs := []attribute.KeyValue{
		attribute.String("trace_id", tc.TraceID),
		attribute.String("run_id", tc.RunID),
		attribute.String("tenant_id", tc.TenantID),
		attribute.String("span.kind", string(kind)),
	}
	if tc.PolicyVersion != "" {
		attrs = append(attrs, attribute.String("policy_version", tc.PolicyVersion))
	}
	if stepID > 0 {
		attrs = append(attrs, attribute.Int("step_id", stepID))
	}
	if callID != "" {
		attrs = append(attrs, attribute.String("tool_call_id", callID))
	}
	if toolName != "" {
		attrs = append(attrs, attribute.String("tool.name", toolName))
	}

	ctx, otelSpan := trace.StartSpan(ctx, "agent."+string(kind)+"."+name, attrs...)
	tc.activeSpan = spanID
	ctx = context.WithValue(ctx, traceCtxKey{}, tc)

	return ctx, &SpanHandle{
		TraceID: tc.TraceID, SpanID: spanID, ParentSpanID: parent,
		Name: name, Kind: kind, StepID: stepID, CallID: callID, ToolName: toolName,
		started: time.Now(), otelSpan: otelSpan,
	}
}

// End closes the span and returns latency, status and optional error text.
func (h *SpanHandle) End(err error) (latencyMS int64, status string, errMsg string) {
	if h == nil {
		return 0, "ok", ""
	}
	latencyMS = time.Since(h.started).Milliseconds()
	status = "ok"
	if err != nil {
		status = "error"
		errMsg = err.Error()
		trace.RecordSpanError(h.otelSpan, err, attribute.Int64("latency_ms", latencyMS))
		return latencyMS, status, errMsg
	}
	trace.RecordSpanSuccess(h.otelSpan, attribute.Int64("latency_ms", latencyMS))
	return latencyMS, status, errMsg
}

// StampEvent copies open span metadata onto a runtime Event.
func (h *SpanHandle) StampEvent(ev *Event) {
	if h == nil || ev == nil {
		return
	}
	ev.TraceID = h.TraceID
	ev.SpanID = h.SpanID
	ev.ParentSpanID = h.ParentSpanID
	ev.SpanKind = string(h.Kind)
	ev.SpanName = h.Name
	ev.StepID = h.StepID
}

// FinishEvent closes the span and records latency/status on the event.
func (h *SpanHandle) FinishEvent(ev *Event, err error) {
	h.StampEvent(ev)
	if h == nil || ev == nil {
		return
	}
	ev.LatencyMS, ev.SpanStatus, ev.SpanError = h.End(err)
}

// ApplyEvent is a shortcut for single-shot spans (step/run) that open and close together.
func (h *SpanHandle) ApplyEvent(ev *Event, err error) {
	h.FinishEvent(ev, err)
}
