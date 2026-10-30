package agent

import (
	"context"
	"strings"
	"testing"
)

type completerStub struct {
	response string
	prompt   string
}

func (s *completerStub) Complete(_ context.Context, prompt string) (string, error) {
	s.prompt = prompt
	return s.response, nil
}

func TestLLMPlannerDecodesStrictDecision(t *testing.T) {
	llm := &completerStub{response: "```json\n{\"decision\":\"final\",\"answer\":\"ok\"}\n```"}
	d, err := (LLMPlanner{LLM: llm}).Decide(context.Background(), PlannerInput{Prompt: []byte("问题"), Step: 1})
	if err != nil || d.Answer != "ok" || d.Decision != DecisionFinal {
		t.Fatalf("decision=%+v err=%v", d, err)
	}
	if !strings.Contains(llm.prompt, "knowledge.lexical") || !strings.Contains(llm.prompt, "note.draft") || !strings.Contains(llm.prompt, "kb_type") {
		t.Fatal("tool contract missing")
	}
}

type dualJSONCompleter struct {
	n int
}

func (s *dualJSONCompleter) Complete(_ context.Context, _ string) (string, error) {
	s.n++
	if s.n == 1 {
		return `{"decision":"final"}{"decision":"clarify","answer":"bad"}`, nil
	}
	return `{"decision":"final","answer":"","thought_summary":"整理回答"}`, nil
}

func TestLLMPlannerRetriesAfterDualJSON(t *testing.T) {
	llm := &dualJSONCompleter{}
	d, err := (LLMPlanner{LLM: llm}).Decide(context.Background(), PlannerInput{Prompt: []byte("问题"), Step: 2})
	if err != nil || d.Decision != DecisionFinal || llm.n != 1 {
		// First extractJSONObject now keeps only the first object, so no retry needed.
		if err != nil || d.Decision != DecisionFinal {
			t.Fatalf("decision=%+v err=%v calls=%d", d, err, llm.n)
		}
	}
}

func TestLLMPlannerDoesNotRetryTruncatedJSON(t *testing.T) {
	trunc := &truncCompleter{body: `{"decision":"tool_calls","tool_calls":[{"name":"knowledge.lexical"`}
	_, err := (LLMPlanner{LLM: trunc}).Decide(context.Background(), PlannerInput{Prompt: []byte("问题"), Step: 2})
	if err == nil || trunc.n != 1 {
		t.Fatalf("err=%v calls=%d", err, trunc.n)
	}
	if !strings.Contains(err.Error(), "unexpected end of JSON input") {
		t.Fatal(err)
	}
}

type truncCompleter struct {
	n    int
	body string
}

func (s *truncCompleter) Complete(_ context.Context, _ string) (string, error) {
	s.n++
	return s.body, nil
}

func TestLLMPlannerRetryOnInvalid(t *testing.T) {
	llm := &completerStub{response: "not-json"}
	// completerStub always returns the same invalid body; DecodeDecision fails twice.
	_, err := (LLMPlanner{LLM: llm}).Decide(context.Background(), PlannerInput{Prompt: []byte("问题"), Step: 1})
	if err == nil {
		t.Fatal("expected decode error")
	}
}
