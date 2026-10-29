package agent

import (
	"encoding/json"
	"testing"
	"time"
)

func TestResultToReplay(t *testing.T) {
	result := Result{
		State:  StateCompleted,
		Answer: "当前证据不足，无法可靠回答。",
		Observations: []Observation{{
			Name:   "knowledge.semantic",
			Output: json.RawMessage(`{"hits":[{"id":"c1","content":"x"}],"total":1}`),
		}},
	}
	replay := ResultToReplay("c1", result)
	if !replay.Refused {
		t.Fatal("expected refusal")
	}
	if replay.CitationCount != 1 {
		t.Fatalf("citations=%d", replay.CitationCount)
	}
}

func TestLauncherReplayExecutor(t *testing.T) {
	store := &runnerStoreStub{}
	planner := &plannerStub{decisions: []Decision{
		{Decision: DecisionTools, ToolCalls: []ToolCall{{CallID: "1", Name: "knowledge.lexical", Arguments: json.RawMessage(`{}`)}}},
		{Decision: DecisionFinal, Answer: "版本 1.2"},
	}}
	launcher := &Launcher{
		Store:     store,
		Planner:   planner,
		PerTenant: 1,
		Tools:     func(RunRecord) ToolExecutor { return &toolStub{} },
	}
	exec := NewLauncherReplayExecutor(LauncherReplayConfig{
		Launcher: launcher,
		Limits:   Limits{Deadline: time.Now().Add(5 * time.Second)},
	})
	report, err := ReplayDataset([]GoldenCase{{
		ID: "v", Query: "参数定义?", TenantID: "t1", ExpectedTools: []string{"knowledge.lexical"},
	}}, exec)
	if err != nil {
		t.Fatal(err)
	}
	if report.PassRate != 1 {
		t.Fatalf("report=%+v", report)
	}
	if report.Metrics.ToolSuccessRate != 1 {
		t.Fatalf("tool success=%v", report.Metrics.ToolSuccessRate)
	}
}
