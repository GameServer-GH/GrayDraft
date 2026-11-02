package llm

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/graydraft/gateway/config"
)

func TestFallbackDoesNotSplicePartialCompletions(t *testing.T) {
	for _, partial := range []bool{false, true} {
		t.Run(fmt.Sprint(partial), func(t *testing.T) {
			var models []string
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				var req ChatRequest
				if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
					t.Error(err)
					return
				}
				models = append(models, req.Model)
				if req.Model == "pro" {
					if partial {
						fmt.Fprint(w, "data: {\"choices\":[{\"delta\":{\"content\":\"半截\"}}]}\n\n")
					} else {
						w.WriteHeader(503)
					}
					return
				}
				fmt.Fprint(w, "data: {\"choices\":[{\"delta\":{\"content\":\"完整\"},\"finish_reason\":\"stop\"}]}\n\n")
			}))
			defer server.Close()
			cfg := &config.Config{}
			cfg.Providers.DeepSeek = config.ProviderConfig{Enabled: true, APIKey: "test", BaseURL: server.URL}
			cfg.Models.LLM = config.ModelBinding{Provider: "deepseek", Model: "pro"}
			cfg.Models.LLMFast = config.ModelBinding{Provider: "deepseek", Model: "fast"}
			cfg.LLM.FallbackLocal = true
			var answer, finish string
			done := false
			err := New(cfg).StreamCompletion(context.Background(), ChatRequest{Model: "complex", Stream: true}, func(d StreamDelta) error { answer += d.Content; done = d.Done; finish = d.FinishReason; return nil })
			if partial {
				if err == nil || len(models) != 1 || answer != "半截" || done {
					t.Fatalf("err=%v models=%v answer=%q done=%v", err, models, answer, done)
				}
			} else if err != nil || len(models) != 2 || answer != "完整" || !done || finish != "stop" {
				t.Fatalf("err=%v models=%v answer=%q done=%v finish=%q", err, models, answer, done, finish)
			}
		})
	}
}
