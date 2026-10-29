package agent

import "testing"

func TestEvaluateAnswerGateRejectsLowConfidence(t *testing.T) {
	bundle := EvidenceBundle{
		RetrievalGrade: "D",
		Hits:           []EvidenceHit{},
	}
	gate := EvaluateAnswerGate("装备隐藏属性是无限大。", bundle, 1, 0.62)
	if !gate.ShouldRefuse || gate.RefuseReason != "no_evidence" {
		t.Fatalf("%+v", gate)
	}
}

func TestEvaluateAnswerGateAcceptsGroundedAnswer(t *testing.T) {
	bundle := EvidenceBundle{
		RetrievalGrade: "B",
		RetrievalScore: 0.78,
		CoverageScore:  0.8,
		Hits: []EvidenceHit{{
			ChunkID: "c1", Content: "当前版本是 1.2.3，新手推荐先完成主线。",
			RecallLane: "vector",
		}},
	}
	gate := EvaluateAnswerGate("当前版本是 1.2.3。", bundle, 1, 0.62)
	if gate.ShouldRefuse || gate.AnswerConfidence < 0.62 {
		t.Fatalf("%+v", gate)
	}
}

func TestCollectEvidenceParsesKnowledgeSearch(t *testing.T) {
	raw := []byte(`{"query":"配装","retrieval_grade":"B","coverage_score":0.7,"hits":[{"id":"c1","content":"配装建议"}]}`)
	bundle := CollectEvidence([]Observation{{Name: "knowledge.semantic", Output: raw}})
	if bundle.RetrievalGrade != "B" || len(bundle.Hits) != 1 || bundle.Hits[0].ChunkID != "c1" {
		t.Fatalf("%+v", bundle)
	}
}

func TestGradeToScore(t *testing.T) {
	if GradeToScore("A") <= GradeToScore("B") || GradeToScore("D") != 0 {
		t.Fatal("grade score ordering invalid")
	}
}
