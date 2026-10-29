package classifier

import (
	"context"
	"strings"

	"github.com/graydraft/core/service/llmclient"
)

// classifyWithLLM asks the fast model for intent when rules are ambiguous (O-09).
func classifyWithLLM(ctx context.Context, llm *llmclient.Client, query string) (Intent, float64, string) {
	if llm == nil {
		return IntentUnknown, 0, "llm_unavailable"
	}
	prompt := `Classify the user query into exactly one label: proper_noun, fact, concept, hybrid.
Reply with only the label.

Query: ` + query
	out, err := llm.Complete(ctx, prompt)
	if err != nil {
		return IntentUnknown, 0, "llm_error"
	}
	label := strings.ToLower(strings.TrimSpace(out))
	switch {
	case strings.Contains(label, "proper"):
		return IntentProperNoun, 0.82, "llm_proper_noun"
	case strings.Contains(label, "fact"):
		return IntentFact, 0.82, "llm_fact"
	case strings.Contains(label, "hybrid"):
		return IntentHybrid, 0.82, "llm_hybrid"
	default:
		return IntentConcept, 0.82, "llm_concept"
	}
}
