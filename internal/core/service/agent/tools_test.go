package agent

import (
	"encoding/json"
	"testing"
)

func TestToolRegistryValidate(t *testing.T) {
	r := NewToolRegistry(ToolSpec{Name: "knowledge.lexical", Version: "1.0.0", Risk: RiskR0, Scopes: []string{"knowledge:read"}, Required: []string{"query"}})
	call := ToolCall{Name: "knowledge.lexical", Arguments: json.RawMessage(`{"query":"InternVL"}`)}
	if _, err := r.Validate(call, map[string]bool{"knowledge:read": true}); err != nil {
		t.Fatal(err)
	}
	if _, err := r.Validate(call, map[string]bool{}); err == nil {
		t.Fatal("expected scope error")
	}
	call.Arguments = json.RawMessage(`{}`)
	if _, err := r.Validate(call, map[string]bool{"knowledge:read": true}); err == nil {
		t.Fatal("expected required field error")
	}
}

func TestToolRegistryBlocksR3(t *testing.T) {
	r := NewToolRegistry(ToolSpec{Name: "account.delete", Version: "1.0.0", Risk: RiskR3})
	if _, err := r.Validate(ToolCall{Name: "account.delete", Arguments: json.RawMessage(`{}`)}, nil); err == nil {
		t.Fatal("expected R3 block")
	}
}
