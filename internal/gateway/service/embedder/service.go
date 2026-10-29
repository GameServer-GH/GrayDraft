package embedder

import (
	"context"
	"fmt"
	"strings"

	"github.com/graydraft/gateway/config"
	"github.com/graydraft/gateway/service/provider"
)

// Service routes embedding requests to configured API providers.
type Service struct {
	cfg *config.Config
}

// New creates an embedding service.
func New(cfg *config.Config) *Service {
	return &Service{cfg: cfg}
}

// Embed returns vectors for each input text.
func (s *Service) Embed(ctx context.Context, texts []string, model string) ([][]float32, string, error) {
	if len(texts) == 0 {
		return nil, model, nil
	}

	binding := s.cfg.Models.Embedding
	if model == "" || model == "default" {
		model = binding.Model
	}

	if s.cfg.APIReady() {
		p, err := s.cfg.ResolveProvider(binding.Provider)
		if err == nil {
			vecs, err := provider.Embed(ctx, p, model, texts)
			if err == nil {
				return vecs, model, nil
			}
		}
	}

	if !s.cfg.Embedding.FallbackLocal {
		return nil, model, fmt.Errorf("embedding API unavailable (provider=%s model=%s)", binding.Provider, model)
	}

	dim := s.cfg.Embedding.Dim
	out := make([][]float32, len(texts))
	for i, t := range texts {
		out[i] = PseudoEmbed(t, dim)
	}
	return out, "pseudo-v1", nil
}

// DefaultModel returns configured embedding model id.
func (s *Service) DefaultModel() string {
	return strings.TrimSpace(s.cfg.Models.Embedding.Model)
}
