package agent

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
)

func TestGovernedExecutorApprovalRequired(t *testing.T) {
	reg := NewToolRegistry(ToolSpec{
		Name: "publish", Version: "1.0.0", Risk: RiskR2, Scopes: []string{"note:write"},
		Required: []string{"title", "body"},
	})
	inner := &RegistryExecutor{
		Registry: reg,
		Scopes:   map[string]bool{"note:write": true},
		Funcs: map[string]ToolFunc{
			"publish": func(context.Context, json.RawMessage) (json.RawMessage, error) {
				return json.RawMessage(`{"status":"queued"}`), nil
			},
		},
	}
	g := GovernedExecutor{
		Registry: reg, Inner: inner, Scopes: inner.Scopes, RunID: "run-1", MaxOutput: 1024,
	}
	_, err := g.Execute(context.Background(), ToolCall{
		CallID: "c1", Name: "publish",
		Arguments: json.RawMessage(`{"title":"t","body":"b"}`),
	})
	if !errors.Is(err, ErrApprovalRequired) {
		t.Fatalf("expected ErrApprovalRequired, got %v", err)
	}
}
