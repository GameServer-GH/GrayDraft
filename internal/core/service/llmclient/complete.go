package llmclient

import (
	"context"
	"strings"
	"time"

	"github.com/graydraft/core/observability"
	"github.com/graydraft/core/service/cost"
)

// Complete runs a single-turn chat and returns the full assistant text (fast tier).
func (c *Client) Complete(ctx context.Context, userPrompt string) (string, error) {
	if c == nil {
		return "", ErrNotConfigured
	}
	var b strings.Builder
	err := c.StreamChatHooks(ctx, []Message{
		{Role: "system", Content: "You are a helpful assistant. Reply concisely."},
		{Role: "user", Content: userPrompt},
	}, TierFast, StreamHooks{
		OnContent: func(delta string) error {
			b.WriteString(delta)
			return nil
		},
	})
	return strings.TrimSpace(b.String()), err
}

// CompleteBounded runs the fast tier with an explicit output cap and temperature.
// Planner decisions use this so a long completion cannot consume the step budget.
func (c *Client) CompleteBounded(ctx context.Context, userPrompt string, maxTokens int, temperature float64) (string, error) {
	if c == nil {
		return "", ErrNotConfigured
	}
	var b strings.Builder
	messages := []Message{
		{Role: "system", Content: "You are a helpful assistant. Reply concisely."},
		{Role: "user", Content: userPrompt},
	}
	err := c.streamFastBounded(ctx, messages, StreamHooks{
		OnContent: func(delta string) error {
			b.WriteString(delta)
			return nil
		},
	}, maxTokens, temperature)
	return strings.TrimSpace(b.String()), err
}

func (c *Client) streamFastBounded(ctx context.Context, messages []Message, hooks StreamHooks, maxTokens int, temperature float64) error {
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
	model := c.fastModel
	if model == "" {
		model = "fast"
	}
	inTok, outTok, err := c.streamWithModelRaw(ctx, messages, model, wrap, streamOpts{MaxTokens: maxTokens, Temperature: temperature})
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

// CompleteWithReasoning streams a completion on the complex/pro model and optionally
// forwards reasoning_content (DeepSeek reasoner / thinking models).
func (c *Client) CompleteWithReasoning(ctx context.Context, userPrompt string, onReasoning func(string) error) (string, error) {
	if c == nil {
		return "", ErrNotConfigured
	}
	var b strings.Builder
	err := c.StreamChatHooks(ctx, []Message{
		{Role: "system", Content: "You are a helpful assistant. Reply concisely."},
		{Role: "user", Content: userPrompt},
	}, TierComplex, StreamHooks{
		OnContent: func(delta string) error {
			b.WriteString(delta)
			return nil
		},
		OnReasoning: onReasoning,
	})
	return strings.TrimSpace(b.String()), err
}

// StreamTokens implements agent.TokenStreamer on the fast model.
// Final answers use StreamTokensWithReasoning so they stay on the complex model.
func (c *Client) StreamTokens(ctx context.Context, system, user string, onDelta func(string) error) error {
	if c == nil {
		return ErrNotConfigured
	}
	msgs := []Message{{Role: "user", Content: user}}
	if strings.TrimSpace(system) != "" {
		msgs = []Message{
			{Role: "system", Content: system},
			{Role: "user", Content: user},
		}
	}
	return c.StreamChatHooks(ctx, msgs, TierFast, StreamHooks{
		OnContent: onDelta,
	})
}

// StreamTokensWithReasoning implements agent.ReasoningTokenStreamer.
// Uses the complex/pro model so reasoner-style models can emit reasoning_content.
func (c *Client) StreamTokensWithReasoning(ctx context.Context, system, user string, onContent, onReasoning func(string) error) error {
	if c == nil {
		return ErrNotConfigured
	}
	msgs := []Message{{Role: "user", Content: user}}
	if strings.TrimSpace(system) != "" {
		msgs = []Message{
			{Role: "system", Content: system},
			{Role: "user", Content: user},
		}
	}
	model := c.complexModel
	if model == "" {
		model = "complex"
	}
	// Final answers should spend the budget on the reply. Long reasoning on the
	// complex tier was consuming the whole run before any content token.
	return c.streamWithModelHooks(ctx, msgs, model, StreamHooks{
		OnContent:   onContent,
		OnReasoning: onReasoning,
	}, streamOpts{DisableThinking: true})
}

// ErrNotConfigured indicates the LLM client is nil.
var ErrNotConfigured = errNotConfigured{}

type errNotConfigured struct{}

func (errNotConfigured) Error() string { return "llm client not configured" }
