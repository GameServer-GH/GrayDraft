package agent

import (
	"testing"
)

func TestExtractJSONObjectFirstOnly(t *testing.T) {
	raw := `{"decision":"final","answer":""}{"decision":"clarify"}`
	got := string(extractJSONObject(raw))
	if got != `{"decision":"final","answer":""}` {
		t.Fatalf("got %q", got)
	}
}

func TestExtractJSONObjectIgnoresBracesInStrings(t *testing.T) {
	raw := `{"decision":"final","answer":"用 {x} 表示"} trailing`
	got := string(extractJSONObject(raw))
	want := `{"decision":"final","answer":"用 {x} 表示"}`
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
	if _, err := DecodeDecision([]byte(got)); err != nil {
		t.Fatal(err)
	}
}

func TestDecodeDecisionAliases(t *testing.T) {
	d, err := DecodeDecision([]byte(`{"decision":"answer","answer":"忽略正文"}`))
	if err != nil || d.Decision != DecisionFinal {
		t.Fatalf("decision=%+v err=%v", d, err)
	}
	d, err = DecodeDecision([]byte(`{"decision":"ready_to_answer"}`))
	if err != nil || d.Decision != DecisionFinal {
		t.Fatalf("decision=%+v err=%v", d, err)
	}
	d, err = DecodeDecision([]byte(`{"decision":"tools","tool_calls":[{"call_id":"1","name":"knowledge.lexical","arguments":{}}]}`))
	if err != nil || d.Decision != DecisionTools || len(d.ToolCalls) != 1 {
		t.Fatalf("decision=%+v err=%v", d, err)
	}
}
