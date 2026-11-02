package agent

import (
	"encoding/json"
	"testing"
)

func TestIsHighRiskDomain(t *testing.T) {
	if !IsHighRiskDomain("我的账号被封了怎么办") {
		t.Fatal("expected high risk")
	}
	if IsHighRiskDomain("新手怎么配装") {
		t.Fatal("expected low risk")
	}
}

func TestEffectiveThresholdHighRisk(t *testing.T) {
	th := effectiveThreshold(0.62, EvidenceBundle{Query: "充值不到账"}, "请联系客服")
	if th != highRiskNoAnswerThreshold {
		t.Fatalf("threshold = %v, want %v", th, highRiskNoAnswerThreshold)
	}
	th = effectiveThreshold(0.62, EvidenceBundle{Query: "配装推荐"}, "推荐战士")
	if th != 0.62 {
		t.Fatalf("threshold = %v, want 0.62", th)
	}
}

func TestBuildSentenceLabels(t *testing.T) {
	hits := []EvidenceHit{{Content: "当前版本是 1.2.3，新手推荐先完成主线。"}}
	labels := BuildSentenceLabels("当前版本是 1.2.3。建议先完成主线。", hits)
	if len(labels) != 2 {
		t.Fatalf("labels=%+v", labels)
	}
	if labels[0].Source != "kb" {
		t.Fatalf("first label=%+v", labels[0])
	}
}

func TestLacksOfficialSource(t *testing.T) {
	bundle := EvidenceBundle{
		RetrievalGrade: "B",
		Hits:           []EvidenceHit{{Content: "配装建议"}},
	}
	if !lacksOfficialSource(bundle) {
		t.Fatal("expected no official source")
	}
	bundle.Hits = []EvidenceHit{{Title: "官方公告", Content: "充值说明"}}
	if lacksOfficialSource(bundle) {
		t.Fatal("expected official hit")
	}
}

func TestHighRiskNonEmptyAnswerStillRequiresOfficialSource(t *testing.T) {
	payload := jsonMarshalKnowledgeSearch("充值问题", "B", 0.7, []EvidenceHit{{
		ChunkID: "c1", Content: "充值一般 1-3 分钟到账。",
	}})
	v := GroundedAnswerValidator{RequiredCitations: 1}
	res, err := v.Validate(nil, "充值可能稍慢。", []Observation{{Name: "knowledge.semantic", Output: payload}})
	if err != nil {
		t.Fatal(err)
	}
	if !res.Rejected || res.Gate.RefuseReason != "high_risk_no_official_source" {
		t.Fatalf("expected source policy refusal, gate=%+v rejected=%v", res.Gate, res.Rejected)
	}
}

func jsonMarshalKnowledgeSearch(query, grade string, coverage float64, hits []EvidenceHit) []byte {
	raw, _ := json.Marshal(map[string]any{
		"query": query, "retrieval_grade": grade, "coverage_score": coverage, "hits": hits,
	})
	return raw
}
