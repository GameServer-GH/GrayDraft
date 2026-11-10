package agent

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestNumericReviewRetainsReproducibleOperandsAndApproximationBoundary(t *testing.T) {
	for _, tc := range []struct{ text, status, action, left, right string }{
		{"±0.7×10⁹ M⊙（即±7 千万太阳质量）", "contradicted_magnitude", "reject", "700000000", "70000000"},
		{"0.7×10^9 M⊙（即7亿太阳质量）", "consistent_magnitude", "none", "700000000", "700000000"},
		{"1.01×10^6 kg（相当于1百万千克）", "approximation_requires_review", "review", "1010000", "1000000"},
		{"约1.01×10^6 kg（即约1百万千克）", "approximation_requires_review", "review", "1010000", "1000000"},
		{"1×10^-3 kg（等于0.000000001百万千克）", "consistent_magnitude", "none", "1/1000", "1/1000"},
		{"3×10^6 米（等于3百万秒）", "unit_binding_unverified", "review", "", ""},
	} {
		rows, _ := reviewNumericOperations(tc.text)
		if len(rows) != 1 {
			t.Fatalf("%q: %+v", tc.text, rows)
		}
		r := rows[0]
		if r.Status != tc.status || r.Action != tc.action || r.LeftMagnitude != tc.left || r.RightMagnitude != tc.right {
			t.Errorf("%q: %+v", tc.text, r)
		}
	}
}

func TestClaimReviewDoesNotStoreFullCandidate(t *testing.T) {
	candidate := "PRIVATE_CANDIDATE_SENTENCE。误差0.7×10^9 M⊙（即7千万太阳质量）。"
	r := reviewAnswerClaims(candidate, EvidenceBundle{})
	raw, _ := json.Marshal(r)
	if strings.Contains(string(raw), "PRIVATE_CANDIDATE_SENTENCE") || r.CandidateSHA256 != candidateReviewHash(candidate) || r.Disposition != "reject" {
		t.Fatalf("%s", raw)
	}
	if len(r.Numeric) != 1 || r.Numeric[0].RelativeDifference != "9/10" || r.Numeric[0].CanonicalUnit != "太阳质量" {
		t.Fatalf("%+v", r.Numeric)
	}
}

func TestReviewKeepsCountingObjectsAndStatisticRolesSeparate(t *testing.T) {
	bundle := EvidenceBundle{Query: "展示了几个工具？", Hits: []EvidenceHit{{ChunkID: "tools", Content: "以下展示5个工具的典型调用方法。"}, {ChunkID: "metric", Content: "匹配滤波信噪比为31。检测统计量为30.4。"}}}
	r := reviewAnswerClaims("示例展示4个工具。信噪比为30.4。", bundle)
	count, metric := false, false
	for _, c := range r.Roles {
		if c.Candidate.Kind == "count" {
			count = true
			if c.Candidate.Label != "工具" || c.Status != "count_value_not_observed_for_role" {
				t.Fatalf("%+v", c)
			}
		}
		if c.Candidate.Kind == "statistic" && c.Candidate.Value == "30.4" {
			metric = true
			if c.Status != "value_seen_under_other_role" {
				t.Fatalf("%+v", c)
			}
		}
		if c.Action != "review" {
			t.Fatal("ambiguous semantic binding became hard rejection")
		}
	}
	if !count || !metric || r.RequestedCountObject != "工具" {
		t.Fatalf("%+v", r)
	}
}

func TestIdentifierReviewUsesOnlyVisiblePacket(t *testing.T) {
	bundle := EvidenceBundle{Query: "请求头名是什么", Hits: []EvidenceHit{{ChunkID: "visible", Content: "Authentication uses the TOOL-ACCESS-KEY header.", Truncated: true}}}
	r := reviewAnswerClaims("请求头名为 `TOOL-ACCESS-KE`。", bundle)
	found := false
	for _, c := range r.Roles {
		if c.Candidate.Kind == "identifier" && c.Candidate.Value == "TOOL-ACCESS-KE" {
			found = true
			if c.Status != "identifier_not_observed_in_packet" {
				t.Fatalf("%+v", c)
			}
			for _, e := range c.Evidence {
				if e.ChunkID != "visible" || !e.PacketReferenceTruncated {
					t.Fatal("unexposed or unmarked evidence")
				}
			}
		}
	}
	if !found {
		t.Fatal("missing identifier observation")
	}
	// No matching text in the exposed packet must remain unverified, not be
	// "proved" by metadata, a title or a presumed complete original document.
	r = reviewAnswerClaims("信噪比为31。", EvidenceBundle{Hits: []EvidenceHit{{ChunkID: "cut", Content: "exposed opening only", Truncated: true}}})
	for _, c := range r.Roles {
		if c.Candidate.Kind == "statistic" && c.Status != "no_role_evidence_in_packet" {
			t.Fatalf("%+v", c)
		}
	}
}

