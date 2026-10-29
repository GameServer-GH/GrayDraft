package chat

import (
	"context"
	"fmt"

	"github.com/graydraft/core/model"
	"github.com/graydraft/core/service/llmclient"
)

func (s *Service) streamLLM(ctx context.Context, req model.ChatRequest, messages []llmclient.Message, onDelta func(string) error) error {
	return s.streamLLMWithTier(ctx, req, messages, llmclient.TierComplex, onDelta)
}

func (s *Service) streamLLMWithTier(ctx context.Context, req model.ChatRequest, messages []llmclient.Message, tier llmclient.Tier, onDelta func(string) error) error {
	if req.LLMOverride.Valid() {
		o := &llmclient.Override{
			Provider: req.LLMOverride.Provider,
			Model:    req.LLMOverride.Model,
			APIKey:   req.LLMOverride.APIKey,
		}
		return o.Stream(ctx, messages, onDelta)
	}
	if s.llm == nil {
		return fmt.Errorf("llm not configured")
	}
	return s.llm.StreamChat(ctx, messages, tier, onDelta)
}

func (s *Service) llmAvailable(req model.ChatRequest) bool {
	return req.LLMOverride.Valid() || s.llm != nil
}
