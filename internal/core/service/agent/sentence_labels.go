package agent

import (
	"context"
	"strings"

	"github.com/graydraft/core/model"
	"github.com/graydraft/core/service/confidence"
)

// BuildSentenceLabels splits the answer into sentences and labels each as kb or supplement
// based on lexical overlap with retrieved evidence hits.
func BuildSentenceLabels(answer string, hits []EvidenceHit) []model.SentenceLabel {
	return BuildSentenceLabelsWithNLI(context.Background(), answer, hits, nil)
}

// BuildSentenceLabelsWithNLI prefers NLI entailment when nli is non-nil; falls back to lexical overlap.
func BuildSentenceLabelsWithNLI(ctx context.Context, answer string, hits []EvidenceHit, nli confidence.CoverageNLIService) []model.SentenceLabel {
	if strings.TrimSpace(answer) == "" {
		return nil
	}
	evidence := make([]string, 0, len(hits))
	for _, h := range hits {
		if t := strings.TrimSpace(h.Content); t != "" {
			evidence = append(evidence, t)
		}
	}
	if len(evidence) == 0 {
		return nil
	}
	lexCtx := strings.ToLower(strings.Join(evidence, " "))

	parts := sentenceRE.Split(strings.TrimSpace(answer), -1)
	var sentences []string
	for _, p := range parts {
		if s := strings.TrimSpace(p); s != "" {
			sentences = append(sentences, s)
		}
	}
	if len(sentences) == 0 {
		return nil
	}

	labels := make([]model.SentenceLabel, 0, len(sentences))
	for _, s := range sentences {
		source := "supplement"
		if sentenceEntailed(ctx, s, evidence, lexCtx, nli) {
			source = "kb"
		}
		labels = append(labels, model.SentenceLabel{Sentence: s, Source: source})
	}
	return labels
}

func sentenceEntailed(ctx context.Context, sentence string, evidence []string, lexCtx string, nli confidence.CoverageNLIService) bool {
	if nli != nil {
		label, err := nli.CheckEntailment(ctx, evidence, sentence)
		if err == nil {
			return label == "entailment"
		}
	}
	return sentenceGrounded(sentence, lexCtx)
}

func sentenceGrounded(sentence, ctx string) bool {
	words := contentWords(sentence)
	if len(words) == 0 {
		return false
	}
	hit := 0
	for w := range words {
		if strings.Contains(ctx, w) {
			hit++
		}
	}
	return float64(hit)/float64(len(words)) >= 0.35
}