func TestEpistemicReviewPreservesAssumptionExplanationEstimateAndNegation(t *testing.T) {
	bundle := EvidenceBundle{Hits: []EvidenceHit{{ChunkID: "source", Content: "如果采用受限假设，估计质量为3。现象可用该模型解释。"}}}
	r := reviewAnswerClaims("已证实该模型。没有唯一确定全部参数。", bundle)
	certainty, negative := false, false
	roles := map[string]bool{}
	for _, c := range r.Roles {
		if c.Candidate.Kind != "epistemic" {
			continue
		}
		if c.Candidate.Label == "certainty" {
			certainty = true
		}
		if c.Candidate.Polarity == "negation_present" {
			negative = true
		}
		for _, e := range c.Evidence {
			roles[e.Label] = true
		}
	}
	if !certainty || !negative || !roles["assumption"] || !roles["explanation"] || !roles["estimate"] {
		t.Fatalf("%+v", r)
	}
	if r.Disposition != "review_required" {
		t.Fatal("cue detection claimed semantic proof")
	}
}

func TestGroundedValidatorRecordsAuditAndDoesNotRejectApproximation(t *testing.T) {
	raw, _ := json.Marshal(knowledgeSearchPayload{Query: "质量是多少", Hits: []EvidenceHit{{ChunkID: "c", Content: "Mass estimated at 1.01 million kilograms."}}})
	obs, _ := prepareAnswerEvidence([]byte("质量是多少"), []Observation{{Name: "knowledge.read", Output: raw}})
	v := GroundedAnswerValidator{RequiredCitations: 1}
	vr, err := v.Validate(context.Background(), "1.01×10^6 kg（相当于1百万千克）。", obs)
	if err != nil || vr.Rejected || vr.Gate.ClaimReview == nil || vr.Gate.ClaimReview.Disposition != "review_required" {
		t.Fatalf("%+v %v", vr, err)
	}
	if vr.Gate.EvidencePacketSHA256 != answerEvidenceDigest(obs) {
		t.Fatal("validator did not independently hash its input")
	}
}

type mismatchedPacketValidator struct{}

func (mismatchedPacketValidator) Validate(context.Context, string, []Observation) (ValidationResult, error) {
	return ValidationResult{Answer: "must not commit", Gate: AnswerGateResult{EvidencePacketSHA256: "wrong-packet"}}, nil
}
func TestRuntimeNeverCommitsAnIndependentlyMismatchedPacket(t *testing.T) {
	sink := &sinkStub{}
	rt := NewRuntime(staticPlanner{Decision{Decision: DecisionFinal}}, nil, sink).WithAnswerStreamer(stubStreamer{deltas: []string{"candidate"}}).WithAnswerValidator(mismatchedPacketValidator{})
	out, err := rt.Run(context.Background(), Request{Prompt: []byte("question"), Limits: Limits{MaxSteps: 1, Deadline: time.Now().Add(time.Second)}})
	if err == nil || out.Answer != "" {
		t.Fatalf("%+v %v", out, err)
	}
	for _, e := range sink.events {
		// 乐观流式下候选 answer.delta 允许；权威提交（run.completed）绝不出现。
		if e.Type == "run.completed" {
			t.Fatal("mismatched packet committed")
		}
	}
}

func TestRuntimeRejectsKnownWrongConversionWithMinimalReproducibleAudit(t *testing.T) {
	candidate := "误差0.7×10^9 kg（即7千万千克）。"
	raw, _ := json.Marshal(knowledgeSearchPayload{Hits: []EvidenceHit{{ChunkID: "source", Content: "The uncertainty is 0.7 billion kilograms."}}})
	sink := &sinkStub{}
	rt := NewRuntime(staticPlanner{Decision{Decision: DecisionFinal}}, nil, sink).WithAnswerStreamer(stubStreamer{deltas: []string{candidate}}).WithAnswerValidator(GroundedAnswerValidator{RequiredCitations: 1})
	out, err := rt.Run(context.Background(), Request{Prompt: []byte("误差是多少"), SeedObservations: []Observation{{Name: "knowledge.read", Output: raw}}, Limits: Limits{MaxSteps: 1, Deadline: time.Now().Add(time.Second)}})
	if err != nil || out.State != StateCompleted || strings.Contains(out.Answer, "7千万") {
		t.Fatalf("%+v %v", out, err)
	}
	found := false
	for _, e := range sink.events {
		if e.Type == "answer.rejected" {
			var gate AnswerGateResult
			if err := json.Unmarshal([]byte(e.Message), &gate); err != nil {
				t.Fatal(err)
			}
			if gate.ClaimReview == nil || gate.ClaimReview.CandidateSHA256 != candidateReviewHash(candidate) || len(gate.ClaimReview.Numeric) != 1 || gate.ClaimReview.Numeric[0].RelativeDifference != "9/10" || gate.EvidencePacketSHA256 == "" {
				t.Fatalf("%+v", gate)
			}
			found = true
		}
		// 乐观流式：被拒候选可能已作为 answer.delta 流出，
		// 但权威终态（run.completed）绝不能包含错误换算。
		if e.Type == "run.completed" && strings.Contains(e.Message, "7千万") {
			t.Fatal("rejected candidate leaked into final answer")
		}
	}
	if !found {
		t.Fatal("missing reproducible rejection audit")
	}
}

