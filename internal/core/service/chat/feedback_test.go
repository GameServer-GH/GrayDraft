package chat

import (
	"testing"

	"github.com/graydraft/core/model"
)

func TestValidateFeedbackRequest(t *testing.T) {
	if err := ValidateFeedbackRequest(model.ChatFeedbackRequest{Vote: "down", ChunkIDs: []string{"550e8400-e29b-41d4-a716-446655440000"}}); err != nil {
		t.Fatal(err)
	}
	if err := ValidateFeedbackRequest(model.ChatFeedbackRequest{Vote: "up", DocIDs: []string{"550e8400-e29b-41d4-a716-446655440001"}}); err != nil {
		t.Fatal(err)
	}
	if err := ValidateFeedbackRequest(model.ChatFeedbackRequest{Vote: "up"}); err == nil {
		t.Fatal("expected error for missing ids")
	}
	if err := ValidateFeedbackRequest(model.ChatFeedbackRequest{Vote: "maybe", ChunkIDs: []string{"x"}}); err == nil {
		t.Fatal("expected error for invalid vote")
	}
}

func TestSourcesFromChunks_dedupe(t *testing.T) {
	chunks := []model.Chunk{
		{ID: "a", DocID: "d1", KBType: "paper", QualityScore: 0.5},
		{ID: "a", DocID: "d1", KBType: "paper"},
	}
	src := SourcesFromChunks(chunks)
	if len(src) != 1 {
		t.Fatalf("want 1 source, got %d", len(src))
	}
}
