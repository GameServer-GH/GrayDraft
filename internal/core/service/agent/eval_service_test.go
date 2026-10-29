package agent

import (
	"context"
	"testing"
)

type evalStoreStub struct {
	saved EvalRunRecord
}

func (s *evalStoreStub) Save(_ context.Context, rec EvalRunRecord) (EvalRunRecord, error) {
	rec.EvalID = "eval-1"
	rec.CreatedAt = "2026-01-01T00:00:00Z"
	s.saved = rec
	return rec, nil
}

func (s *evalStoreStub) List(context.Context, string, int) ([]EvalRunRecord, error) {
	return nil, nil
}

func TestEvalServiceRunGolden(t *testing.T) {
	store := &evalStoreStub{}
	svc := &EvalService{Store: store, Executor: StructuralOracleExecutor(), DatasetRoot: "../../../../tests/agent-eval"}
	rec, err := svc.RunGolden(context.Background(), RunGoldenRequest{UseOracle: true})
	if err != nil {
		t.Fatal(err)
	}
	if rec.Total != 6 {
		t.Fatalf("expected 6 cases, got %d", rec.Total)
	}
	if rec.PassRate < 1 {
		t.Fatalf("oracle executor should pass all cases, pass_rate=%v", rec.PassRate)
	}
}

func TestStructuralOracleExecutorRefusal(t *testing.T) {
	r, err := StructuralOracleExecutor()(GoldenCase{ID: "x", MustRefuse: true})
	if err != nil || !r.Refused {
		t.Fatalf("expected refusal, got %+v err=%v", r, err)
	}
}
