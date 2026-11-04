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

type dashscopeClient struct {
	baseURL string
	apiKey  string
	http    *http.Client
}

func newDashscope(cfg *config.Config) *dashscopeClient {
	if cfg == nil || !cfg.Multimodal.Enabled {
		return nil
	}
	key := strings.TrimSpace(cfg.Multimodal.APIKey)
	if key == "" {
		return nil
	}
	base := strings.TrimRight(cfg.Multimodal.BaseURL, "/")
	if base == "" {
		base = "https://dashscope.aliyuncs.com/compatible-mode"
	}
	timeout := time.Duration(cfg.Multimodal.TimeoutSeconds) * time.Second
	if timeout <= 0 {
		timeout = 120 * time.Second
	}
	return &dashscopeClient{
		baseURL: base,
		apiKey:  key,
		http:    &http.Client{Timeout: timeout},
	}
}

type visionPart struct {
	Type     string `json:"type"`
	Text     string `json:"text,omitempty"`
	ImageURL *struct {
		URL string `json:"url"`
	} `json:"image_url,omitempty"`
	VideoURL *struct {
		URL string `json:"url"`
	} `json:"video_url,omitempty"`
}

func (c *dashscopeClient) chat(ctx context.Context, model, prompt, mediaDataURL string, isVideo bool) (string, error) {
	if c == nil {
		return "", fmt.Errorf("dashscope not configured")
	}
	content := []visionPart{{Type: "text", Text: prompt}}
	if mediaDataURL != "" {
		if isVideo {
			content = append([]visionPart{{
				Type: "video_url",
				VideoURL: &struct {
					URL string `json:"url"`
				}{URL: mediaDataURL},
			}}, content...)
		} else {
			content = append([]visionPart{{
				Type: "image_url",
				ImageURL: &struct {
					URL string `json:"url"`
				}{URL: mediaDataURL},
			}}, content...)
		}
	}
	body, _ := json.Marshal(map[string]any{
		"model": model,
		"messages": []map[string]any{
			{"role": "user", "content": content},
		},
	})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.chatCompletionsURL(), bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	resp, err := c.http.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 400 {
		return "", fmt.Errorf("dashscope %d: %s", resp.StatusCode, truncate(string(b), 400))
	}
	var parsed struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(b, &parsed); err != nil {
		return "", err
	}
	if len(parsed.Choices) == 0 {
		return "", fmt.Errorf("dashscope: empty choices")
	}
	return strings.TrimSpace(parsed.Choices[0].Message.Content), nil
}

func (c *dashscopeClient) chatCompletionsURL() string {
	base := strings.TrimRight(c.baseURL, "/")
	if strings.HasSuffix(base, "/chat/completions") {
		return base
	}
	if strings.Contains(base, "ark.cn-beijing.volces.com") || strings.HasSuffix(base, "/api/v3") || strings.HasSuffix(base, "/v1") {
		return base + "/chat/completions"
	}
	return base + "/v1/chat/completions"
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
