package main

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/graydraft/core/service/agent"
)

func main() {
	runner := agent.DockerSandboxRunner{Image: os.Getenv("GRAYDRAFT_TOOL_RUNNER_IMAGE"), EgressNetwork: os.Getenv("GRAYDRAFT_TOOL_EGRESS_NETWORK")}
	token := os.Getenv("GRAYDRAFT_TOOL_RUNNER_TOKEN")
	mux := http.NewServeMux()
	mux.HandleFunc("/health", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) })
	mux.HandleFunc("/execute", func(w http.ResponseWriter, r *http.Request) {
		if token != "" && r.Header.Get("Authorization") != "Bearer "+token {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		var req struct {
			Spec    agent.ToolSpec    `json:"spec"`
			Call    agent.ToolCall    `json:"call"`
			Secrets map[string]string `json:"secrets"`
		}
		dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20))
		if err := dec.Decode(&req); err != nil {
			http.Error(w, "invalid request", http.StatusBadRequest)
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
		defer cancel()
		obs, err := runner.Run(ctx, req.Spec, req.Call, req.Secrets)
		result := struct {
			CallID string          `json:"call_id"`
			Name   string          `json:"name"`
			Output json.RawMessage `json:"output,omitempty"`
			Error  string          `json:"error,omitempty"`
		}{CallID: obs.CallID, Name: obs.Name, Output: obs.Output}
		if err != nil {
			result.Error = strings.TrimSpace(err.Error())
		}
		w.Header().Set("Content-Type", "application/json")
		if err != nil {
			w.WriteHeader(http.StatusUnprocessableEntity)
		}
		_ = json.NewEncoder(w).Encode(result)
	})
	server := &http.Server{Addr: ":18090", Handler: mux, ReadHeaderTimeout: 5 * time.Second, IdleTimeout: 30 * time.Second}
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		panic(err)
	}
}
