package agent

import (
	"context"
	"encoding/json"
	"testing"
)

func TestExplicitNumericEquivalence(t *testing.T) {
	for _, tc := range []struct {
		text string
		bad  bool
	}{
		{"±0.7×10⁹ M⊙（即±7 千万太阳质量）", true},
		{"±0.7×10^9 M⊙（即±7 亿太阳质量）", false},
		{"1.2×10^3 米（即1.2万米）", true},
		{"1.2×10^3 米（即1.2千米）", false},
		{"3×10^6 kg（等于3百万千克）", false},
		{"3×10^6 kg（等于30百万千克）", true},
		{"3×10^6 米（相当于3百万秒）", false},
		{"质量(6.5±0.7)×10^9 M⊙，环直径42±3微角秒。", false},
		{"距离410 Mpc和40 Mpc，中心值之比约10倍。", false},
	} {
		if got := len(numericConsistencyIssues(tc.text)) > 0; got != tc.bad {
			t.Errorf("%q: bad=%v want=%v", tc.text, got, tc.bad)
		}
	}
}

func TestNonEmptyAnswerCannotBypassNumericContradiction(t *testing.T) {
	raw, _ := json.Marshal(knowledgeSearchPayload{RetrievalGrade: "A", Hits: []EvidenceHit{{ChunkID: "source", Content: "Uncertainty is 0.7×10^9 solar masses."}}})
	vr, err := (GroundedAnswerValidator{RequiredCitations: 1}).Validate(context.Background(), "误差±0.7×10⁹ M⊙（即±7 千万太阳质量）。", []Observation{{Name: "knowledge.read", Output: raw}})
	if err != nil || !vr.Rejected || vr.Gate.RefuseReason != "numeric_inconsistency" {
		t.Fatalf("%+v %v", vr, err)
	}
}
