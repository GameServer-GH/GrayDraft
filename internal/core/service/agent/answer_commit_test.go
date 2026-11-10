package agent

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

type checkCandidateValidator struct {
	t      *testing.T
	sink   *sinkStub
	reject bool
	err    error
}

func (v checkCandidateValidator) Validate(_ context.Context, answer string, _ []Observation) (ValidationResult, error) {
	if answer != "模型候选答案" {
		v.t.Fatalf("gate did not see generated candidate: %q", answer)
	}
	// 乐观流式下候选答案的 answer.delta 允许先于门禁出现；但 run.completed
	// （权威提交）绝不能在门禁通过前发出。
	for _, ev := range v.sink.events {
		if ev.Type == "run.completed" {
			v.t.Fatalf("committed before validation: %s", ev.Type)
		}
	}
	return ValidationResult{Answer: "门禁确认后的回复", Rejected: v.reject, Gate: AnswerGateResult{RefuseReason: "no_evidence"}}, v.err
}

func TestRuntimeCommitsOnlyTheValidatedCandidate(t *testing.T) {
	for _, reject := range []bool{false, true} {
		sink := &sinkStub{}
		rt := NewRuntime(staticPlanner{Decision{Decision: DecisionFinal, Answer: "规划草稿"}}, nil, sink).
			WithAnswerStreamer(stubStreamer{deltas: []string{"模型候选答案"}}).
			WithAnswerValidator(checkCandidateValidator{t: t, sink: sink, reject: reject})
		out, err := rt.Run(context.Background(), Request{Prompt: []byte("问题"), Limits: Limits{MaxSteps: 1, Deadline: time.Now().Add(time.Second)}})
		if err != nil || out.State != StateCompleted || out.Answer != "门禁确认后的回复" {
			t.Fatalf("%+v %v", out, err)
		}
		var streamed strings.Builder
		completed := ""
		for _, ev := range sink.events {
			if ev.Type == "answer.delta" {
				streamed.WriteString(ev.Message)
			}
			if ev.Type == "run.completed" {
				completed = ev.Message
			}
		}
		// 逐 token 乐观流出的只有候选文本，且不得整体重放造成重复。
		if streamed.String() != "模型候选答案" {
			t.Fatalf("streamed=%q", streamed.String())
		}
		if completed != "门禁确认后的回复" {
			t.Fatalf("completed=%q", completed)
		}
	}
}

func TestRuntimeGenerationFailureHasNoAnswerOrCompletedEvent(t *testing.T) {
	for _, tc := range []struct {
		err   error
		state State
		event string
	}{
		{context.DeadlineExceeded, StateExpired, "run.expired"},
		{context.Canceled, StateCancelled, "run.cancelled"},
		{errors.New("truncated"), StateFailed, "run.failed"},
	} {
		sink := &sinkStub{}
		rt := NewRuntime(staticPlanner{Decision{Decision: DecisionFinal, Answer: "草稿"}}, nil, sink).WithAnswerStreamer(stubStreamer{deltas: []string{"半截"}, err: tc.err})
		out, err := rt.Run(context.Background(), Request{Prompt: []byte("问题"), Limits: Limits{MaxSteps: 1, Deadline: time.Now().Add(time.Second)}})
		if !errors.Is(err, tc.err) || out.State != tc.state || out.Answer != "" {
			t.Fatalf("%+v %v", out, err)
		}
		found := false
		leaked := ""
		for _, ev := range sink.events {
			if ev.Type == tc.event {
				found = true
			}
			if ev.Type == "answer.delta" {
				leaked += ev.Message
			}
			if ev.Type == "run.completed" {
				t.Fatalf("leaked partial: %+v", ev)
			}
		}
		// 乐观流式：半截候选已逐 delta 发出（由前端以错误提示收尾），
		// 但绝不能出现 run.completed 权威提交。
		if leaked != "半截" {
			t.Fatalf("streamed partial=%q", leaked)
		}
		if !found {
			t.Fatal(tc.event)
		}
	}
}

func TestRuntimeValidatorErrorDoesNotCommit(t *testing.T) {
	sink := &sinkStub{}
	rt := NewRuntime(staticPlanner{Decision{Decision: DecisionFinal}}, nil, sink).WithAnswerStreamer(stubStreamer{deltas: []string{"模型候选答案"}}).
		WithAnswerValidator(checkCandidateValidator{t: t, sink: sink, err: errors.New("validator unavailable")})
	out, err := rt.Run(context.Background(), Request{Prompt: []byte("问题"), Limits: Limits{MaxSteps: 1, Deadline: time.Now().Add(time.Second)}})
	if err == nil || out.State != StateFailed {
		t.Fatalf("%+v %v", out, err)
	}
	for _, ev := range sink.events {
		// 候选 answer.delta 允许（乐观流式），但不得有权威提交。
		if ev.Type == "run.completed" {
			t.Fatal(ev)
		}
	}
}
