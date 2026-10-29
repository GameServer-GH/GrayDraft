package provider

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/graydraft/gateway/config"
)

// openAIURL joins an OpenAI-compatible base with an API path.
// Bases that already end in /v1, such as the InternLM endpoint, are not given a second /v1.
func openAIURL(base, path string) string {
	base = strings.TrimRight(base, "/")
	path = strings.TrimLeft(path, "/")
	if strings.HasSuffix(base, "/v1") {
		return base + "/" + path
	}
	return base + "/v1/" + path
}

// Embed calls OpenAI-compatible POST /v1/embeddings.
func Embed(ctx context.Context, p config.ProviderConfig, model string, texts []string) ([][]float32, error) {
	if model == "" || model == "default" {
		return nil, fmt.Errorf("embedding model not set")
	}
	body, _ := json.Marshal(map[string]any{"model": model, "input": texts})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, openAIURL(p.BaseURL, "embeddings"), bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+p.APIKey)

	resp, err := (&http.Client{Timeout: 60 * time.Second}).Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		b, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("embeddings API %d: %s", resp.StatusCode, string(b))
	}

	var parsed struct {
		Data []struct {
			Embedding []float64 `json:"embedding"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&parsed); err != nil {
		return nil, err
	}
	if len(parsed.Data) != len(texts) {
		return nil, fmt.Errorf("embeddings: size mismatch")
	}
	out := make([][]float32, len(texts))
	for i, d := range parsed.Data {
		vec := make([]float32, len(d.Embedding))
		for j, v := range d.Embedding {
			vec[j] = float32(v)
		}
		out[i] = vec
	}
	return out, nil
}

// ChatMessage for completions API.
type ChatMessage struct {
	Role    string `json:"role"`
	Content any    `json:"content"` // string or array for vision
}

// StreamChat streams OpenAI-compatible POST /v1/chat/completions.
// emit receives content and/or reasoning_content deltas (DeepSeek reasoner / thinking models).
func StreamChat(
	ctx context.Context,
	p config.ProviderConfig,
	model string,
	messages []ChatMessage,
	maxTokens int,
	temperature float64,
	disableThinking bool,
	emit func(content, reasoning string, done bool) error,
) error {
	if model == "" || model == "default" {
		return fmt.Errorf("llm model not set")
	}
	payload := map[string]any{
		"model": model, "messages": messages, "stream": true,
	}
	if maxTokens > 0 {
		payload["max_tokens"] = maxTokens
	}
	if temperature > 0 {
		payload["temperature"] = temperature
	}
	if disableThinking {
		payload["thinking_mode"] = false
	}
	body, _ := json.Marshal(payload)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, openAIURL(p.BaseURL, "chat/completions"), bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+p.APIKey)

	resp, err := (&http.Client{Timeout: 5 * time.Minute}).Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		b, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("chat API %d: %s", resp.StatusCode, string(b))
	}

	scanner := bufio.NewScanner(resp.Body)
	buf := make([]byte, 0, 64*1024)
	scanner.Buffer(buf, 1024*1024)
	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, "data: ") {
			continue
		}
		payload := strings.TrimPrefix(line, "data: ")
		if payload == "[DONE]" {
			return emit("", "", true)
		}
		var chunk struct {
			Choices []struct {
				Delta struct {
					Content          string `json:"content"`
					ReasoningContent string `json:"reasoning_content"`
					Reasoning        string `json:"reasoning"`
				} `json:"delta"`
				FinishReason *string `json:"finish_reason"`
			} `json:"choices"`
		}
		if json.Unmarshal([]byte(payload), &chunk) != nil || len(chunk.Choices) == 0 {
			continue
		}
		d := chunk.Choices[0].Delta
		reasoning := d.ReasoningContent
		if reasoning == "" {
			reasoning = d.Reasoning
		}
		if d.Content != "" || reasoning != "" {
			if err := emit(d.Content, reasoning, false); err != nil {
				return err
			}
		}
		// Intern 等兼容接口会在每个增量里带上空字符串 finish_reason。
		// 只有非空结束原因才表示流结束，否则会在第一个 token 后截断。
		if fr := chunk.Choices[0].FinishReason; fr != nil && strings.TrimSpace(*fr) != "" {
			return emit("", "", true)
		}
	}
	return scanner.Err()
}

// Complete calls OpenAI-compatible POST /v1/chat/completions and returns the assistant text.
func Complete(ctx context.Context, p config.ProviderConfig, model string, messages []ChatMessage, maxTokens int, temperature float64) (string, error) {
	if model == "" || model == "default" {
		return "", fmt.Errorf("llm model not set")
	}
	payload := map[string]any{
		"model": model, "messages": messages, "stream": false, "temperature": temperature,
	}
	if maxTokens > 0 {
		payload["max_tokens"] = maxTokens
	}
	body, _ := json.Marshal(payload)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, openAIURL(p.BaseURL, "chat/completions"), bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+p.APIKey)
	resp, err := (&http.Client{Timeout: 60 * time.Second}).Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		b, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("chat API %d: %s", resp.StatusCode, string(b))
	}
	var parsed struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&parsed); err != nil {
		return "", err
	}
	if len(parsed.Choices) == 0 {
		return "", fmt.Errorf("chat API: empty choices")
	}
	return strings.TrimSpace(parsed.Choices[0].Message.Content), nil
}
