package smartimport

import (
	"testing"

	"github.com/graydraft/core/model"
)

func TestApplyIngestChunkStrategyAfterPivot_section(t *testing.T) {
	req := &model.SmartImportRequest{
		Extensions: map[string]any{"semantic_pivot": "structured_markdown"},
	}
	applyIngestChunkStrategyAfterPivot(req)
	if req.Extensions["ingest_chunk_strategy"] != "section" {
		t.Fatalf("expected section, got %v", req.Extensions["ingest_chunk_strategy"])
	}
}

func TestApplyIngestChunkStrategyAfterPivot_skipsFactTable(t *testing.T) {
	req := &model.SmartImportRequest{
		Extensions: map[string]any{
			"semantic_pivot":          "structured_markdown",
			"ingest_chunk_strategy":   "fact_table",
		},
	}
	applyIngestChunkStrategyAfterPivot(req)
	if req.Extensions["ingest_chunk_strategy"] != "fact_table" {
		t.Fatal("fact_table must not be overridden")
	}
}
