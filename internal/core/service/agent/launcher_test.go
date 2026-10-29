package agent

import (
	"context"
	"testing"
	"time"
)

type notifyingStore struct{ done chan State }

func (s *notifyingStore) AppendEvent(context.Context, RunEvent) error { return nil }
func (s *notifyingStore) UpdateStatus(_ context.Context, _ string, status State, _, _ int) error {
	if status == StateCompleted {
		s.done <- status
	}
	return nil
}

func TestLauncherExecutesRun(t *testing.T) {
	store := &notifyingStore{done: make(chan State, 1)}
	l := &Launcher{Store: store, Planner: staticPlanner{decision: Decision{Decision: DecisionFinal, Answer: "ok"}}, PerTenant: 1}
	l.Launch(RunRecord{RunID: "r1", TenantID: "t1", Prompt: []byte("q"), Deadline: time.Now().Add(time.Second)}, Limits{})
	select {
	case <-store.done:
	case <-time.After(2 * time.Second):
		t.Fatal("launcher timeout")
	}
}
