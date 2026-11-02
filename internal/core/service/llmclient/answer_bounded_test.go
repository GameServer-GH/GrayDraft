package llmclient

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestBoundedAnswerUsesComplexModelAndOutputCap(t *testing.T) {
	var request chatRequest
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Error(err)
		}
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprint(w, "data: {\"choices\":[{\"delta\":{\"content\":\"中文概述。\"}}]}\n\ndata: [DONE]\n\n")
	}))
	defer server.Close()
	c := &Client{baseURL: server.URL, fastModel: "planner", complexModel: "answer", http: server.Client()}
	var answer string
	err := c.StreamAnswerBounded(context.Background(), "system", "evidence", 600, func(s string) error { answer += s; return nil })
	if err != nil || answer != "中文概述。" || request.Model != "answer" || request.MaxTokens != 600 || !request.DisableThinking {
		t.Fatalf("answer=%q request=%+v err=%v", answer, request, err)
	}
}

func TestOverviewTierDoesNotChangeOrdinaryAnswerModel(t *testing.T) {
	var models []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req chatRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Error(err)
		}
		models = append(models, req.Model)
		if req.MaxTokens != 600 || !req.DisableThinking {
			t.Errorf("invalid bounded request: %+v", req)
		}
		fmt.Fprint(w, "data: {\"choices\":[{\"delta\":{\"content\":\"中文概述。\"},\"finish_reason\":\"stop\"}]}\n\ndata: [DONE]\n")
	}))
	defer server.Close()
	c := &Client{baseURL: server.URL, fastModel: "fast-answer", complexModel: "complex-answer", overviewTier: "fast", http: server.Client()}
	for _, call := range []func() error{
		func() error {
			return c.StreamOverviewBounded(context.Background(), "system", "evidence", 600, func(string) error { return nil })
		},
		func() error {
			return c.StreamAnswerBounded(context.Background(), "system", "evidence", 600, func(string) error { return nil })
		},
	} {
		if err := call(); err != nil {
			t.Fatal(err)
		}
	}
	if len(models) != 2 || models[0] != "fast-answer" || models[1] != "complex-answer" {
		t.Fatal(models)
	}
}
