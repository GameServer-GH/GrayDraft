package provider

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/graydraft/gateway/config"
)

// RerankHit is one document score from a dedicated rerank API.
type RerankHit struct {
	Index int
	Score float64
}

// DashScopeRerankURL is the native text-rerank endpoint.
// The compatible-mode base used for embeddings does not serve this API.
func DashScopeRerankURL(base string) string {
	host := "dashscope.aliyuncs.com"
	if u, err := url.Parse(strings.TrimSpace(base)); err == nil && u.Host != "" {
		host = u.Host
	}
	return "https://" + host + "/api/v1/services/rerank/text-rerank/text-rerank"
}

// TextRerank scores documents with a cross-encoder rerank model.
func TextRerank(ctx context.Context, p config.ProviderConfig, model, query string, documents []string, topN int) ([]RerankHit, error) {
	if model == "" || model == "default" {
		return nil, fmt.Errorf("rerank model not set")
	}
	if len(documents) == 0 {
		return nil, nil
	}
	if topN <= 0 || topN > len(documents) {
		topN = len(documents)
	}
	params := map[string]any{
		"top_n":            topN,
		"return_documents": false,
	}
	if strings.Contains(strings.ToLower(model), "qwen") {
		params["instruct"] = "Given a web search query, retrieve relevant passages that answer the query."
	}
	body, err := json.Marshal(map[string]any{
		"model": model,
		"input": map[string]any{
			"query":     query,
			"documents": documents,
		},
		"parameters": params,
	})
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, DashScopeRerankURL(p.BaseURL), bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+p.APIKey)

	resp, err := (&http.Client{Timeout: 45 * time.Second}).Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("rerank API %d: %s", resp.StatusCode, strings.TrimSpace(string(raw)))
	}
	var parsed struct {
		Output struct {
			Results []struct {
				Index int     `json:"index"`
				Score float64 `json:"relevance_score"`
			} `json:"results"`
		} `json:"output"`
		Code    string `json:"code"`
		Message string `json:"message"`
	}
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return nil, fmt.Errorf("rerank API: %w", err)
	}
	if parsed.Code != "" {
		return nil, fmt.Errorf("rerank API %s: %s", parsed.Code, parsed.Message)
	}
	out := make([]RerankHit, 0, len(parsed.Output.Results))
	for _, row := range parsed.Output.Results {
		if row.Index < 0 || row.Index >= len(documents) {
			continue
		}
		score := row.Score
		if score < 0 {
			score = 0
		}
		if score > 1 {
			score = 1
		}
		out = append(out, RerankHit{Index: row.Index, Score: score})
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("rerank API: empty results")
	}
	return out, nil
}
