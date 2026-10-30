package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/graydraft/core/observability/trace"
	"github.com/graydraft/core/service/cost"
)

type RunStore interface {
	AppendEvent(context.Context, RunEvent) error
	UpdateStatus(context.Context, string, State, int, int) error
}

type persistedSink struct {
	store RunStore
	run   RunRecord
}

func (s persistedSink) Append(ctx context.Context, ev Event) error {
	payload := map[string]any{
		"message": ev.Message,
		"state":   ev.State,
	}
	if ev.CallID != "" {
		payload["call_id"] = ev.CallID
	}
	if ev.ToolName != "" {
		payload["tool_name"] = ev.ToolName
	}
	if ev.TraceID != "" {
		payload["trace_id"] = ev.TraceID
	}
	if ev.SpanID != "" {
		payload["span_id"] = ev.SpanID
	}
	if ev.ParentSpanID != "" {
		payload["parent_span_id"] = ev.ParentSpanID
	}
	if ev.SpanKind != "" {
		payload["span_kind"] = ev.SpanKind
	}
	if ev.SpanName != "" {
		payload["span_name"] = ev.SpanName
	}
	if ev.StepID > 0 {
		payload["step_id"] = ev.StepID
	}
	if ev.LatencyMS > 0 {
		payload["latency_ms"] = ev.LatencyMS
	}
	if ev.SpanStatus != "" {
		payload["status"] = ev.SpanStatus
	}
	if ev.SpanError != "" {
		payload["error"] = ev.SpanError
	}
	raw, _ := json.Marshal(payload)
	return s.store.AppendEvent(ctx, RunEvent{
		RunID:      s.run.RunID,
		TenantID:   s.run.TenantID,
		Seq:        ev.Seq,
		Type:       ev.Type,
		Payload:    raw,
		OccurredAt: ev.At,
	})
}

// ExecuteRun is the worker boundary. It uses the same Runtime as tests and
// HTTP, so retries and replay do not create a second implementation of Loop.
func ExecuteRun(ctx context.Context, store RunStore, run RunRecord, planner Planner, tools ToolExecutor, limits Limits) (Result, error) {
	return ExecuteRunWith(ctx, store, run, planner, tools, limits, nil, GroundedAnswerValidator{RequiredCitations: 1})
}

// ExecuteRunOptions wires optional streaming and validation into the worker boundary.
type ExecuteRunOptions struct {
	Streamer  TokenStreamer
	Validator AnswerValidator
}

// ExecuteRunWith is the full worker entry used by Launcher.
// Optional seed observations (e.g. long-term memory) are injected before the first decide.
func ExecuteRunWith(ctx context.Context, store RunStore, run RunRecord, planner Planner, tools ToolExecutor, limits Limits, streamer TokenStreamer, validator AnswerValidator, seed ...Observation) (Result, error) {
	started := time.Now()
	if store == nil {
		return Result{State: StateFailed}, fmt.Errorf("agent: run store is required")
	}
	traceID := run.TraceID
	if traceID == "" {
		traceID = trace.FromContext(ctx)
	}
	ctx = WithTraceContext(ctx, TraceContext{
		TraceID:       traceID,
		RunID:         run.RunID,
		TenantID:      run.TenantID,
		PolicyVersion: run.PolicyVersion,
	})
	ctx = cost.WithScope(ctx, cost.Scope{
		TenantID: run.TenantID,
		AgentID:  run.AgentID,
		RunID:    run.RunID,
	})
	_ = store.UpdateStatus(ctx, run.RunID, StateDeciding, 0, 0)
	if validator == nil {
		validator = GroundedAnswerValidator{RequiredCitations: 1}
	}
	rt := NewRuntime(planner, tools, persistedSink{store: store, run: run}).
		WithAnswerValidator(validator).
		WithAnswerStreamer(streamer)
	result, err := rt.Run(ctx, Request{
		Prompt:           run.Prompt,
		Limits:           limits,
		AuthorizedKBs:    run.AuthorizedKBs,
		DisableThinking:  run.DisableThinking,
		SeedObservations: seed,
	})
	status := string(result.State)
	if err != nil && status == "" {
		status = "failed"
	}
	RecordRunMetric(status, time.Since(started), result.Steps, result.ToolCalls)
	_ = store.UpdateStatus(ctx, run.RunID, result.State, result.Steps, result.ToolCalls)
	return result, err
}
