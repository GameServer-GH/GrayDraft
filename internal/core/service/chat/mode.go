package chat

import (
	"github.com/graydraft/core/model"
	"github.com/graydraft/core/service/confidence"
)

const (
	// ChatModeKBOnly answers only from retrieval snippets (no LLM).
	ChatModeKBOnly = "kb_only"
	// ChatModeHybrid uses RAG when retrieval is useful; otherwise general LLM.
	ChatModeHybrid = "hybrid"
)

// NormalizeChatMode returns a supported mode; empty defaults to hybrid.
func NormalizeChatMode(mode string) string {
	switch mode {
	case ChatModeKBOnly:
		return ChatModeKBOnly
	default:
		return ChatModeHybrid
	}
}

// HasUsefulRetrieval reports whether retrieved chunks pass the composite confidence gate.
// Deprecated: use EvaluateRetrievalGrade for A/B/C/D grading.
func HasUsefulRetrieval(chunks []model.Chunk, total int, query, queryIntent string) bool {
	return confidence.PassesRetrievalGateWithIntent(query, queryIntent, chunks)
}

// EvaluateRetrievalGrade assigns A/B/C/D grade based on retrieval quality.
func EvaluateRetrievalGrade(
	chunks []model.Chunk,
	total int,
	query, queryIntent string,
	aliasIdx confidence.EntityAliasResolver,
) confidence.GradeDecision {
	if len(chunks) == 0 || total == 0 {
		return confidence.GradeDecision{
			Grade:      confidence.GradeD,
			Confidence: 0,
			Score:      0,
			Threshold:  confidence.RetrievalGateThreshold(queryIntent),
			Reason:     "no_chunks",
			Useful:     false,
		}
	}
	return confidence.EvaluateRetrievalGrade(query, queryIntent, chunks, aliasIdx)
}

// RetrievalGateDecision exposes the useful/miss decision with debug details for SSE meta.
// Deprecated: use EvaluateRetrievalGrade for A/B/C/D grading.
func RetrievalGateDecision(chunks []model.Chunk, total int, query, queryIntent string) confidence.GateDecision {
	if len(chunks) == 0 || total == 0 {
		return confidence.GateDecision{
			Useful:    false,
			Score:     0,
			Threshold: confidence.RetrievalGateThreshold(queryIntent),
			Reason:    "no_chunks",
		}
	}
	return confidence.EvaluateRetrievalGate(query, queryIntent, chunks)
}
