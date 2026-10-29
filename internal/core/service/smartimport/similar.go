package smartimport

import (
	"context"
	"strings"

	"github.com/graydraft/core/model"
)

func (s *Service) findSimilarDocuments(ctx context.Context, kbType, title, content string) []model.SimilarDocument {
	if s.embed == nil || s.chunks == nil {
		return nil
	}
	kbType = strings.TrimSpace(kbType)
	if kbType == "" {
		return nil
	}
	snippet := strings.TrimSpace(title)
	body := strings.TrimSpace(content)
	if len(body) > 400 {
		body = body[:400]
	}
	query := strings.TrimSpace(snippet + "\n" + body)
	if query == "" {
		return nil
	}
	vec, err := s.embed.EmbedQuery(ctx, query)
	if err != nil || len(vec) == 0 {
		return nil
	}
	docs, err := s.chunks.SearchSimilarDocuments(ctx, kbType, vec, 5)
	if err != nil {
		return nil
	}
	return docs
}
