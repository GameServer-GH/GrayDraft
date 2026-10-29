package agent

import (
	"context"
	"testing"

	"github.com/graydraft/core/service/confidence"
)

type entailNLI struct{}

func (entailNLI) CheckEntailment(_ context.Context, _ []string, _ string) (string, error) {
	return "entailment", nil
}

func TestBuildSentenceLabelsWithNLI(t *testing.T) {
	hits := []EvidenceHit{{Content: "与句子无关的背景文本"}}
	// Lexical overlap would mark as supplement; NLI forces kb.
	labels := BuildSentenceLabelsWithNLI(context.Background(), "当前版本是 9.9.9。", hits, entailNLI{})
	if len(labels) != 1 || labels[0].Source != "kb" {
		t.Fatalf("labels=%+v", labels)
	}
}

func TestBuildSentenceLabelsNLIFallback(t *testing.T) {
	nli := confidence.LLMCoverageNLI{LLM: nil}
	hits := []EvidenceHit{{Content: "当前版本是 1.2.3"}}
	labels := BuildSentenceLabelsWithNLI(context.Background(), "当前版本是 1.2.3。", hits, nli)
	if len(labels) != 1 || labels[0].Source != "kb" {
		t.Fatalf("expected lexical fallback kb, got %+v", labels)
	}
}
