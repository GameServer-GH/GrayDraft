package agent

import (
	"strings"

	"github.com/graydraft/core/model"
)

const defaultNoAnswerThreshold = 0.62

// AnswerGateResult is the composite refusal decision for a final answer.
type AnswerGateResult struct {
	ClaimReview              *ClaimReview `json:"claim_review,omitempty"`
	EvidencePacketSHA256     string       `json:"evidence_packet_sha256,omitempty"`
	NumericConsistencyIssues []string     `json:"numeric_consistency_issues,omitempty"`
	GroundednessResult
	ClaimCoverage     float64               `json:"claim_coverage"`
	AnswerConfidence  float64               `json:"answer_confidence"`
	RetrievalGrade    string                `json:"retrieval_grade,omitempty"`
	RetrievalScore    float64               `json:"retrieval_score,omitempty"`
	CoverageScore     float64               `json:"coverage_score,omitempty"`
	RefuseReason      string                `json:"refuse_reason,omitempty"`
	RequiredCitations int                   `json:"required_citations,omitempty"`
	CitationCount     int                   `json:"citation_count,omitempty"`
	HighRisk          bool                  `json:"high_risk,omitempty"`
	ThresholdUsed     float64               `json:"threshold_used,omitempty"`
	SentenceLabels    []model.SentenceLabel `json:"sentence_labels,omitempty"`
	HitSummaries      []EvidenceHitSummary  `json:"hit_summaries,omitempty"`
}

// EvidenceHitSummary is a compact citation view for clients and audit events.
type EvidenceHitSummary struct {
	ChunkID  string `json:"chunk_id,omitempty"`
	DocID    string `json:"doc_id,omitempty"`
	Title    string `json:"title,omitempty"`
	MediaURL string `json:"media_url,omitempty"`
}

// GradeToScore maps retrieval grade to a 0–1 score for answer confidence.
func GradeToScore(grade string) float64 {
	switch strings.ToUpper(strings.TrimSpace(grade)) {
	case "A":
		return 1.0
	case "B":
		return 0.78
	case "C":
		return 0.45
	case "D":
		return 0.0
	default:
		return 0.35
	}
}

// AnswerConfidence combines groundedness, claim coverage and retrieval grade.
func AnswerConfidence(groundedness, claimCoverage, retrievalGradeScore float64) float64 {
	return 0.5*groundedness + 0.3*claimCoverage + 0.2*retrievalGradeScore
}

// EvaluateAnswerGate scores an answer against structured evidence and retrieval quality.
func EvaluateAnswerGate(answer string, bundle EvidenceBundle, requiredCitations int, threshold float64) AnswerGateResult {
	if threshold <= 0 {
		threshold = defaultNoAnswerThreshold
	}
	evidence := bundle.texts()
	citationCount := len(bundle.Hits)
	if citationCount == 0 && len(evidence) > 0 {
		citationCount = len(evidence)
	}

	base := EvaluateGroundedness(answer, evidence, citationCount, requiredCitations)
	claimCoverage := claimCoverageScore(answer, bundle.Hits)
	retrievalScore := bundle.RetrievalScore
	if retrievalScore <= 0 {
		retrievalScore = GradeToScore(bundle.RetrievalGrade)
	}
	confidence := AnswerConfidence(base.Groundedness, claimCoverage, retrievalScore)

	out := AnswerGateResult{
		GroundednessResult: base,
		ClaimCoverage:      claimCoverage,
		AnswerConfidence:   confidence,
		RetrievalGrade:     bundle.RetrievalGrade,
		RetrievalScore:     retrievalScore,
		CoverageScore:      bundle.CoverageScore,
		RequiredCitations:  requiredCitations,
		CitationCount:      citationCount,
	}

	switch {
	case len(evidence) == 0 && strings.TrimSpace(bundle.RetrievalGrade) == "D":
		out.ShouldRefuse = true
		out.RefuseReason = "no_evidence"
	case requiredCitations > 0 && citationCount < requiredCitations:
		out.ShouldRefuse = true
		out.RefuseReason = "insufficient_citations"
	case out.Claims > 0 && claimCoverage < threshold:
		out.ShouldRefuse = true
		out.RefuseReason = "low_claim_coverage"
	case confidence < threshold:
		out.ShouldRefuse = true
		out.RefuseReason = "low_answer_confidence"
	default:
		out.ShouldRefuse = base.ShouldRefuse
		if out.ShouldRefuse {
			out.RefuseReason = "low_groundedness"
		}
	}
	return out
}

func claimCoverageScore(answer string, hits []EvidenceHit) float64 {
	parts := sentenceRE.Split(strings.TrimSpace(answer), -1)
	var claims []string
	for _, p := range parts {
		if strings.TrimSpace(p) != "" {
			claims = append(claims, strings.TrimSpace(p))
		}
	}
	if len(claims) == 0 {
		return 0
	}
	supported := 0
	for _, claim := range claims {
		words := contentWords(claim)
		for _, hit := range hits {
			if overlap(words, contentWords(hit.Content)) >= 0.35 {
				supported++
				break
			}
		}
	}
	return float64(supported) / float64(len(claims))
}
