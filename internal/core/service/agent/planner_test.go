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
	if err != nil || d.Answer != "ok" {
		t.Fatalf("decision=%+v err=%v", d, err)
	}
	if !strings.Contains(llm.prompt, "knowledge.lexical") || !strings.Contains(llm.prompt, "note.draft") || !strings.Contains(llm.prompt, "kb_type") {
		t.Fatal("tool contract missing")
	}
}
