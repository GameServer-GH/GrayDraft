package agent

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// TraceSpan is one node in a reconstructed Run trace timeline.
type TraceSpan struct {
	SpanID       string    `json:"span_id"`
	ParentSpanID string    `json:"parent_span_id,omitempty"`
	Kind         string    `json:"kind"`
	Name         string    `json:"name,omitempty"`
	EventType    string    `json:"event_type"`
	Seq          int64     `json:"seq"`
	StepID       int       `json:"step_id,omitempty"`
	CallID       string    `json:"call_id,omitempty"`
	ToolName     string    `json:"tool_name,omitempty"`
	LatencyMS    int64     `json:"latency_ms,omitempty"`
	Status       string    `json:"status"`
	Error        string    `json:"error,omitempty"`
	Message      string    `json:"message,omitempty"`
	OccurredAt   time.Time `json:"occurred_at"`
}

// RunTrace aggregates all spans for one Agent Run.
type RunTrace struct {
	TraceID       string      `json:"trace_id"`
	RunID         string      `json:"run_id"`
	TenantID      string      `json:"tenant_id"`
	PolicyVersion string      `json:"policy_version,omitempty"`
	Spans         []TraceSpan `json:"spans"`
}

type eventPayload struct {
	TraceID      string `json:"trace_id"`
	SpanID       string `json:"span_id"`
	ParentSpanID string `json:"parent_span_id"`
	SpanKind     string `json:"span_kind"`
	SpanName     string `json:"span_name"`
	StepID       int    `json:"step_id"`
	CallID       string `json:"call_id"`
	ToolName     string `json:"tool_name"`
	LatencyMS    int64  `json:"latency_ms"`
	Status       string `json:"status"`
	Error        string `json:"error"`
	Message      string `json:"message"`
	State        State  `json:"state"`
}

// BuildRunTrace reconstructs a span tree from append-only Run events.
func BuildRunTrace(runID, tenantID, policyVersion string, events []RunEvent) RunTrace {
	out := RunTrace{RunID: runID, TenantID: tenantID, PolicyVersion: policyVersion}
	for _, ev := range events {
		var p eventPayload
		_ = json.Unmarshal(ev.Payload, &p)
		if out.TraceID == "" && p.TraceID != "" {
			out.TraceID = p.TraceID
		}
		kind := p.SpanKind
		name := p.SpanName
		if kind == "" {
			kind = eventSpanKind(ev.Type)
			name = ev.Type
		}
		status := p.Status
		if status == "" {
			status = spanStatusFromEvent(ev.Type, p.Error)
		}
		span := TraceSpan{
			SpanID:       p.SpanID,
			ParentSpanID: p.ParentSpanID,
			Kind:         kind,
			Name:         name,
			EventType:    ev.Type,
			Seq:          ev.Seq,
			StepID:       p.StepID,
			CallID:       firstNonEmpty(p.CallID, payloadCallID(ev.Payload)),
			ToolName:     firstNonEmpty(p.ToolName, payloadToolName(ev.Payload)),
			LatencyMS:    p.LatencyMS,
			Status:       status,
			Error:        firstNonEmpty(p.Error, errorFromMessage(ev.Type, p.Message)),
			Message:      p.Message,
			OccurredAt:   ev.OccurredAt,
		}
		if span.SpanID == "" {
			span.SpanID = syntheticSpanID(ev.RunID, ev.Seq)
		}
		out.Spans = append(out.Spans, span)
	}
	return out
}

func eventSpanKind(eventType string) string {
	switch {
	case strings.HasPrefix(eventType, "run."):
		return string(SpanKindRun)
	case eventType == "step.started":
		return string(SpanKindStep)
	case strings.HasPrefix(eventType, "tool."):
		return string(SpanKindTool)
	case eventType == "answer.rejected":
		return string(SpanKindRun)
	default:
		return "event"
	}
}

func spanStatusFromEvent(eventType, errText string) string {
	if errText != "" {
		return "error"
	}
	switch eventType {
	case "run.completed":
		return "ok"
	case "run.failed", "run.cancelled", "run.expired", "run.budget_exceeded",
		"run.tool_budget_exceeded", "run.no_progress", "run.invalid_decision", "answer.rejected":
		return "error"
	case "tool.completed":
		return "ok"
	default:
		return "ok"
	}
}

func errorFromMessage(eventType, message string) string {
	if message == "" {
		return ""
	}
	switch eventType {
	case "run.failed", "run.cancelled", "run.expired", "run.no_progress", "tool.completed":
		return message
	default:
		return ""
	}
}

func syntheticSpanID(runID string, seq int64) string {
	return fmt.Sprintf("%s:%d", runID, seq)
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}

func payloadCallID(raw []byte) string {
	var m map[string]any
	if json.Unmarshal(raw, &m) != nil {
		return ""
	}
	v, _ := m["call_id"].(string)
	return v
}

func payloadToolName(raw []byte) string {
	var m map[string]any
	if json.Unmarshal(raw, &m) != nil {
		return ""
	}
	v, _ := m["tool_name"].(string)
	return v
}
