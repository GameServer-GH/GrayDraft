package agent

import (
	"encoding/json"
	"testing"
)

func TestResolveClarifyWithEvidenceForcesFinal(t *testing.T) {
	payload, _ := json.Marshal(knowledgeSearchPayload{
		Query: "ViT-22B", Total: 1, RetrievalGrade: "B",
		Hits: []EvidenceHit{{ChunkID: "c1", Content: "InternVL scales the vision encoder to 6B and is an alternative to ViT-22B."}},
	})
	d := resolveClarify(Decision{Decision: DecisionClarify, Answer: "请补充范围"}, []Observation{{
		CallID: "1", Name: "knowledge.lexical", Output: payload,
	}})
	if d.Decision != DecisionFinal || d.Answer != "" {
		t.Fatalf("got %+v", d)
	}
}

func TestResolveClarifyEmptyEvidenceRefuses(t *testing.T) {
	payload, _ := json.Marshal(knowledgeSearchPayload{Query: "营收", Total: 0, RetrievalGrade: "D", Hits: nil})
	d := resolveClarify(Decision{Decision: DecisionClarify}, []Observation{{
		CallID: "1", Name: "knowledge.semantic", Output: payload,
	}})
	if d.Decision != DecisionFinal || d.Answer != refuseNoEvidenceTemplate {
		t.Fatalf("got %+v", d)
	}
}

func TestResolveClarifyWithoutSearchKeepsClarify(t *testing.T) {
	d := resolveClarify(Decision{Decision: DecisionClarify, Answer: ""}, nil)
	if d.Decision != DecisionClarify || d.Answer != clarifyAskTemplate {
		t.Fatalf("got %+v", d)
	}
}