func TestAvailableCountsAndExampleCountsDoNotProveEachOther(t *testing.T) {
	r := reviewAnswerClaims("典型示例包含5个工具。", EvidenceBundle{Query: "展示几个工具", Hits: []EvidenceHit{{ChunkID: "available", Content: "The server exposes 5 tools."}}})
	found := false
	for _, c := range r.Roles {
		if c.Candidate.Kind == "count" {
			found = true
			if c.Status == "role_value_observed_binding_unverified" {
				t.Fatalf("different counting sets conflated: %+v", c)
			}
		}
	}
	if !found {
		t.Fatal("missing count review")
	}
	r = reviewAnswerClaims("参数名为 `TOOL-ACCESS-KEY`。", EvidenceBundle{Query: "参数名是什么", Hits: []EvidenceHit{{ChunkID: "header", Content: "Authentication uses the TOOL-ACCESS-KEY header."}}})
	found = false
	for _, c := range r.Roles {
		if c.Candidate.Kind == "identifier" {
			found = true
			if c.Status != "identifier_seen_under_other_role" {
				t.Fatalf("header and parameter roles conflated: %+v", c)
			}
		}
	}
	if !found {
		t.Fatal("missing identifier role review")
	}
}

func TestPlainParameterNamesAndScientificIdentifiersDoNotBecomeMetricValues(t *testing.T) {
	r := reviewAnswerClaims("参数名为 limit。信噪比X123456为30.4。", EvidenceBundle{Query: "参数名是什么", Hits: []EvidenceHit{{ChunkID: "parameters", Content: "parameter name is limit."}, {ChunkID: "metric", Content: "检测统计量为30.4。"}}})
	found := false
	for _, c := range r.Roles {
		if c.Candidate.Kind == "identifier" && c.Candidate.Value == "limit" {
			found = true
			if c.Candidate.Label != "parameter_name" {
				t.Fatalf("%+v", c)
			}
		}
		if c.Candidate.Kind == "statistic" && c.Candidate.Value == "123456" {
			t.Fatal("entity ID became statistic value")
		}
	}
	if !found {
		t.Fatal("plain parameter name missed")
	}
}

func TestExplicitIdentifierReviewIgnoresAdjacentDocumentAndClassNames(t *testing.T) {
	answer := "在 `example-client-doc` 的 `ExampleClient` 类中，请求头名为 `TOOL-ACCESS-KEY`。"
	r := reviewAnswerClaims(answer, EvidenceBundle{Query: "请求头名是什么"})
	var names []string
	for _, c := range r.Roles {
		if c.Candidate.Kind == "identifier" {
			names = append(names, c.Candidate.Value)
		}
	}
	if len(names) != 1 || names[0] != "TOOL-ACCESS-KEY" {
		t.Fatalf("adjacent names became header candidates: %v", names)
	}
}

func TestReviewFlagsQuestionCountScopeAndNestedStatisticRole(t *testing.T) {
	r := reviewAnswerClaims("服务器暴露5个工具。匹配滤波信噪比：检测统计量为30.4。", EvidenceBundle{Query: "展示了几个工具的典型调用方法", Hits: []EvidenceHit{{ChunkID: "c", Content: "该服务暴露5个工具。示例展示4个工具。匹配滤波信噪比31。检测统计量30.4。"}}})
	count, stat := false, false
	for _, c := range r.Roles {
		if c.Status == "count_scope_requires_review" {
			count = true
		}
		if c.Status == "statistic_role_binding_ambiguous" && c.Candidate.Label == "snr" && c.Candidate.RelatedRole == "detection_statistic" {
			stat = true
		}
	}
	if !count || !stat || r.RequestedCountScope != "example_count" {
		t.Fatalf("%+v", r)
	}
}

func TestReviewDoesNotConfuseHeaderContainersOrSplitStatisticLexemes(t *testing.T) {
	r := reviewAnswerClaims("请求头为 `headers={\"TOOL-ACCESS-KEY\": \"placeholder\"}`。误警率小于1 / 203 000年。显著性大于5.1σ。", EvidenceBundle{Query: "请求头及误警率是什么"})
	header, rate := false, false
	for _, c := range r.Roles {
		if c.Candidate.Kind == "identifier" {
			if c.Candidate.Value != "TOOL-ACCESS-KEY" {
				t.Fatalf("container labeled as header: %+v", c)
			}
			header = true
		}
		if c.Candidate.Label == "false_alarm" {
			if c.Candidate.Value != "1/203000" || c.Candidate.Bound != "lower_than" {
				t.Fatalf("split statistic: %+v", c)
			}
			rate = true
		}
	}
	if !header || !rate {
		t.Fatalf("%+v", r)
	}
}
