package agent

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"
)

type staticPlanner struct{ decision Decision }

func (p staticPlanner) Decide(context.Context, PlannerInput) (Decision, error) {
	return p.decision, nil
}

type plannerStub struct {
	decisions []Decision
	i         int
}

func (p *plannerStub) Decide(context.Context, PlannerInput) (Decision, error) {
	if p.i >= len(p.decisions) {
		return Decision{}, errors.New("unexpected decide")
	}
	d := p.decisions[p.i]
	p.i++
	return d, nil
}

type toolStub struct{ calls []ToolCall }

func (t *toolStub) Execute(_ context.Context, c ToolCall) (Observation, error) {
	t.calls = append(t.calls, c)
	return Observation{CallID: c.CallID, Name: c.Name, Output: json.RawMessage(`{"ok":true}`)}, nil
}

type sinkStub struct{ events []Event }

func (s *sinkStub) Append(_ context.Context, e Event) error {
	s.events = append(s.events, e)
	return nil
}

func TestRuntimeRunsToolThenFinal(t *testing.T) {
	p := &plannerStub{decisions: []Decision{{Decision: DecisionTools, ToolCalls: []ToolCall{{CallID: "1", Name: "knowledge.lexical", Arguments: json.RawMessage(`{"query":"InternVL"}`)}}}, {Decision: DecisionFinal, Answer: "当前版本是 1.2"}}}
	s := &sinkStub{}
	r := NewRuntime(p, &toolStub{}, s)
	got, err := r.Run(context.Background(), Request{Prompt: []byte("版本？"), Limits: Limits{MaxSteps: 3, Deadline: time.Now().Add(time.Second)}})
	if err != nil || got.State != StateCompleted || got.ToolCalls != 1 {
		t.Fatalf("result=%+v err=%v", got, err)
	}
	if len(s.events) < 4 || s.events[0].Type != "run.started" {
		t.Fatalf("events=%+v", s.events)
	}
}

func TestRuntimeStopsRepeatedTool(t *testing.T) {
	call := ToolCall{CallID: "1", Name: "knowledge.lexical", Arguments: json.RawMessage(`{"query":"InternVL"}`)}
	p := &plannerStub{decisions: []Decision{{Decision: DecisionTools, ToolCalls: []ToolCall{call}}, {Decision: DecisionTools, ToolCalls: []ToolCall{call}}}}
	r := NewRuntime(p, &toolStub{}, nil)
	_, err := r.Run(context.Background(), Request{Prompt: []byte("x"), Limits: Limits{MaxSteps: 4, MaxSameActionRepeat: 1, Deadline: time.Now().Add(time.Second)}})
	if err == nil {
		t.Fatal("expected repeated tool error")
	}
}

func TestDecodeDecision(t *testing.T) {
	if _, err := DecodeDecision([]byte(`{"decision":"tool_calls","tool_calls":[]}`)); err == nil {
		t.Fatal("expected empty call error")
	}
	if _, err := DecodeDecision([]byte(`{"decision":"final","answer":"ok"}`)); err != nil {
		t.Fatal(err)
	}
}

type rejectOnce struct{ n int }

func (v *rejectOnce) Validate(context.Context, string, []Observation) (ValidationResult, error) {
	v.n++
	if v.n == 1 {
		return ValidationResult{Rejected: true, Answer: "证据不足", Gate: AnswerGateResult{RefuseReason: "no_evidence"}}, nil
	}
	return ValidationResult{Answer: "InternVL 负责视觉理解。"}, nil
}

func TestRuntimeRetriesAfterEvidenceGap(t *testing.T) {
	p := &plannerStub{decisions: []Decision{
		{Decision: DecisionFinal, Answer: "先猜一个答案"},
		{Decision: DecisionTools, ToolCalls: []ToolCall{{CallID: "1", Name: "knowledge.lexical", Arguments: json.RawMessage(`{"query":"InternVL"}`)}}},
		{Decision: DecisionFinal, Answer: "InternVL 负责视觉理解。"},
	}}
	v := &rejectOnce{}
	r := NewRuntime(p, &toolStub{}, &sinkStub{}).WithAnswerValidator(v)
	got, err := r.Run(context.Background(), Request{
		Prompt: []byte("InternVL 负责什么"),
		Limits: Limits{MaxSteps: 4, Deadline: time.Now().Add(time.Second)},
	})
	if err != nil || got.State != StateCompleted || got.Answer != "InternVL 负责视觉理解。" || v.n != 2 {
		t.Fatalf("result=%+v validator=%d err=%v", got, v.n, err)
	}
	found := false
	for _, obs := range got.Observations {
		if obs.Name == "answer.gap" {
			found = true
		}
	}
	if !found {
		t.Fatal("expected answer.gap observation")
	}
}

type countingValidator struct{ calls int }

func (v *countingValidator) Validate(context.Context, string, []Observation) (ValidationResult, error) {
	v.calls++
	return ValidationResult{Answer: "ok"}, nil
}

func TestRuntimeRequiresPlanner(t *testing.T) {
	r := NewRuntime(nil, nil, &sinkStub{})
	_, err := r.Run(context.Background(), Request{
		Prompt:          []byte("q"),
		Limits:          Limits{MaxSteps: 4, Deadline: time.Now().Add(time.Second)},
		DisableThinking: true,
	})
	if err == nil || !strings.Contains(err.Error(), "planner") {
		t.Fatalf("expected planner error, got %v", err)
	}
}
