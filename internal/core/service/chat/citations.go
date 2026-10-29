package chat

import "github.com/graydraft/core/model"

// CitationsFromChunks returns one entry per document (one citation card per source document).
func CitationsFromChunks(chunks []model.Chunk) []model.ChatCitation {
	byDoc := make(map[string]*model.ChatCitation)
	order := make([]string, 0)
	for _, c := range chunks {
		if c.DocID == "" {
			continue
		}
		if _, ok := byDoc[c.DocID]; !ok {
			byDoc[c.DocID] = &model.ChatCitation{
				DocID:      c.DocID,
				DocTitle:   c.DocTitle,
				KBType:     c.KBType,
				Extensions: c.Extensions,
				ChunkIDs:   []string{},
			}
			order = append(order, c.DocID)
		}
		cit := byDoc[c.DocID]
		if c.ID != "" {
			cit.ChunkIDs = append(cit.ChunkIDs, c.ID)
		}
		if len(c.Extensions) > 0 && len(cit.Extensions) == 0 {
			cit.Extensions = c.Extensions
		}
	}
	out := make([]model.ChatCitation, 0, len(order))
	for _, docID := range order {
		out = append(out, *byDoc[docID])
	}
	return out
}
