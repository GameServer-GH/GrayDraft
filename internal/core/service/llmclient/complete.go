package llmclient

import (
	"context"
	"strings"
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
	return c.StreamChatHooks(ctx, msgs, TierComplex, StreamHooks{
		OnContent:   onContent,
		OnReasoning: onReasoning,
	})
}

// ErrNotConfigured indicates the LLM client is nil.
var ErrNotConfigured = errNotConfigured{}

type errNotConfigured struct{}

func (errNotConfigured) Error() string { return "llm client not configured" }
