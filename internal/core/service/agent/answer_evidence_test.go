package agent

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestStructuredInputPreservesLiteralMultilineQuestion(t *testing.T) {
	question := "top_k=5\n请解释参数含义\n保留这三行，不是运行配置"
	raw := EncodeRunInput(question, InputConfig{TopK: 9, ChatMode: "hybrid", WorkspaceContext: "workspace background"})
	// JSON queue round trip must preserve the persisted envelope.
	queued, _ := json.Marshal(RunRecord{Prompt: raw})
	var restored RunRecord
	_ = json.Unmarshal(queued, &restored)
	got, config := DecodeRunInput(restored.Prompt)
	if got != question || config.TopK != 9 {
		t.Fatalf("%q %+v", got, config)
	}
	if CurrentUserQuery(literalQuestionPrefix+got) != question {
		t.Fatal("literal question was parsed as legacy metadata")
	}
	if CurrentUserQuery("第一行问题\n第二行问题") != "第一行问题\n第二行问题" {
		t.Fatal("multiline question lost")
	}
	plain := `{"question":"ordinary JSON question"}`
	got, _ = DecodeRunInput([]byte(plain))
	if got != plain {
		t.Fatal("ordinary JSON decoded as envelope")
	}
}

func TestPreparedPacketIsBoundedAndRetainsExactVisibleEvidence(t *testing.T) {
	raw, _ := json.Marshal(knowledgeSearchPayload{Query: "retrieval subquery", RetrievalGrade: "A", Hits: []EvidenceHit{{ChunkID: "c", DocID: "d", Title: "Independent source", SourceRevision: "revision", Content: strings.Repeat("测量正文", 6000) + "HIDDEN_TAIL"}}})
	original := string(raw)
	obs, sha := prepareAnswerEvidence([]byte("actual question"), []Observation{{Name: "knowledge.read", Output: raw}})
	if len(sha) != 64 || len(obs) != 1 || string(raw) != original {
		t.Fatal("mutated source or missing packet")
	}
	hits := CollectEvidence(obs).Hits
	if len(hits) != 1 || !hits[0].Truncated || hits[0].OriginalContentSHA256 == "" || hits[0].SourceRevision != "revision" {
		t.Fatalf("%+v", hits)
	}
	if CollectEvidence(obs).Query != "actual question" {
		t.Fatal("gate received retrieval subquery as user question")
	}
	visible := answerObservation(obs[0], 12000)
	if visible != string(obs[0].Output) || strings.Contains(visible, "HIDDEN_TAIL") || len([]rune(visible)) > 12000 {
		t.Fatal("packet re-truncated or leaked tail")
	}
	prompt := buildAnswerStreamPrompt([]byte("actual question"), obs, "")
	if !strings.Contains(prompt, visible) {
		t.Fatal("composition did not use packet bytes")
	}
	if !strings.Contains(answerEvidenceAudit(obs, sha), sha) {
		t.Fatal("audit packet hash missing")
	}
}

func TestOverviewGateOnlySeesEvidenceSuppliedToSelectedGroups(t *testing.T) {
	catalog := json.RawMessage(`{"total":2,"documents":[{"title":"论文 Alpha"},{"title":"论文 Beta"}]}`)
	read, _ := json.Marshal(knowledgeSearchPayload{Hits: []EvidenceHit{{ChunkID: "a", Title: "论文 Alpha", Content: "Alpha body"}, {ChunkID: "b", Title: "论文 Beta", Content: "Beta body"}, {ChunkID: "x", Title: "论文 Other", Content: "UNSEEN_OTHER"}}})
	lexical, _ := json.Marshal(knowledgeSearchPayload{Hits: []EvidenceHit{{ChunkID: "l", Title: "论文 Alpha", Content: "UNSEEN_LEXICAL"}}})
	obs, _ := prepareAnswerEvidence([]byte("两篇各自讲什么"), []Observation{{Name: "knowledge.catalog", Output: catalog}, {Name: "knowledge.read", Output: read}, {Name: "knowledge.lexical", Output: lexical}})
	hits := CollectEvidence(obs).Hits
	if len(hits) != 2 {
		t.Fatalf("gate saw unexposed evidence: %+v", hits)
	}
	for _, h := range hits {
		if strings.Contains(h.Content, "UNSEEN") {
			t.Fatal("unexposed evidence leaked")
		}
	}
}

