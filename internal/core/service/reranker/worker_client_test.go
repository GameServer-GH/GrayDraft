package reranker

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/graydraft/core/config"
	"github.com/graydraft/core/model"
)

func TestClient_Rerank_L1L2(t *testing.T) {
	var paths []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		paths = append(paths, r.URL.Path)
		// This test exercises sequential fallback, not the preferred combined route.
		if r.URL.Path == "/v1/rerank/combined" {
			http.NotFound(w, r)
			return
		}
		var req rerankRequest
		_ = json.NewDecoder(r.Body).Decode(&req)
		top := req.TopN
		if top == 0 {
			top = len(req.Candidates)
		}
		results := make([]scoredCandidate, 0, top)
		for i := 0; i < top && i < len(req.Candidates); i++ {
			results = append(results, scoredCandidate{ID: req.Candidates[i].ID, Score: 1.0 - float64(i)*0.1})
		}
		_ = json.NewEncoder(w).Encode(rerankResponse{Results: results, Model: "test"})
	}))
	defer srv.Close()

	cfg := &config.Config{}
	cfg.AIWorker.URL = srv.URL
	cfg.AIWorker.TimeoutSeconds = 5
	cfg.Retrieval.RerankL1TopN = 2
	cfg.Retrieval.RerankL2TopN = 1

	c := NewClient(cfg)
	chunks := []model.Chunk{
		{ID: "a", Content: "混合检索 A"},
		{ID: "b", Content: "混合检索 B"},
		{ID: "c", Content: "无关"},
	}
	out, err := c.Rerank(context.Background(), "混合检索", chunks, 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(out) != 1 {
		t.Fatalf("expected 1 chunk, got %d", len(out))
	}
	if out[0].Source != "rerank_l2" {
		t.Fatalf("expected rerank_l2 source, got %s", out[0].Source)
	}
	if len(paths) != 3 || paths[1] != "/v1/rerank/l1" || paths[2] != "/v1/rerank/l2" {
		t.Fatalf("fallback route sequence: %v", paths)
	}
}
