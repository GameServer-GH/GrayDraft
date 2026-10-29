package rerank

import (
	"context"
	"sort"
	"strings"

	"github.com/graydraft/gateway/config"
	"github.com/graydraft/gateway/service/provider"
)

// Candidate is a document slice to rerank.
type Candidate struct {
	ID      string  `json:"id"`
	Content string  `json:"content"`
	Text    string  `json:"text"`
	Score   float64 `json:"score,omitempty"`
	KBType  string  `json:"kb_type,omitempty"`
	Source  string  `json:"source,omitempty"`
}

func (c Candidate) bodyText() string {
	if c.Content != "" {
		return c.Content
	}
	return c.Text
}

// Service scores candidates with a dedicated rerank model.
type Service struct {
	cfg *config.Config
}

// New creates rerank service.
func New(cfg *config.Config) *Service {
	return &Service{cfg: cfg}
}

// Rerank orders candidates by cross-encoder relevance.
func (s *Service) Rerank(ctx context.Context, query string, candidates []Candidate, model string, topN int) ([]Candidate, string, error) {
	if len(candidates) == 0 {
		return candidates, model, nil
	}
	binding := s.cfg.Models.Rerank
	if model == "" || model == "default" {
		model = binding.Model
	}
	if topN <= 0 || topN > len(candidates) {
		topN = len(candidates)
	}
	if !s.cfg.APIReady() {
		return nil, model, errAPIUnavailable(binding.Provider, model)
	}
	p, err := s.cfg.ResolveProvider(binding.Provider)
	if err != nil {
		return nil, model, err
	}
	docs := make([]string, len(candidates))
	for i, c := range candidates {
		text := strings.TrimSpace(c.bodyText())
		if text == "" {
			text = " "
		}
		docs[i] = text
	}
	hits, err := provider.TextRerank(ctx, p, model, query, docs, topN)
	if err != nil {
		return nil, model, err
	}
	out := orderByHits(candidates, hits, topN)
	return out, model, nil
}

func orderByHits(candidates []Candidate, hits []provider.RerankHit, topN int) []Candidate {
	scores := make([]float64, len(candidates))
	seen := make([]bool, len(candidates))
	for _, hit := range hits {
		if hit.Index < 0 || hit.Index >= len(candidates) || seen[hit.Index] {
			continue
		}
		scores[hit.Index] = hit.Score
		seen[hit.Index] = true
	}
	order := make([]int, len(candidates))
	for i := range order {
		order[i] = i
	}
	sort.SliceStable(order, func(i, j int) bool {
		a, b := order[i], order[j]
		if scores[a] == scores[b] {
			return a < b
		}
		return scores[a] > scores[b]
	})
	if topN > len(order) {
		topN = len(order)
	}
	out := make([]Candidate, topN)
	for i := 0; i < topN; i++ {
		idx := order[i]
		out[i] = candidates[idx]
		out[i].Score = scores[idx]
		out[i].Source = "text_rerank"
	}
	return out
}

var errAPIUnavailable = func(providerName, model string) error {
	return &rerankErr{"rerank API unavailable (provider=" + providerName + " model=" + model + ")"}
}

type rerankErr struct{ msg string }

func (e *rerankErr) Error() string { return e.msg }
