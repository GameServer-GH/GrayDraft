package agent

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/graydraft/core/service/confidence"
)

type GroundedAnswerValidator struct {
	RequiredCitations int
	ActiveVersion     string
	NoAnswerThreshold float64
	NLI               confidence.CoverageNLIService
}

func (v GroundedAnswerValidator) threshold() float64 {
	if v.NoAnswerThreshold > 0 {
		return v.NoAnswerThreshold
	}
	return defaultNoAnswerThreshold
}

func (v GroundedAnswerValidator) Validate(ctx context.Context, answer string, obs []Observation) (ValidationResult, error) {
	bundle := CollectEvidence(obs)
	threshold := effectiveThreshold(v.threshold(), bundle, answer)
	gate := EvaluateAnswerGate(answer, bundle, v.RequiredCitations, threshold)
	gate.HighRisk = IsHighRiskDomain(strings.TrimSpace(bundle.Query + " " + answer))
	gate.ThresholdUsed = threshold
	gate.HitSummaries = summarizeHits(bundle.Hits)

	evidenceGap := gate.ShouldRefuse && (gate.RefuseReason == "no_evidence" || gate.RefuseReason == "insufficient_citations")
	if evidenceGap {
		RecordAnswerMetric(gate.Groundedness, true)
		return ValidationResult{Answer: refusalMessage(gate), Rejected: true, Gate: gate}, nil
	}

	// A drafted answer that already has usable evidence is kept. Evidence gaps return to the planner.
	if strings.TrimSpace(answer) != "" {
		gate.ShouldRefuse = false
		gate.RefuseReason = ""
		gate.SentenceLabels = BuildSentenceLabels(answer, bundle.Hits)
		RecordAnswerMetric(gate.Groundedness, false)
		return ValidationResult{Answer: answer, Rejected: false, Gate: gate}, nil
	}

	gate.SentenceLabels = BuildSentenceLabelsWithNLI(ctx, answer, bundle.Hits, v.NLI)
	if gate.HighRisk && lacksOfficialSource(bundle) {
		gate.ShouldRefuse = true
		gate.RefuseReason = "high_risk_no_official_source"
	}
	if VersionConflict(answer, v.ActiveVersion, bundle.texts()) {
		gate.ShouldRefuse = true
		gate.RefuseReason = "version_conflict"
	}
	RecordAnswerMetric(gate.Groundedness, gate.ShouldRefuse)
	if gate.ShouldRefuse {
		return ValidationResult{Answer: refusalMessage(gate), Rejected: true, Gate: gate}, nil
	}
	return ValidationResult{Answer: answer, Gate: gate}, nil
}

func summarizeHits(hits []EvidenceHit) []EvidenceHitSummary {
	if len(hits) == 0 {
		return nil
	}
	out := make([]EvidenceHitSummary, 0, len(hits))
	seen := map[string]bool{}
	for _, h := range hits {
		key := h.ChunkID
		if key == "" {
			key = h.DocID + ":" + h.Title
		}
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, EvidenceHitSummary{ChunkID: h.ChunkID, DocID: h.DocID, Title: h.Title, MediaURL: h.MediaURL})
	}
	return out
}

func refusalMessage(gate AnswerGateResult) string {
	switch gate.RefuseReason {
	case "no_evidence":
		return "当前资料里没有可靠依据，无法回答。请缩小到某个概念、参数或文档。"
	case "version_conflict":
		return "资料里的版本说明互相冲突，无法给出可靠答案。请说明你看的是哪一版文档。"
	case "insufficient_citations":
		return "当前证据不足，无法给出带原文引用的回答。请换一种问法，或指出要看的章节。"
	case "high_risk_no_official_source":
		return "该问题涉及账号、支付或权限，当前资料里没有可引用的说明，无法给出可靠回答。"
	default:
		return "当前证据不足，无法可靠回答。请补充要查询的概念、参数或文档范围。"
	}
}

// GateDetailJSON serializes the gate result for audit events.
func GateDetailJSON(gate AnswerGateResult) []byte {
	raw, _ := json.Marshal(gate)
	return raw
}
