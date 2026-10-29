package agent

import (
	"context"
	"encoding/json"
	"testing"
	"time"
)

type governedTool struct{}

func (governedTool) Execute(context.Context, ToolCall) (Observation, error) {
	return Observation{Output: json.RawMessage(`{"ok":true}`)}, nil
}
func TestGovernedExecutorRequiresR2Approval(t *testing.T) {
	r := NewToolRegistry(ToolSpec{Name: "publish", Version: "1", Risk: RiskR2})
	a := NewApprovalManager()
	g := GovernedExecutor{Registry: r, Inner: governedTool{}, Approvals: a, RunID: "r1"}
	if _, err := g.Execute(context.Background(), ToolCall{CallID: "c1", Name: "publish", Arguments: json.RawMessage(`{}`)}); err == nil {
		t.Fatal("expected approval")
	}
	a.Approve("r1", "c1", time.Minute)
	if _, err := g.Execute(context.Background(), ToolCall{CallID: "c1", Name: "publish", Arguments: json.RawMessage(`{}`)}); err != nil {
		t.Fatal(err)
	}
}

type auditCapture struct{ argumentsHash, outputHash string }

func (a *auditCapture) Record(_ context.Context, _, _, _ string, _ RiskLevel, argumentsHash, outputHash, _ string, _ time.Duration, _ error) {
	a.argumentsHash, a.outputHash = argumentsHash, outputHash
}

func TestGovernedExecutorRecordsHashes(t *testing.T) {
	r := NewToolRegistry(ToolSpec{Name: "read", Version: "1", Risk: RiskR0})
	a := &auditCapture{}
	call := ToolCall{CallID: "c1", Name: "read", Arguments: json.RawMessage(`{"q":"x"}`)}
	g := GovernedExecutor{Registry: r, Inner: governedTool{}, Audit: a}
	obs, err := g.Execute(context.Background(), call)
	if err != nil {
		t.Fatal(err)
	}
	if a.argumentsHash != ArgumentsHash(call) || a.outputHash != OutputHash(obs.Output) {
		t.Fatal("audit hashes were not recorded")
	}
}
