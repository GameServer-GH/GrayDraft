package agent

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
)

type captureStreamer struct{ system string }

func (c *captureStreamer) StreamTokens(_ context.Context, system, _ string, emit func(string) error) error {
	c.system = system
	return emit("资料回答。")
}

func TestAnswerEvidenceBudgetRetainsLaterSourcesAndValidJSON(t *testing.T) {
	var hits []EvidenceHit
	for i := 0; i < 8; i++ {
		hits = append(hits, EvidenceHit{DocID: "first", Title: "First material", Content: strings.Repeat("long evidence ", 600)})
	}
	hits = append(hits, EvidenceHit{DocID: "second", Title: "Independent material", Content: "Independent measurement 32.4 with specified conditions."})
	raw, _ := json.Marshal(knowledgeSearchPayload{Hits: hits})
	got := answerObservation(Observation{Name: "knowledge.lexical", Output: raw}, 12000)
	if !json.Valid([]byte(got)) || len([]rune(got)) > 12000 || !strings.Contains(got, "Independent measurement 32.4") {
		t.Fatalf("lost later source or malformed budget: %s", got)
	}
}

func TestAnswerPromptKeepsProperNamesFromEvidence(t *testing.T) {
	s := &captureStreamer{}
	_, err := StreamFinalAnswer(context.Background(), s, []byte("各自讲什么"), nil, "", nil)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(s.system, "按证据原文保留") || !strings.Contains(s.system, "不要换成同一段里出现的另一个名称") {
		t.Fatal(s.system)
	}
}
