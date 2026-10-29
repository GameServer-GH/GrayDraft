package multimodal

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/graydraft/core/config"
)

// docIntelClient calls Alibaba Document Mind / DashScope document OCR.
type docIntelClient struct {
	enabled bool
	baseURL string
	apiKey  string
	model   string
	http    *http.Client
}

func newDocIntel(cfg *config.Config) *docIntelClient {
	if cfg == nil || !cfg.Multimodal.DocIntelEnabled {
		return nil
	}
	key := strings.TrimSpace(cfg.Multimodal.DocIntelAPIKey)
	if key == "" {
		key = strings.TrimSpace(cfg.Multimodal.APIKey)
	}
	if key == "" {
		return nil
	}
	base := strings.TrimRight(cfg.Multimodal.DocIntelBaseURL, "/")
	if base == "" {
		base = "https://dashscope.aliyuncs.com/api/v1/services/document-ai/document-processing"
	}
	model := strings.TrimSpace(cfg.Multimodal.DocIntelModel)
	if model == "" {
		model = "doc-ocr"
	}
	timeout := time.Duration(cfg.Multimodal.TimeoutSeconds) * time.Second
	if timeout <= 0 {
		timeout = 120 * time.Second
	}
	return &docIntelClient{
		enabled: true,
		baseURL: base,
		apiKey:  key,
		model:   model,
		http:    &http.Client{Timeout: timeout},
	}
}

type docIntelResult struct {
	Markdown     string
	StructuredKV string
}

// recognize runs OCR-first path (4.0 §4.2 path A).
func (d *docIntelClient) recognize(ctx context.Context, mime, dataURL string) (*docIntelResult, error) {
	if d == nil || !d.enabled {
		return nil, fmt.Errorf("doc intel disabled")
	}
	body, _ := json.Marshal(map[string]any{
		"model": d.model,
		"input": map[string]any{
			"document": map[string]any{
				"url":  dataURL,
				"type": docTypeFromMime(mime),
			},
		},
		"parameters": map[string]any{
			"output_format": "markdown",
			"extract_kv":    true,
		},
	})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, d.baseURL, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+d.apiKey)
	resp, err := d.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("doc intel %d: %s", resp.StatusCode, truncate(string(raw), 300))
	}
	return parseDocIntelResponse(raw)
}

func docTypeFromMime(mime string) string {
	switch {
	case strings.HasPrefix(mime, "image/"):
		return "image"
	case mime == "application/pdf":
		return "pdf"
	default:
		return "image"
	}
}

func parseDocIntelResponse(raw []byte) (*docIntelResult, error) {
	var envelope struct {
		Output struct {
			Text     string `json:"text"`
			Markdown string `json:"markdown"`
			KV       []struct {
				Key   string `json:"key"`
				Value string `json:"value"`
			} `json:"kv_pairs"`
			Choices []struct {
				Message struct {
					Content string `json:"content"`
				} `json:"message"`
			} `json:"choices"`
		} `json:"output"`
		Result struct {
			Markdown string `json:"markdown"`
			KV       []struct {
				Key   string `json:"key"`
				Value string `json:"value"`
			} `json:"kv_pairs"`
		} `json:"result"`
	}
	if err := json.Unmarshal(raw, &envelope); err != nil {
		return nil, err
	}
	out := &docIntelResult{}
	out.Markdown = strings.TrimSpace(envelope.Output.Markdown)
	if out.Markdown == "" {
		out.Markdown = strings.TrimSpace(envelope.Output.Text)
	}
	if out.Markdown == "" && len(envelope.Output.Choices) > 0 {
		out.Markdown = strings.TrimSpace(envelope.Output.Choices[0].Message.Content)
	}
	if out.Markdown == "" {
		out.Markdown = strings.TrimSpace(envelope.Result.Markdown)
	}
	kv := envelope.Output.KV
	if len(kv) == 0 {
		kv = envelope.Result.KV
	}
	var kvLines []string
	for _, pair := range kv {
		k := strings.TrimSpace(pair.Key)
		v := strings.TrimSpace(pair.Value)
		if k != "" && v != "" {
			kvLines = append(kvLines, k+": "+v)
		}
	}
	out.StructuredKV = strings.Join(kvLines, "\n")
	if out.Markdown == "" && out.StructuredKV == "" {
		return nil, fmt.Errorf("doc intel: empty output")
	}
	return out, nil
}
