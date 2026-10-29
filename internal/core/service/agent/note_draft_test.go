package agent

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
)

func TestEducationNoteDraftIsNotAnApprovalGate(t *testing.T) {
	spec, ok := DefaultEducationRegistry().Get("note.draft")
	if !ok {
		t.Fatal("note.draft missing")
	}
	if spec.Risk != RiskR1 {
		t.Fatalf("risk=%s, want R1", spec.Risk)
	}
	reg := DefaultEducationRegistry()
	reg.merge([]ToolSpec{{
		Name: "note.draft", Version: "9", Risk: RiskR2, Scopes: []string{"note:write"}, Required: []string{"title"},
	}})
	reg.ForceEducationNotePolicy()
	spec, _ = reg.Get("note.draft")
	if spec.Risk != RiskR1 {
		t.Fatalf("merged risk=%s, want R1", spec.Risk)
	}
	joined := strings.Join(spec.Required, ",")
	if !strings.Contains(joined, "citations") {
		t.Fatalf("required=%v", spec.Required)
	}
}

func TestNoteDraftRequiresCitations(t *testing.T) {
	tool := NoteDraftTool{
		Ingest: func(context.Context, string, string, string, string, string, []string) (string, error) {
			t.Fatal("ingest should not run")
			return "", nil
		},
	}
	_, err := tool.Call(context.Background(), json.RawMessage(`{"title":"参数","body":"结论"}`))
	if err == nil || !strings.Contains(err.Error(), "citations") {
		t.Fatalf("got %v", err)
	}
}
