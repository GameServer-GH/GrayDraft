package llmclient

import (
	"errors"
	"strings"
	"testing"
)

func TestParseSSECompletionContract(t *testing.T) {
	for _, tc := range []struct{ name, suffix, code string }{
		{"stop", `data:{"choices":[{"delta":{},"finish_reason":"stop"}]}`, ""},
		{"done", "data: [DONE]", ""},
		{"eof", "", "unexpected_eof"},
		{"length", `data: {"choices":[{"delta":{},"finish_reason":"length"}]}` + "\ndata: [DONE]", "incomplete_completion"},
		{"filter", `data: {"choices":[{"delta":{},"finish_reason":"content_filter"}]}`, "incomplete_completion"},
		{"upstream", `data: {"error":{"message":"upstream failed"}}`, "upstream_error"},
		{"broken_json", `data: {"choices":`, "invalid_json"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			payload := `data: {"choices":[{"delta":{"content":"半截"},"finish_reason":""}]}` + "\n" + tc.suffix + "\n"
			var answer, finish string
			_, _, err := parseSSE(strings.NewReader(payload), StreamHooks{OnContent: func(s string) error { answer += s; return nil }, OnFinish: func(s string) { finish = s }})
			if answer != "半截" {
				t.Fatal(answer)
			}
			if tc.code == "" {
				if err != nil {
					t.Fatal(err)
				}
				return
			}
			var se *StreamError
			if !errors.As(err, &se) || se.Code != tc.code {
				t.Fatalf("%T %v", err, err)
			}
			if tc.name == "length" && finish != "length" {
				t.Fatal(finish)
			}
		})
	}
}

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
