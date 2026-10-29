package chat

import (
	"testing"

	"github.com/graydraft/core/model"
)

func TestCitationsFromChunks_groupsByDoc(t *testing.T) {
	chunks := []model.Chunk{
		{ID: "c1", DocID: "d1", KBType: "tutorial", Extensions: model.Extensions{"article_id": "a1"}},
		{ID: "c2", DocID: "d1", KBType: "tutorial"},
		{ID: "c3", DocID: "d2", KBType: "tutorial", Extensions: model.Extensions{"article_id": "a2"}},
	}
	cits := CitationsFromChunks(chunks)
	if len(cits) != 2 {
		t.Fatalf("want 2 citations, got %d", len(cits))
	}
	if cits[0].Extensions["article_id"] != "a1" || len(cits[0].ChunkIDs) != 2 {
		t.Fatalf("unexpected first citation: %+v", cits[0])
	}
}
