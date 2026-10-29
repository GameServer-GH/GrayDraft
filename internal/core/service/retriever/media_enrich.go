package retriever

import (
	"github.com/graydraft/core/model"
)

// enrichChunksWithMedia attaches multimodal media refs from document extensions (4.0 §5.2).
func enrichChunksWithMedia(chunks []model.Chunk) []model.Chunk {
	for i := range chunks {
		c := &chunks[i]
		mm, ok := c.Extensions["multimodal"].(map[string]any)
		if !ok {
			continue
		}
		assets, ok := mm["assets"].([]any)
		if !ok || len(assets) == 0 {
			continue
		}
		first, ok := assets[0].(map[string]any)
		if !ok {
			continue
		}
		if u, ok := first["media_url"].(string); ok && u != "" {
			c.MediaURL = u
		}
		if ts, ok := first["timestamps"].([]any); ok {
			for _, t := range ts {
				if s, ok := t.(string); ok && s != "" {
					c.MediaTimestamps = append(c.MediaTimestamps, s)
				}
			}
		}
	}
	return chunks
}
