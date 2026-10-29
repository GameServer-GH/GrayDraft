package chat

import "github.com/graydraft/core/model"

// SourcesFromChunks builds citation list for SSE / feedback (dedupe by chunk id).
func SourcesFromChunks(chunks []model.Chunk) []model.ChatSource {
	seen := make(map[string]struct{}, len(chunks))
	out := make([]model.ChatSource, 0, len(chunks))
	for _, c := range chunks {
		if c.ID == "" || c.DocID == "" {
			continue
		}
		if _, ok := seen[c.ID]; ok {
			continue
		}
		seen[c.ID] = struct{}{}
	out = append(out, model.ChatSource{
			ChunkID:      c.ID,
			DocID:        c.DocID,
			KBType:       c.KBType,
			DocTitle:     c.DocTitle,
			QualityScore: c.QualityScore,
			DocExtensions: c.DocExtensions,
			Extensions:   c.Extensions,
		})
	}
	return out
}