type packetCaptureStreamer struct{ prompt string }

func (s *packetCaptureStreamer) StreamTokens(_ context.Context, _, user string, emit func(string) error) error {
	s.prompt = user
	return emit("正文中的候选答案。")
}

type packetCaptureValidator struct {
	prompt *packetCaptureStreamer
	t      *testing.T
}

func (v packetCaptureValidator) Validate(_ context.Context, answer string, obs []Observation) (ValidationResult, error) {
	for _, o := range obs {
		if !strings.Contains(v.prompt.prompt, string(o.Output)) {
			v.t.Fatal("validator received evidence absent from generation")
		}
	}
	return ValidationResult{Answer: answer}, nil
}
func TestRuntimeSharesPacketAndKeepsConfigOutOfQuestion(t *testing.T) {
	raw, _ := json.Marshal(knowledgeSearchPayload{Hits: []EvidenceHit{{ChunkID: "c", DocID: "d", Content: strings.Repeat("正文。", 6000) + "UNEXPOSED_TAIL"}}})
	streamer := &packetCaptureStreamer{}
	sink := &sinkStub{}
	rt := NewRuntime(staticPlanner{Decision{Decision: DecisionFinal}}, nil, sink).WithAnswerStreamer(streamer).WithAnswerValidator(packetCaptureValidator{streamer, t})
	_, err := rt.Run(context.Background(), Request{Prompt: []byte("legacy context"), UserQuestion: "第一行问题\n第二行问题", InputConfig: InputConfig{TopK: 7, ChatMode: "hybrid"}, SeedObservations: []Observation{{Name: "knowledge.read", Output: raw}}, Limits: Limits{MaxSteps: 1, Deadline: time.Now().Add(time.Second)}})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(streamer.prompt, "第一行问题\n第二行问题") || strings.Contains(streamer.prompt, "top_k=7") || strings.Contains(streamer.prompt, "UNEXPOSED_TAIL") {
		t.Fatal("question/config/evidence boundary violated")
	}
	packetHash, gateHash := "", ""
	for _, ev := range sink.events {
		var m map[string]any
		if json.Unmarshal([]byte(ev.Message), &m) != nil {
			continue
		}
		if ev.Type == "answer.evidence.prepared" {
			packetHash, _ = m["evidence_packet_sha256"].(string)
		}
		if ev.Type == "answer.validated" {
			gateHash, _ = m["evidence_packet_sha256"].(string)
		}
	}
	if packetHash == "" || packetHash != gateHash {
		t.Fatal("audit and validator packet differ")
	}
}

type overviewPacketCapture struct{ hits []EvidenceHit }

func (s *overviewPacketCapture) StreamTokens(_ context.Context, _, user string, emit func(string) error) error {
	var payload struct {
		Evidence []EvidenceHit `json:"evidence"`
	}
	if err := json.Unmarshal([]byte(user), &payload); err != nil {
		return err
	}
	s.hits = append(s.hits, payload.Evidence...)
	return emit("该资料介绍独立实验方法及观测结果。")
}

type overviewPacketValidator struct {
	streamer *overviewPacketCapture
	t        *testing.T
}

