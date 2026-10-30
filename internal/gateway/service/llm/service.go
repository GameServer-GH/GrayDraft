package llm

import (
	"context"
	"strings"

	"github.com/graydraft/gateway/config"
	"github.com/graydraft/gateway/service/provider"
)

// Service routes chat completion requests.
type Service struct {
	cfg *config.Config
}

// New creates an LLM service.
func New(cfg *config.Config) *Service {
	return &Service{cfg: cfg}
}

// StreamCompletion streams tokens to emit callback.
func (s *Service) StreamCompletion(ctx context.Context, req ChatRequest, emit func(StreamDelta) error) error {
	if req.MaxTokens == 0 {
		req.MaxTokens = s.cfg.LLM.MaxTokens
	}

	binding, model, complexTier := resolveBinding(s.cfg, req.Model)
	err := s.streamWithBinding(ctx, req, binding, model, !complexTier || req.DisableThinking, emit)
	if err == nil {
		return nil
	}

	// Complex tier: fall back to fast model when pro API fails.
	if complexTier {
		fast := s.cfg.Models.LLMFast
		if fast.Model != "" && !strings.EqualFold(fast.Model, model) {
			if fbErr := s.streamWithBinding(ctx, req, fast, fast.Model, true, emit); fbErr == nil {
				return nil
			}
		}
	}

	if !s.cfg.LLM.FallbackLocal {
		return err
	}
	return PseudoStream(ctx, req.Messages, emit)
}

func (s *Service) streamWithBinding(
	ctx context.Context,
	req ChatRequest,
	binding config.ModelBinding,
	model string,
	disableThinking bool,
	emit func(StreamDelta) error,
) error {
	if !s.cfg.APIReady() {
		return providerStreamError(binding.Provider, model)
	}
	p, err := s.cfg.ResolveProvider(binding.Provider)
	if err != nil {
		return err
	}
	msgs := make([]provider.ChatMessage, len(req.Messages))
	for i, m := range req.Messages {
		msgs[i] = provider.ChatMessage{Role: m.Role, Content: m.Content} // Content is any, passes through as-is
	}
	temp := req.Temperature
	if temp == 0 {
		temp = s.cfg.LLM.Temperature
	}
	if disableThinking && !strings.EqualFold(binding.Provider, "intern") {
		disableThinking = false
	}
	return provider.StreamChat(ctx, p, model, msgs, req.MaxTokens, temp, disableThinking, func(content, reasoning string, done bool) error {
		return emit(StreamDelta{Content: content, Reasoning: reasoning, Done: done})
	})
}

func providerStreamError(provider, model string) error {
	return &providerError{msg: "llm API unavailable (provider=" + provider + " model=" + model + ")"}
}

type providerError struct{ msg string }

func (e *providerError) Error() string { return e.msg }
