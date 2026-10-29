package llmclient

import (
	"strings"
	"testing"
)

func TestParseSSEReasoningContent(t *testing.T) {
	payload := strings.Join([]string{
		`data: {"choices":[{"delta":{"reasoning_content":"先"}}]}`,
		`data: {"choices":[{"delta":{"reasoning_content":"想"}}]}`,
		`data: {"choices":[{"delta":{"content":"答案"}}]}`,
		`data: [DONE]`,
		"",
	}, "\n")
	var content, reasoning strings.Builder
	_, _, err := parseSSE(strings.NewReader(payload), StreamHooks{
		OnContent:   func(s string) error { content.WriteString(s); return nil },
		OnReasoning: func(s string) error { reasoning.WriteString(s); return nil },
	})
	if err != nil {
		t.Fatal(err)
	}
	if reasoning.String() != "先想" {
		t.Fatalf("reasoning=%q", reasoning.String())
	}
	if content.String() != "答案" {
		t.Fatalf("content=%q", content.String())
	}
}

func TestParseSSEReasoningAlias(t *testing.T) {
	payload := "data: {\"choices\":[{\"delta\":{\"reasoning\":\"内部推理\"}}]}\ndata: [DONE]\n"
	var reasoning strings.Builder
	_, _, err := parseSSE(strings.NewReader(payload), StreamHooks{
		OnReasoning: func(s string) error { reasoning.WriteString(s); return nil },
	})
	if err != nil {
		t.Fatal(err)
	}
	if reasoning.String() != "内部推理" {
		t.Fatalf("reasoning=%q", reasoning.String())
	}
}
