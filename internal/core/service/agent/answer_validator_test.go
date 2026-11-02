package agent

import (
	"context"
	"encoding/json"
	"testing"
)

func TestGroundedAnswerValidatorKeepsCrossLanguageEvidenceAnswer(t *testing.T) {
	v := GroundedAnswerValidator{RequiredCitations: 1}
	payload, _ := json.Marshal(knowledgeSearchPayload{
		RetrievalGrade: "C",
		Hits:           []EvidenceHit{{Content: "The current version is 1.2."}},
	})
	vr, err := v.Validate(context.Background(), "资料当前版本为 1.2。", []Observation{{
		Name: "knowledge.semantic", Output: payload,
	}})
	if err != nil || vr.Rejected || vr.Answer != "资料当前版本为 1.2。" {
		t.Fatalf("expected keep answer, vr=%+v err=%v", vr, err)
	}
}

func TestNonEmptyAnswerCannotBypassHardPolicies(t *testing.T) {
	for _, tc := range []struct{ name, answer, evidence, version, reason string }{
		{"no_source", "有答案。", "", "", "no_evidence"},
		{"version", "当前版本 2.0。", "官方说明，当前版本 1.2。", "1.2", "version_conflict"},
		{"risk", "支付订单可以直接退款。", "某社区关于退款的讨论。", "", "high_risk_no_official_source"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			payload, _ := json.Marshal(knowledgeSearchPayload{RetrievalGrade: "A", Hits: []EvidenceHit{{ChunkID: "source", Content: tc.evidence}}})
			vr, err := (GroundedAnswerValidator{RequiredCitations: 1, ActiveVersion: tc.version}).Validate(context.Background(), tc.answer, []Observation{{Name: "knowledge.read", Output: payload}})
			if err != nil || !vr.Rejected || vr.Gate.RefuseReason != tc.reason {
				t.Fatalf("%+v %v", vr, err)
			}
		})
	}
}

func TestGroundedAnswerValidatorRejectsEmptyAnswer(t *testing.T) {
	v := GroundedAnswerValidator{RequiredCitations: 1}
	payload, _ := json.Marshal(knowledgeSearchPayload{
		RetrievalGrade: "D",
		Hits:           nil,
	})
	vr, err := v.Validate(context.Background(), "   ", []Observation{{
		Name: "knowledge.semantic", Output: payload,
	}})
	if err != nil || !vr.Rejected || vr.Gate.RefuseReason == "" {
		t.Fatalf("expected refuse empty answer, vr=%+v err=%v", vr, err)
	}
}
