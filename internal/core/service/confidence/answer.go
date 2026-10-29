package confidence

import (
	"regexp"
	"strings"
	"unicode"

	"github.com/graydraft/core/model"
)

// AnswerConfidence captures multi-dimensional answer quality signals.
type AnswerConfidence struct {
	Score                 float64  `json:"score"`
	FactConsistency       float64  `json:"fact_consistency"`
	OverConfidenceWarning bool     `json:"over_confidence_warning"`
	SourceConfusionRisk   bool     `json:"source_confusion_risk"`
	SupplementRatio       float64  `json:"supplement_ratio,omitempty"`
}

// FromAnswer estimates answer faithfulness vs context (O-10 heuristic; no extra LLM call).
func FromAnswer(answer string, contextChunks []string, retrievalConf float64) float64 {
	result := EvaluateAnswerConfidence(answer, contextChunks, retrievalConf, GradeC, nil)
	return result.Score
}

// EvaluateAnswerConfidence provides the extended confidence assessment with
// fact consistency check, over-confidence detection, and source confusion risk.
// grade is the retrieval grade; sentenceLabels (optional) enables supplement ratio.
func EvaluateAnswerConfidence(
	answer string,
	contextChunks []string,
	retrievalConf float64,
	grade RetrievalGrade,
	sentenceLabels []model.SentenceLabel,
) AnswerConfidence {
	answer = strings.TrimSpace(answer)
	if answer == "" {
		return AnswerConfidence{Score: 0}
	}

	ctx := strings.Join(contextChunks, "\n")
	if ctx == "" {
		return AnswerConfidence{Score: clamp(retrievalConf * 0.5)}
	}

	// 1. Base token-overlap score (original algorithm)
	terms := tokenSet(answer)
	ctxTerms := tokenSet(ctx)
	if len(terms) == 0 {
		return AnswerConfidence{Score: clamp(retrievalConf * 0.6)}
	}
	hit := 0
	for t := range terms {
		if _, ok := ctxTerms[t]; ok {
			hit++
		}
	}
	overlap := float64(hit) / float64(len(terms))
	score := 0.35*retrievalConf + 0.65*overlap
	if len(answer) < 20 {
		score *= 0.85
	}

	result := AnswerConfidence{
		Score:           clamp(score),
		FactConsistency: clamp(overlap),
	}

	// 2. Fact consistency: check proper nouns and numbers via alias normalization
	result.FactConsistency = computeFactConsistency(answer, contextChunks)

	// 3. Over-confidence detection: triggered when grade <= C
	if grade == GradeC || grade == GradeD {
		result.OverConfidenceWarning = detectOverConfidence(answer, contextChunks)
	}

	// 4. Source confusion risk: check supplement ratio from sentence labels
	if len(sentenceLabels) > 0 {
		suppCount := 0
		for _, sl := range sentenceLabels {
			if sl.Source == "supplement" {
				suppCount++
			}
		}
		result.SupplementRatio = float64(suppCount) / float64(len(sentenceLabels))
		if result.SupplementRatio > 0.6 {
			result.SourceConfusionRisk = true
		}
	} else {
		// Fallback: check if answer looks like pure LLM without KB grounding
		if grade == GradeD {
			result.SourceConfusionRisk = true
		}
	}

	return result
}

// computeFactConsistency checks if proper nouns and numbers in the answer
// appear in at least one context chunk (with alias normalization).
func computeFactConsistency(answer string, contextChunks []string) float64 {
	if len(contextChunks) == 0 {
		return 0.5
	}

	// Extract potential proper nouns and numbers from answer
	entities := extractAnswerEntities(answer)
	if len(entities) == 0 {
		return 0.8 // No entities to check → assume reasonable consistency
	}

	fullCtx := strings.ToLower(strings.Join(contextChunks, " "))
	matched := 0
	for _, e := range entities {
		eLower := strings.ToLower(e)
		if strings.Contains(fullCtx, eLower) {
			matched++
			continue
		}
		// Also try numeric normalization
		if isNumeric(e) {
			if numericOverlapInContext(e, contextChunks) {
				matched++
			}
		}
	}

	return clamp(float64(matched) / float64(len(entities)))
}

var (
	reProperNoun    = regexp.MustCompile(`[\p{Han}]{2,6}|[A-Z][a-z]+(?:\s[A-Z][a-z]+)*`)
	reNumber        = regexp.MustCompile(`\d+(?:\.\d+)?\s*(%|[%％]|倍|x|点|级|层|s|秒|m|米|w|万)?`)
	reAbsoluteTerms = regexp.MustCompile(`(一定|绝对|所有|必然|100%|百分百|肯定|毋庸置疑)`)
)

func extractAnswerEntities(answer string) []string {
	var out []string
	seen := make(map[string]bool)

	// Extract proper nouns (CJK 2-6 chars or Latin capitalized words)
	for _, m := range reProperNoun.FindAllString(answer, -1) {
		m = strings.TrimSpace(m)
		if len([]rune(m)) >= 2 && !isStopEntity(m) {
			k := strings.ToLower(m)
			if !seen[k] {
				seen[k] = true
				out = append(out, m)
			}
		}
	}

	// Extract numbers with units
	for _, m := range reNumber.FindAllString(answer, -1) {
		m = strings.TrimSpace(m)
		if !seen[strings.ToLower(m)] {
			seen[strings.ToLower(m)] = true
			out = append(out, m)
		}
	}

	return out
}

func isStopEntity(s string) bool {
	stops := map[string]bool{
		"不是": true, "没有": true, "可以": true, "如果": true, "因为": true,
		"所以": true, "但是": true, "而且": true, "或者": true, "以及": true,
		"一个": true, "这个": true, "那个": true, "什么": true, "怎么": true,
		"如何": true, "为什么": true, "是否": true, "已经": true, "可能": true,
	}
	return stops[strings.ToLower(s)]
}

func isNumeric(s string) bool {
	return reNumber.MatchString(s)
}

func numericOverlapInContext(val string, contextChunks []string) bool {
	// Strip unit and compare numeric portion
	numeric := reNumber.FindStringSubmatch(val)
	if len(numeric) < 2 {
		return false
	}
	num := numeric[1]
	for _, c := range contextChunks {
		matches := reNumber.FindAllStringSubmatch(c, -1)
		for _, m := range matches {
			if len(m) >= 2 && m[1] == num {
				return true
			}
		}
	}
	return false
}

func detectOverConfidence(answer string, contextChunks []string) bool {
	// Check for absolute terms when context is sparse
	if !reAbsoluteTerms.MatchString(answer) {
		return false
	}
	// If context chunks have very little content, an absolute claim is suspicious
	totalLen := 0
	for _, c := range contextChunks {
		totalLen += len([]rune(c))
	}
	if totalLen < 100 {
		return true
	}
	// Check if answer is much longer than context (suggesting speculation)
	answerLen := len([]rune(answer))
	if answerLen > totalLen*2 {
		return true
	}
	return false
}

// tokenSet returns lowercase unique tokens (>=2 chars) with punctuation stripped.
func tokenSet(s string) map[string]struct{} {
	out := make(map[string]struct{})
	for _, w := range strings.Fields(strings.ToLower(s)) {
		w = trimPunct(w)
		if len([]rune(w)) < 2 {
			continue
		}
		out[w] = struct{}{}
	}
	return out
}

func trimPunct(s string) string {
	return strings.TrimFunc(s, func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsNumber(r)
	})
}
