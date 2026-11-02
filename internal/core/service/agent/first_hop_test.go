package agent

import (
	"context"
	"encoding/json"
	"testing"
	"time"
)

func TestFirstHopPlannerSkipsLLMOnStepOne(t *testing.T) {
	inner := &plannerStub{decisions: []Decision{{Decision: DecisionFinal, Answer: "should not run"}}}
	p := FirstHopPlanner{
		Inner: inner,
		Select: func(string, []string) (string, bool) {
			return "knowledge.lexical", true
		},
	}
	d, err := p.Decide(context.Background(), PlannerInput{Prompt: []byte("species taxonomy"), Step: 1})
	if err != nil || d.Decision != DecisionTools || len(d.ToolCalls) != 1 || d.ToolCalls[0].Name != "knowledge.lexical" {
		t.Fatalf("decision=%+v err=%v", d, err)
	}
	if inner.i != 0 {
		t.Fatal("inner planner should not run on first hop")
	}
	var args map[string]any
	if err := json.Unmarshal(d.ToolCalls[0].Arguments, &args); err != nil || args["query"] != "species taxonomy" {
		t.Fatalf("args=%v err=%v", args, err)
	}
}

func TestFirstHopPlannerAllowsOneFollowUpThenFinalizes(t *testing.T) {
	inner := &plannerStub{decisions: []Decision{{Decision: DecisionTools, ToolCalls: []ToolCall{{Name: "knowledge.read"}}}}}
	p := FirstHopPlanner{Inner: inner, Select: func(string, []string) (string, bool) { return "knowledge.semantic", true }}
	obs := []Observation{{
		Name:   "knowledge.lexical",
		Output: json.RawMessage(`{"total":1,"hits":[{"id":"c1","content":"Needle 200k"}]}`),
	}}
	d, err := p.Decide(context.Background(), PlannerInput{
		Prompt: []byte("MMMU 分数"), Step: 2, Observations: obs, AuthorizedKBs: []string{"gd_demo"},
	})
	if err != nil || d.Decision != DecisionTools || inner.i != 0 || d.ToolCalls[0].Name != "knowledge.read" {
		t.Fatalf("follow-up decision=%+v inner.i=%d err=%v", d, inner.i, err)
	}
	var args map[string]any
	if err := json.Unmarshal(d.ToolCalls[0].Arguments, &args); err != nil || args["span"] != "group" || args["chunk_id"] != "c1" {
		t.Fatalf("args=%v err=%v", args, err)
	}
	d, err = p.Decide(context.Background(), PlannerInput{Prompt: []byte("MMMU 分数"), Step: 3, Observations: obs})
	if err != nil || d.Decision != DecisionFinal || inner.i != 0 {
		t.Fatalf("finalize decision=%+v inner.i=%d err=%v", d, inner.i, err)
	}
}

func TestFirstHopPlannerDelegatesAfterKnowledgeObs(t *testing.T) {
	inner := &plannerStub{decisions: []Decision{{Decision: DecisionFinal}}}
	p := FirstHopPlanner{
		Inner:  inner,
		Select: func(string, []string) (string, bool) { return "knowledge.lexical", true },
	}
	d, err := p.Decide(context.Background(), PlannerInput{
		Prompt: []byte("q"), Step: 2,
		Observations: []Observation{{Name: "knowledge.lexical", Output: json.RawMessage(`{"total":1}`)}},
	})
	if err != nil || d.Decision != DecisionFinal || inner.i != 1 {
		t.Fatalf("decision=%+v inner.i=%d err=%v", d, inner.i, err)
	}
}

func TestRuntimeFirstHopThenFinal(t *testing.T) {
	inner := &plannerStub{decisions: []Decision{{Decision: DecisionFinal, Answer: "9606"}}}
	p := FirstHopPlanner{
		Inner:  inner,
		Select: func(string, []string) (string, bool) { return "knowledge.lexical", true },
	}
	tools := &toolStub{}
	r := NewRuntime(p, tools, &sinkStub{})
	got, err := r.Run(context.Background(), Request{
		Prompt: []byte("taxonomy id"),
		Limits: Limits{MaxSteps: 4, Deadline: time.Now().Add(time.Second)},
	})
	if err != nil || got.State != StateCompleted || got.ToolCalls != 1 || got.Answer != "9606" {
		t.Fatalf("result=%+v err=%v", got, err)
	}
}

func TestCurrentUserQueryIgnoresPreamble(t *testing.T) {
	prompt := "top_k=5\nchat_mode=hybrid\nworkspace_context:\n科研技能介绍\nspecies 人类 taxonomy ID 是多少"
	if got := CurrentUserQuery(prompt); got != "species 人类 taxonomy ID 是多少" {
		t.Fatal(got)
	}
	hist := "user: 上一问\nassistant: 上一答\nuser: CrystalX 本地上传先调什么"
	if got := CurrentUserQuery(hist); got != "CrystalX 本地上传先调什么" {
		t.Fatal(got)
	}
}

func TestPrefersLexicalIdentifiers(t *testing.T) {
	if !PrefersLexical("species 人类 taxonomy ID 是多少") {
		t.Fatal("taxonomy id")
	}
	if !PrefersLexical("CrystalX 本地上传先调什么，INS 和 HKL 上限分别多少") {
		t.Fatal("crystalx")
	}
	if !PrefersLexical("InternVL 视觉规模与 ViT-22B 的关系") {
		t.Fatal("vit")
	}
	if PrefersLexical("2027 财年营收是多少") {
		t.Fatal("revenue should stay on the normal router")
	}
}

func TestFirstHopPlannerLexicalOverridesSemanticSelect(t *testing.T) {
	inner := &plannerStub{decisions: []Decision{{Decision: DecisionFinal, Answer: "should not run"}}}
	p := FirstHopPlanner{
		Inner: inner,
		Select: func(string, []string) (string, bool) {
			return "knowledge.semantic", true
		},
	}
	d, err := p.Decide(context.Background(), PlannerInput{
		Prompt: []byte("top_k=5\nchat_mode=hybrid\nspecies 人类 taxonomy ID 是多少"),
		Step:   1,
	})
	if err != nil || d.ToolCalls[0].Name != "knowledge.lexical" {
		t.Fatalf("decision=%+v err=%v", d, err)
	}
	var args map[string]any
	if err := json.Unmarshal(d.ToolCalls[0].Arguments, &args); err != nil {
		t.Fatal(err)
	}
	if args["query"] != "species 人类 taxonomy ID 是多少" {
		t.Fatalf("query=%v", args["query"])
	}
}

func TestFirstHopFromIntent(t *testing.T) {
	if FirstHopFromIntent("proper_noun") != "knowledge.lexical" {
		t.Fatal("proper_noun")
	}
	if FirstHopFromIntent("concept") != "knowledge.semantic" {
		t.Fatal("concept")
	}
	if FirstHopFromIntent("hybrid") != "knowledge.catalog" {
		t.Fatal("hybrid")
	}
}
