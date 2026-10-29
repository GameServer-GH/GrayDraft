package agent

import (
	"context"
	"encoding/json"
	"testing"
)

func TestGroundedAnswerValidatorKeepsNonEmptyAnswer(t *testing.T) {
	v := GroundedAnswerValidator{RequiredCitations: 1}
	payload, _ := json.Marshal(knowledgeSearchPayload{
		RetrievalGrade: "C",
		Hits:           []EvidenceHit{{Content: "当前版本 1.2"}},
	})
	vr, err := v.Validate(context.Background(), "隐藏属性无限大", []Observation{{
		Name: "knowledge.semantic", Output: payload,
	}})
	if err != nil || vr.Rejected || vr.Answer != "隐藏属性无限大" {
		t.Fatalf("expected keep answer, vr=%+v err=%v", vr, err)
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
