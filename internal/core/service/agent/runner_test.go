package agent

import (
	"context"
	"encoding/json"
	"testing"
	"time"
)

type runnerStoreStub struct {
	events []RunEvent
	status State
}

func (s *runnerStoreStub) AppendEvent(_ context.Context, ev RunEvent) error {
	s.events = append(s.events, ev)
	return nil
}
func (s *runnerStoreStub) UpdateStatus(_ context.Context, _ string, status State, _, _ int) error {
	s.status = status
	return nil
}

func TestExecuteRunPersistsRuntimeEvents(t *testing.T) {
	planner := staticPlanner{decision: Decision{Decision: DecisionFinal, Answer: "ok"}}
	store := &runnerStoreStub{}
	result, err := ExecuteRun(context.Background(), store, RunRecord{RunID: "r1", TenantID: "t1", Prompt: []byte("q")}, planner, nil, Limits{Deadline: time.Now().Add(time.Second)})
	if err != nil || result.State != StateCompleted || store.status != StateCompleted || len(store.events) < 2 {
		t.Fatalf("result=%+v status=%s events=%d err=%v", result, store.status, len(store.events), err)
	}
	if !json.Valid(store.events[0].Payload) {
		t.Fatal("event payload is not JSON")
	}
}
