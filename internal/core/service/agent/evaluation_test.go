package agent

import "testing"

func TestBuildReport(t *testing.T) {
	cases := []GoldenCase{{ID: "c1", Query: "q", TenantID: "t", ExpectedKeywords: []string{"1024"}, ExpectedTools: []string{"knowledge.lexical"}, RequiredCitations: 1}}
	r := BuildReport(cases, []ReplayResult{{CaseID: "c1", Answer: "嵌入维度 1024", Tools: []string{"knowledge.lexical"}, CitationCount: 1}})
	if r.PassRate != 1 {
		t.Fatalf("%+v", r)
	}
}
func TestReplayDataset(t *testing.T) {
	cases := []GoldenCase{{ID: "c", Query: "q", TenantID: "t", MustRefuse: true}}
	r, err := ReplayDataset(cases, func(c GoldenCase) (ReplayResult, error) { return ReplayResult{CaseID: c.ID, Refused: true}, nil })
	if err != nil || r.PassRate != 1 {
		t.Fatalf("%+v %v", r, err)
	}
}
