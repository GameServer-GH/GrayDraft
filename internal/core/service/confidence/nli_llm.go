package confidence

import (
	"context"
	"fmt"
	"strings"
)

// Completer is the minimal LLM surface used by LLMCoverageNLI.
type Completer interface {
	Complete(ctx context.Context, userPrompt string) (string, error)
}

// LLMCoverageNLI implements CoverageNLIService via a short LLM classification prompt.
// On failure callers should fall back to lexical overlap.
type LLMCoverageNLI struct {
	LLM Completer
}

func (n LLMCoverageNLI) CheckEntailment(ctx context.Context, premiseParts []string, hypothesis string) (string, error) {
	if n.LLM == nil {
		return "", fmt.Errorf("nli: llm unavailable")
	}
	hyp := strings.TrimSpace(hypothesis)
	if hyp == "" {
		return "neutral", nil
	}
	premise := strings.TrimSpace(strings.Join(premiseParts, "\n"))
	if premise == "" {
		return "neutral", nil
	}
	if len([]rune(premise)) > 3500 {
		premise = string([]rune(premise)[:3500]) + "…"
	}
	prompt := fmt.Sprintf(`你是自然语言推理（NLI）分类器。只根据前提判断假设，输出一个英文标签：
entailment（前提蕴含假设）
contradiction（前提与假设矛盾）
neutral（无法判定）

前提：
%s

假设：
%s

只输出一个词：entailment 或 contradiction 或 neutral。`, premise, hyp)

	raw, err := n.LLM.Complete(ctx, prompt)
	if err != nil {
		return "", err
	}
	return normalizeNLILabel(raw), nil
}

func normalizeNLILabel(raw string) string {
	s := strings.ToLower(strings.TrimSpace(raw))
	s = strings.Trim(s, "\"'` \n\t")
	switch {
	case strings.Contains(s, "entail"):
		return "entailment"
	case strings.Contains(s, "contradict"):
		return "contradiction"
	case strings.Contains(s, "neutral"):
		return "neutral"
	default:
		// first token fallback
		fields := strings.Fields(s)
		if len(fields) > 0 {
			switch fields[0] {
			case "entailment", "contradiction", "neutral":
				return fields[0]
			}
		}
		return "neutral"
	}
}
