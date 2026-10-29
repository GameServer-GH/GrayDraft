package confidence

import (
	"strings"

	"github.com/graydraft/core/model"
)

const (
	rerankWeight = 0.6
	vectorWeight = 0.3
)

// RetrievalGrade categorizes retrieval quality for context injection.
type RetrievalGrade string

const (
	GradeA RetrievalGrade = "A" // high-confidence reference
	GradeB RetrievalGrade = "B" // medium-confidence reference
	GradeC RetrievalGrade = "C" // low-confidence reference
	GradeD RetrievalGrade = "D" // no reference
)

// IsUseful returns true if the grade indicates at least some KB reference is available.
func (g RetrievalGrade) IsUseful() bool {
	return g == GradeA || g == GradeB || g == GradeC
}

// CalculateFinalConfidence combines rerank, vector, and title-hit signals (GrayDraft 3.5 generic).
func CalculateFinalConfidence(query string, chunks []model.Chunk) float64 {
	return CalculateFinalConfidenceWithIntent(query, "", chunks)
}

// CalculateFinalConfidenceWithIntent applies intent-aware table bonus (4.0+ fact QA patch).
func CalculateFinalConfidenceWithIntent(query, queryIntent string, chunks []model.Chunk) float64 {
	_ = queryIntent
	if len(chunks) == 0 {
		return 0
	}
	var maxRerank, maxVector float64
	titleHit := false
	for _, c := range chunks {
		r := c.RerankScore
		if r <= 0 {
			r = c.Score
		}
		if r > maxRerank {
			maxRerank = r
		}
		if c.VectorScore > maxVector {
			maxVector = c.VectorScore
		}
		if c.TitleMatched {
			titleHit = true
		}
	}
	bonus := 0.0
	if titleHit {
		bonus = titleHitBonus
	}
	final := maxRerank*rerankWeight + maxVector*vectorWeight + bonus + StructuredTableBonus(query, chunks)
	return clamp(final)
}

// PassesRetrievalGate reports whether composite confidence meets kb_only / hybrid gate.
func PassesRetrievalGate(query string, chunks []model.Chunk) bool {
	return PassesRetrievalGateWithIntent(query, "", chunks)
}

// GateDecision explains why retrieval is considered useful or not.
type GateDecision struct {
	Useful    bool
	Score     float64
	Threshold float64
	Reason    string
}

// GradeDecision carries the retrieval grade plus coverage and diagnostic info.
type GradeDecision struct {
	Grade         RetrievalGrade `json:"grade"`
	Confidence    float64        `json:"confidence"`
	CoverageScore float64        `json:"coverage_score"`
	Reason        string         `json:"reason"`
	Useful        bool           `json:"useful"`
	Score         float64        `json:"score"`
	Threshold     float64        `json:"threshold"`
	MissingTopics []string       `json:"missing_topics,omitempty"`
}

// PassesRetrievalGateWithIntent uses dynamic thresholds (fact/concept/default) per 4.0+.
func PassesRetrievalGateWithIntent(query, queryIntent string, chunks []model.Chunk) bool {
	return EvaluateRetrievalGate(query, queryIntent, chunks).Useful
}

// EvaluateRetrievalGate returns the useful/miss decision plus score details for observability.
// Deprecated: use EvaluateRetrievalGrade for A/B/C/D grading.
func EvaluateRetrievalGate(query, queryIntent string, chunks []model.Chunk) GateDecision {
	gd := EvaluateRetrievalGrade(query, queryIntent, chunks, nil)
	return GateDecision{
		Useful:    gd.Grade.IsUseful(),
		Score:     gd.Score,
		Threshold: gd.Threshold,
		Reason:    gd.Reason,
	}
}

// EvaluateRetrievalGrade assigns A/B/C/D grade based on retrieval quality.
// aliasIdx is optional; when provided, entity matching uses Jaccard semantic similarity
// instead of the legacy EntityMatched flag.
func EvaluateRetrievalGrade(
	query, queryIntent string,
	chunks []model.Chunk,
	aliasIdx EntityAliasResolver,
) GradeDecision {
	zero := GradeDecision{
		Grade:      GradeD,
		Confidence: 0,
		Score:      0,
		Threshold:  RetrievalGateThreshold(queryIntent),
		Reason:     "no_chunks",
		Useful:     false,
	}
	if len(chunks) == 0 {
		return zero
	}
	score := CalculateFinalConfidenceWithIntent(query, queryIntent, chunks)
	threshold := RetrievalGateThresholdDynamic(queryIntent, chunks)

	top := chunks[0]
	topQuality := top.QualityScore
	hasEntityMatch := top.EntityMatched
	if aliasIdx != nil {
		hasEntityMatch = EntityJaccardMatch(query, chunks, aliasIdx)
	}
	hasTitleMatch := top.TitleMatched

	var grade RetrievalGrade
	var reason string

	if passesHighQualityGrounding(query, chunks) {
		if hasEntityMatch {
			grade = GradeA
			reason = "high_quality_grounding_entity"
		} else {
			grade = GradeB
			reason = "high_quality_grounding_no_entity"
		}
	} else if hasDirectEvidence(query, chunks) {
		grade = GradeB
		reason = "direct_evidence"
	} else if hasTitleMatch || topQuality >= 0.50 {
		grade = GradeB
		reason = "moderate_quality_or_title"
	} else if score >= threshold {
		grade = GradeC
		reason = "score_above_threshold_weak"
	} else {
		grade = GradeC
		reason = "score_below_threshold"
	}

	coverageScore := computeBaseCoverage(grade, topQuality, len(chunks))

	return GradeDecision{
		Grade:         grade,
		Confidence:    score,
		CoverageScore: coverageScore,
		Reason:        reason,
		Useful:        grade.IsUseful(),
		Score:         score,
		Threshold:     threshold,
	}
}

