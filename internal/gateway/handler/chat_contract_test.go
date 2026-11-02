package handler

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/graydraft/gateway/config"
	"github.com/graydraft/gateway/service/llm"
)

func TestGatewayPreservesCompletionAndFailure(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, tc := range []struct {
		name, upstream, want string
		wantDone             bool
	}{
		{"length", `data: {"choices":[{"delta":{"content":"半截"},"finish_reason":"length"}]}`, `"finish_reason":"length"`, true},
		{"stop", `data: {"choices":[{"delta":{"content":"完整"},"finish_reason":"stop"}]}`, `"finish_reason":"stop"`, true},
		{"eof", `data: {"choices":[{"delta":{"content":"半截"},"finish_reason":""}]}`, `"error":`, false},
		{"error", `data: {"error":{"message":"upstream unavailable"}}`, `"error":`, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { fmt.Fprintln(w, tc.upstream) }))
			defer server.Close()
			cfg := &config.Config{}
			cfg.Providers.DeepSeek = config.ProviderConfig{Enabled: true, APIKey: "test", BaseURL: server.URL}
			cfg.Models.LLMFast = config.ModelBinding{Provider: "deepseek", Model: "fast"}
			router := gin.New()
			router.POST("/v1/chat/completions", NewChatHandler(llm.New(cfg)).Completions)
			rr := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(`{"model":"fast","stream":true,"messages":[]}`))
			req.Header.Set("Content-Type", "application/json")
			router.ServeHTTP(rr, req)
			body := rr.Body.String()
			if !strings.Contains(body, tc.want) || strings.Contains(body, "data: [DONE]") != tc.wantDone {
				t.Fatalf("%s", body)
			}
		})
	}
}
