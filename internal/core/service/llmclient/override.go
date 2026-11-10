package llmclient

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/graydraft/gocommon/sealed"
)

// Override streams or completes using an OpenAI-compatible endpoint (workspace-provided key).
type Override struct {
	Provider string
	Model    string
	APIKey   string
}

func providerBaseURL(provider string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(provider)) {
	case "deepseek":
		return "https://api.deepseek.com", nil
	case "openai":
		return "https://api.openai.com", nil
	case "zhipu":
		return "https://open.bigmodel.cn/api/paas/v4", nil
	case "google":
		return "https://generativelanguage.googleapis.com/v1beta/openai", nil
	case "intern":
		return "https://chat.intern-ai.org.cn/api/v1", nil
	default:
		return "", fmt.Errorf("unsupported llm provider for override: %s (use intern, deepseek, openai, zhipu, or google)", provider)
	}
}

func chatCompletionsURL(base, provider string) string {
	base = strings.TrimRight(base, "/")
	if strings.EqualFold(strings.TrimSpace(provider), "intern") {
		return base + "/chat/completions"
	}
	return base + "/v1/chat/completions"
}

func (o *Override) Stream(ctx context.Context, messages []Message, onDelta func(string) error) error {
	return o.StreamHooks(ctx, messages, StreamHooks{OnContent: onDelta})
}

func (o *Override) StreamHooks(ctx context.Context, messages []Message, hooks StreamHooks) error {
	if o == nil || o.Model == "" {
		return fmt.Errorf("llm override incomplete")
	}
	if strings.TrimSpace(o.APIKey) == "" && strings.EqualFold(o.Provider, "intern") {
		o.APIKey = sealed.Intern()
	}
	if o.APIKey == "" {
		return fmt.Errorf("llm override incomplete")
	}
	base, err := providerBaseURL(o.Provider)
	if err != nil {
		return err
	}
	body, err := json.Marshal(map[string]any{
		"model":    o.Model,
		"messages": messages,
		"stream":   true,
	})
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, chatCompletionsURL(base, o.Provider), bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "text/event-stream")
	req.Header.Set("Authorization", "Bearer "+o.APIKey)

	client := &http.Client{Timeout: 5 * time.Minute}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		b, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("llm override %d: %s", resp.StatusCode, strings.TrimSpace(string(b)))
	}
	_, _, err = parseSSE(resp.Body, hooks)
	return err
}

func (o *Override) Complete(ctx context.Context, userPrompt string) (string, error) {
	return o.CompleteWithReasoning(ctx, userPrompt, nil)
}

func (o *Override) CompleteWithReasoning(ctx context.Context, userPrompt string, onReasoning func(string) error) (string, error) {
	var b strings.Builder
	err := o.StreamHooks(ctx, []Message{
		{Role: "system", Content: "You are a helpful assistant. Reply concisely."},
		{Role: "user", Content: userPrompt},
	}, StreamHooks{
		OnContent: func(delta string) error {
			b.WriteString(delta)
			return nil
		},
		OnReasoning: onReasoning,
	})
	return strings.TrimSpace(b.String()), err
}