// computeBaseCoverage gives a rough coverage estimate before semantic evaluation.
func computeBaseCoverage(grade RetrievalGrade, topQuality float64, total int) float64 {
	switch grade {
	case GradeA:
		cov := 0.80 + 0.20*(topQuality-0.72)/0.28
		return clamp(cov)
	case GradeB:
		cov := 0.40 + 0.40*(topQuality-0.50)/0.22
		if topQuality < 0.50 {
			cov = 0.40
		}
		return clamp(cov)
	case GradeC:
		cov := 0.10 + 0.30*float64(total)/float64(total+5)
		return clamp(cov)
	default:
		return 0
	}
}

// EntityAliasResolver is the interface for canonicalizing entity names.
type EntityAliasResolver interface {
	CanonicalizeEntities(entities []string) []string
}

// EntityJaccardMatch checks semantic entity overlap using Jaccard similarity
// after normalizing both query and chunk entities through the alias index.
func EntityJaccardMatch(query string, chunks []model.Chunk, aliasIdx EntityAliasResolver) bool {
	if aliasIdx == nil {
		return false
	}
	if len(chunks) == 0 {
		return false
	}

	queryEntities := model.FinalSanitize(nil, query)
	normalizedQuery := aliasIdx.CanonicalizeEntities(queryEntities)
	if len(normalizedQuery) == 0 {
		return false
	}

	var allChunkEntities []string
	for _, c := range chunks {
		if c.Extensions != nil {
			if entities, ok := c.Extensions["entities"]; ok {
				switch v := entities.(type) {
				case []string:
					allChunkEntities = append(allChunkEntities, v...)
				case []interface{}:
					for _, e := range v {
						if s, ok := e.(string); ok {
							allChunkEntities = append(allChunkEntities, s)
						}
					}
				}
			}
		}
	}
	if len(allChunkEntities) == 0 {
		return false
	}
	normalizedChunk := aliasIdx.CanonicalizeEntities(allChunkEntities)
	if len(normalizedChunk) == 0 {
		return false
	}

	intersection := intersectSets(normalizedQuery, normalizedChunk)
	union := unionSets(normalizedQuery, normalizedChunk)
	if len(union) == 0 {
		return false
	}
	return float64(len(intersection))/float64(len(union)) >= 0.3
}

func intersectSets(a, b []string) []string {
	set := make(map[string]struct{}, len(a))
	for _, s := range a {
		set[strings.ToLower(s)] = struct{}{}
	}
	var out []string
	for _, s := range b {
		if _, ok := set[strings.ToLower(s)]; ok {
			out = append(out, s)
		}
	}
	return out
}

func unionSets(a, b []string) []string {
	set := make(map[string]struct{})
	for _, s := range a {
		set[strings.ToLower(s)] = struct{}{}
	}
	for _, s := range b {
		set[strings.ToLower(s)] = struct{}{}
	}
	out := make([]string, 0, len(set))
	for s := range set {
		out = append(out, s)
	}
	return out
}

func hasDirectEvidence(query string, chunks []model.Chunk) bool {
	if len(chunks) == 0 {
		return false
	}
	for i, c := range chunks {
		if i >= 3 {
			break
		}
		if TitleHit(query, c.DocTitle, c.Extensions) || TableKeywordHit(query, c.Content) {
			return true
		}
		if c.QualityScore >= 0.65 && contentKeywordHit(query, c.Content) {
			return true
		}
	}
	return false
}

func contentKeywordHit(query, content string) bool {
	if content == "" {
		return false
	}
	lower := strings.ToLower(content)
	for _, term := range QueryTerms(query) {
		term = strings.TrimSpace(term)
		if len([]rune(term)) < 2 || strings.ContainsAny(term, "，。！？,.!?：:；; ") {
			continue
		}
		if strings.Contains(lower, strings.ToLower(term)) {
			return true
		}
	}
	return false
}
