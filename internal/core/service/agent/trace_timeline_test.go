package agent

import (
	"encoding/json"
	"testing"
	"time"
)

func TestBuildRunTraceFromEvents(t *testing.T) {
	events := []RunEvent{{
		RunID: "r1", TenantID: "t1", Seq: 1, Type: "run.started",
		Payload: []byte(`{"trace_id":"tr1","span_id":"s1","span_kind":"run","status":"ok"}`),
		OccurredAt: time.Now(),
	}, {
		RunID: "r1", TenantID: "t1", Seq: 2, Type: "tool.completed",
		Payload: []byte(`{"trace_id":"tr1","span_id":"s2","parent_span_id":"s1","span_kind":"tool","tool_name":"knowledge.semantic","latency_ms":42,"status":"ok"}`),
		OccurredAt: time.Now(),
	}}
	trace := BuildRunTrace("r1", "t1", "p1", events)
	if trace.TraceID != "tr1" || len(trace.Spans) != 2 {
		t.Fatalf("trace=%+v", trace)
	}
	if trace.Spans[1].LatencyMS != 42 || trace.Spans[1].ToolName != "knowledge.semantic" {
		t.Fatalf("tool span=%+v", trace.Spans[1])
	}
}

func TestBuildRunTraceSyntheticSpanID(t *testing.T) {
	events := []RunEvent{{
		RunID: "r1", TenantID: "t1", Seq: 3, Type: "step.started",
		Payload: json.RawMessage(`{}`), OccurredAt: time.Now(),
	}}
	trace := BuildRunTrace("r1", "t1", "", events)
	if trace.Spans[0].SpanID != "r1:3" {
		t.Fatalf("span id=%s", trace.Spans[0].SpanID)
	}
}
