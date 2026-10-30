package llmclient

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

	"github.com/graydraft/core/config"
	"github.com/graydraft/core/observability"
	"github.com/graydraft/core/service/cost"
)

// Client streams chat completions from the model gateway.
type Client struct {
	baseURL      string
	fastModel    string
	complexModel string
	http         *http.Client
}

// Message is a chat message.
type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// StreamHooks receives content and optional model reasoning (e.g. DeepSeek reasoning_content).
type StreamHooks struct {
	OnContent   func(string) error
	OnReasoning func(string) error
}

// NewClient creates a gateway LLM client.
func NewClient(cfg *config.Config) *Client {
	if cfg.ModelGateway.URL == "" {
		return nil
	}
	timeout := time.Duration(cfg.ModelGateway.StreamTimeoutSeconds) * time.Second
	if timeout == 0 {
		timeout = 5 * time.Minute
	}
	return &Client{
		baseURL:      cfg.ModelGateway.URL,
		fastModel:    cfg.ModelGateway.LLMModelFast,
		complexModel: cfg.ModelGateway.LLMModel,
		http:         &http.Client{Timeout: timeout},
	}
}

type chatRequest struct {
	Model           string    `json:"model"`
	Messages        []Message `json:"messages"`
	Stream          bool      `json:"stream"`
	MaxTokens       int       `json:"max_tokens,omitempty"`
	Temperature     float64   `json:"temperature,omitempty"`
	DisableThinking bool      `json:"disable_thinking,omitempty"`
}

type streamChunk struct {
	Choices []struct {
		Delta struct {
			Content          string `json:"content"`
			ReasoningContent string `json:"reasoning_content"`
			Reasoning        string `json:"reasoning"`
		} `json:"delta"`
	} `json:"choices"`
	Usage *struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
	} `json:"usage"`
}

func (c *Client) streamWithModel(ctx context.Context, messages []Message, model string, onDelta func(string) error) error {
	return c.streamWithModelHooks(ctx, messages, model, StreamHooks{OnContent: onDelta})
}

func (c *Client) streamWithModelHooks(ctx context.Context, messages []Message, model string, hooks StreamHooks, opts ...streamOpts) error {
	start := time.Now()
	var out strings.Builder
	contentHook := hooks.OnContent
	wrap := StreamHooks{
		OnReasoning: hooks.OnReasoning,
		OnContent: func(delta string) error {
			out.WriteString(delta)
			if contentHook != nil {
				return contentHook(delta)
			}
			return nil
		},
	}
	opt := streamOpts{}
	if len(opts) > 0 {
		opt = opts[0]
	}
	inTok, outTok, err := c.streamWithModelRaw(ctx, messages, model, wrap, opt)
	lat := time.Since(start)
	observability.RecordLLMLatency("gateway", model, "total", lat.Seconds())
	if inTok == 0 {
		for _, m := range messages {
			inTok += cost.EstimateTokens(m.Content)
		}
	}
	if outTok == 0 {
		outTok = cost.EstimateTokens(out.String())
	}
	status := "ok"
	if err != nil {
		status = "error"
	}
	cost.RecordAsync(ctx, cost.Event{
		Provider: "gateway", Model: model, Operation: "llm",
		InputTokens: inTok, OutputTokens: outTok,
		LatencyMS: int(lat.Milliseconds()), Status: status,
	})
	return err
}

// StreamChat calls gateway /v1/chat/completions with stream=true.

// TierComplex uses the pro model; Gateway falls back to flash on API failure.
func (c *Client) StreamChat(ctx context.Context, messages []Message, tier Tier, onDelta func(string) error) error {
	return c.StreamChatHooks(ctx, messages, tier, StreamHooks{OnContent: onDelta})
}

// StreamChatHooks streams content and reasoning deltas.
func (c *Client) StreamChatHooks(ctx context.Context, messages []Message, tier Tier, hooks StreamHooks) error {
	if c == nil {
		return fmt.Errorf("llm client not configured")
	}
	model := c.fastModel
	if tier == TierComplex {
		model = c.complexModel
	}
	if model == "" {
		if tier == TierComplex {
			model = "complex"
		} else {
			model = "fast"
		}
	}
	return c.streamWithModelHooks(ctx, messages, model, hooks)
}

type streamOpts struct {
	MaxTokens       int
	Temperature     float64
	DisableThinking bool
}

func (c *Client) streamWithModelRaw(ctx context.Context, messages []Message, model string, hooks StreamHooks, opts streamOpts) (int64, int64, error) {
	body, err := json.Marshal(chatRequest{
		Model: model, Messages: messages, Stream: true,
		MaxTokens: opts.MaxTokens, Temperature: opts.Temperature,
		DisableThinking: opts.DisableThinking || opts.MaxTokens > 0,
	})
	if err != nil {
		return 0, 0, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/v1/chat/completions", bytes.NewReader(body))
	if err != nil {
		return 0, 0, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "text/event-stream")

	resp, err := c.http.Do(req)
	if err != nil {
		return 0, 0, fmt.Errorf("gateway chat: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		b, _ := io.ReadAll(resp.Body)
		return 0, 0, fmt.Errorf("gateway chat status %d: %s", resp.StatusCode, string(b))
	}

	return parseSSE(resp.Body, hooks)
}

func parseSSE(r io.Reader, hooks StreamHooks) (int64, int64, error) {
	var inTok, outTok int64
	scanner := bufio.NewScanner(r)
	buf := make([]byte, 0, 64*1024)
	scanner.Buffer(buf, 1024*1024)
	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, "data: ") {
			continue
		}
		payload := strings.TrimPrefix(line, "data: ")
		if payload == "[DONE]" {
			return inTok, outTok, nil
		}
		var chunk streamChunk
		if err := json.Unmarshal([]byte(payload), &chunk); err != nil {
			continue
		}
		if chunk.Usage != nil {
			if chunk.Usage.PromptTokens > 0 {
				inTok = int64(chunk.Usage.PromptTokens)
			}
			if chunk.Usage.CompletionTokens > 0 {
				outTok = int64(chunk.Usage.CompletionTokens)
			}
		}
		if len(chunk.Choices) == 0 {
			continue
		}
		d := chunk.Choices[0].Delta
		reasoning := d.ReasoningContent
		if reasoning == "" {
			reasoning = d.Reasoning
		}
		if reasoning != "" && hooks.OnReasoning != nil {
			if err := hooks.OnReasoning(reasoning); err != nil {
				return inTok, outTok, err
			}
		}
		if d.Content != "" && hooks.OnContent != nil {
			if err := hooks.OnContent(d.Content); err != nil {
				return inTok, outTok, err
			}
		}
	}
	if err := scanner.Err(); err != nil {
		return inTok, outTok, err
	}
	return inTok, outTok, nil
}
