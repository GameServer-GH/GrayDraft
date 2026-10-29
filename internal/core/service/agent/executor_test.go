package agent

import (
	"context"
	"encoding/json"
	"testing"
)

func TestRegistryExecutorValidatesBeforeCalling(t *testing.T) {
	called := false
	e := &RegistryExecutor{Registry: DefaultR0Registry(), Scopes: map[string]bool{"knowledge:read": true}, Funcs: map[string]ToolFunc{
		"knowledge.lexical": func(_ context.Context, _ json.RawMessage) (json.RawMessage, error) {
			called = true
			return json.RawMessage(`{"total":1}`), nil
		},
	}}
	obs, err := e.Execute(context.Background(), ToolCall{Name: "knowledge.lexical", Arguments: json.RawMessage(`{"query":"InternVL"}`)})
	if err != nil || !called || len(obs.Output) == 0 {
		t.Fatalf("obs=%+v err=%v", obs, err)
	}
	_, err = e.Execute(context.Background(), ToolCall{Name: "knowledge.lexical", Arguments: json.RawMessage(`{"query":"InternVL"}`)})
	if err != nil {
		t.Fatal(err)
	}
	e.Scopes = map[string]bool{}
	_, err = e.Execute(context.Background(), ToolCall{Name: "knowledge.lexical", Arguments: json.RawMessage(`{"query":"InternVL"}`)})
	if err == nil {
		t.Fatal("expected scope denial")
	}
}