func (v overviewPacketValidator) Validate(_ context.Context, answer string, obs []Observation) (ValidationResult, error) {
	supplied := map[string]string{}
	for _, h := range v.streamer.hits {
		supplied[h.ChunkID] = h.Content
	}
	hits := CollectEvidence(obs).Hits
	if len(hits) != len(supplied) {
		v.t.Fatalf("overview evidence mismatch: gate=%d supplied=%d", len(hits), len(supplied))
	}
	for _, h := range hits {
		if supplied[h.ChunkID] != h.Content {
			v.t.Fatal("overview validator saw unexposed content")
		}
	}
	return ValidationResult{Answer: answer}, nil
}
func TestRuntimeOverviewSharesExactlyTheGroupEvidence(t *testing.T) {
	streamer := &overviewPacketCapture{}
	catalog := json.RawMessage(`{"total":2,"documents":[{"title":"论文 Alpha"},{"title":"论文 Beta"}]}`)
	raw, _ := json.Marshal(knowledgeSearchPayload{Hits: []EvidenceHit{{ChunkID: "a", Title: "论文 Alpha", Content: strings.Repeat("独立实验背景。", 4000) + "HIDDEN_TAIL"}, {ChunkID: "b", Title: "论文 Beta", Content: "Beta independent method."}, {ChunkID: "x", Title: "论文 Other", Content: "UNUSED_SOURCE"}}})
	rt := NewRuntime(staticPlanner{Decision{Decision: DecisionFinal}}, nil, &sinkStub{}).WithAnswerStreamer(streamer).WithAnswerValidator(overviewPacketValidator{streamer, t})
	out, err := rt.Run(context.Background(), Request{UserQuestion: "两篇各自讲什么", SeedObservations: []Observation{{Name: "knowledge.catalog", Output: catalog}, {Name: "knowledge.read", Output: raw}}, Limits: Limits{MaxSteps: 1, Deadline: time.Now().Add(time.Second)}})
	if err != nil || out.State != StateCompleted {
		t.Fatalf("%+v %v", out, err)
	}
	for _, h := range streamer.hits {
		if strings.Contains(h.Content, "HIDDEN_TAIL") || strings.Contains(h.Content, "UNUSED_SOURCE") {
			t.Fatal("overview exceeded packet")
		}
	}
}

func TestContextCannotInvalidatePacketOrErasePlannerObservations(t *testing.T) {
	contextText := strings.Repeat("context line\n\"quote\" ", 400)
	raw := []Observation{{Name: "workspace.context", Output: []byte(contextText)}, {Name: "knowledge.read", Output: json.RawMessage(`{"hits":[{"id":"c","content":"KNOWN_EVIDENCE"}]}`)}}
	plannerPrompt := buildPlannerPrompt(PlannerInput{Prompt: []byte("actual question"), Observations: raw})
	if !strings.Contains(plannerPrompt, "KNOWN_EVIDENCE") {
		t.Fatal("context JSON erased all observations")
	}
	obs, hash := prepareAnswerEvidence([]byte("actual question"), raw)
	encoded, err := json.Marshal(obs)
	if err != nil || len(encoded) == 0 || len(hash) != 64 {
		t.Fatalf("packet JSON: %v", err)
	}
	if len(CollectEvidence(obs).Hits) != 1 {
		t.Fatal("context became evidence or evidence disappeared")
	}
}

type structuredInputPlanner struct {
	t        *testing.T
	question string
}

func (p structuredInputPlanner) Decide(_ context.Context, in PlannerInput) (Decision, error) {
	if CurrentUserQuery(string(in.Prompt)) != p.question || in.TopK != 7 {
		p.t.Fatalf("planner input lost structure: %+v", in)
	}
	foundHistory, foundWorkspace := false, false
	for _, o := range in.Observations {
		if !json.Valid(o.Output) {
			p.t.Fatal("invalid context JSON")
		}
		if o.Name == "memory.history" {
			foundHistory = true
		}
		if o.Name == "workspace.context" {
			foundWorkspace = true
		}
	}
	if !foundHistory || !foundWorkspace {
		p.t.Fatal("history/workspace context disappeared")
	}
	return Decision{Decision: DecisionFinal, Answer: "ok"}, nil
}
func TestLauncherSeparatesHistoryWorkspaceAndLiteralQuestion(t *testing.T) {
	question := "top_k=5\n这是用户原文\n保留全部行"
	store := &notifyingStore{done: make(chan State, 2)}
	l := &Launcher{Store: store, Planner: structuredInputPlanner{t, question}, Memory: func(RunRecord) ShortTermMemory {
		return memoryStub{msgs: []MemoryMessage{{Role: "user", Content: "old question"}}}
	}}
	_, err := l.Run(context.Background(), RunRecord{RunID: "structured", TenantID: "t", ConversationID: "conversation", Prompt: EncodeRunInput(question, InputConfig{TopK: 7, WorkspaceContext: "background"}), Deadline: time.Now().Add(time.Second)}, Limits{MaxSteps: 1})
	if err != nil {
		t.Fatal(err)
	}
}
