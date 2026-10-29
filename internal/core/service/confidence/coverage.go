package confidence

import (
	"context"
	"encoding/json"
	"log/slog"
	"strings"

	"github.com/graydraft/core/model"
)

// CoverageResult captures semantic coverage evaluation output.
type CoverageResult struct {
	Score             float64  `json:"score"`
	TotalSubQuestions int      `json:"total_sub_questions"`
	CoveredCount      int      `json:"covered_count"`
	MissingTopics     []string `json:"missing_topics,omitempty"`
}

// CoverageLLM abstracts the LLM call for question decomposition.
type CoverageLLM interface {
	Chat(ctx context.Context, messages []CoverageMessage, onChunk func(string) error) (string, error)
}

// CoverageMessage is a simple chat message for coverage evaluation.
type CoverageMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// CoverageNLIService abstracts NLI entailment checks against chunk context.
type CoverageNLIService interface {
	// CheckEntailment returns entailment/neutral/contradiction for a hypothesis
	// given a premise built from reference chunks.
	CheckEntailment(ctx context.Context, premiseParts []string, hypothesis string) (string, error)
}

// EvaluateCoverage decomposes the user question into sub-questions and tests
// each against retrieved chunks using NLI. Returns a CoverageResult.
//
// When llm is nil, falls back to a token-overlap estimate.
// When nli is nil, uses token-overlap for each sub-question.
func EvaluateCoverage(
	ctx context.Context,
	query string,
	chunks []model.Chunk,
	llm CoverageLLM,
	nli CoverageNLIService,
) CoverageResult {
	if len(chunks) == 0 {
		return CoverageResult{Score: 0}
	}

	subQuestions := decomposeQuestion(ctx, query, llm)
	if len(subQuestions) == 0 {
		subQuestions = []string{query}
	}

	premiseParts := make([]string, 0, len(chunks))
	for _, c := range chunks {
		if c.Content != "" {
			premiseParts = append(premiseParts, c.Content)
		}
	}

	if len(premiseParts) == 0 {
		return CoverageResult{
			Score:             0,
			TotalSubQuestions: len(subQuestions),
			MissingTopics:     subQuestions,
		}
	}

	covered := 0
	var missing []string

	for _, sq := range subQuestions {
		if checkSubQuestion(ctx, sq, premiseParts, nli) {
			covered++
		} else {
			missing = append(missing, sq)
		}
	}

	score := 0.0
	if len(subQuestions) > 0 {
		score = float64(covered) / float64(len(subQuestions))
	}

	return CoverageResult{
		Score:             score,
		TotalSubQuestions: len(subQuestions),
		CoveredCount:      covered,
		MissingTopics:     missing,
	}
}

// decomposeQuestion uses LLM to split a complex question into sub-questions.
func decomposeQuestion(ctx context.Context, query string, llm CoverageLLM) []string {
	if llm == nil {
		return nil
	}

	prompt := `将以下用户问题分解为2-5个独立的子问题，每个子问题应是可以单独检验的事实点。
输出纯JSON字符串数组，不附带解释。
问题：` + query + `
输出：`

	messages := []CoverageMessage{
		{Role: "user", Content: prompt},
	}

	result, err := llm.Chat(ctx, messages, func(string) error { return nil })
	if err != nil {
		slog.Debug("coverage: llm decompose failed", "error", err)
		return nil
	}

	result = strings.TrimSpace(result)
	result = strings.TrimPrefix(result, "```json")
	result = strings.TrimPrefix(result, "```")
	result = strings.TrimSuffix(result, "```")
	result = strings.TrimSpace(result)

	var subs []string
	if err := json.Unmarshal([]byte(result), &subs); err != nil {
		slog.Debug("coverage: failed to parse sub-questions", "raw", result, "error", err)
		return nil
	}

	filtered := make([]string, 0, len(subs))
	for _, s := range subs {
		s = strings.TrimSpace(s)
		if s != "" {
			filtered = append(filtered, s)
		}
	}
	return filtered
}

// checkSubQuestion tests if a single sub-question is covered by the premise chunks.
func checkSubQuestion(ctx context.Context, subQuestion string, premiseParts []string, nli CoverageNLIService) bool {
	if nli == nil {
		return tokenOverlapCheck(subQuestion, premiseParts)
	}
	label, err := nli.CheckEntailment(ctx, premiseParts, subQuestion)
	if err != nil {
		slog.Debug("coverage: nli check failed, falling back to token overlap", "error", err)
		return tokenOverlapCheck(subQuestion, premiseParts)
	}
	return label == "entailment"
}

// tokenOverlapCheck falls back to counting shared tokens between sub-question and premise.
func tokenOverlapCheck(subQuestion string, premiseParts []string) bool {
	premise := strings.ToLower(strings.Join(premiseParts, " "))
	terms := QueryTerms(subQuestion)
	hit := 0
	for _, t := range terms {
		if strings.Contains(premise, strings.ToLower(t)) {
			hit++
		}
	}
	if len(terms) == 0 {
		return false
	}
	return float64(hit)/float64(len(terms)) >= 0.4
}
