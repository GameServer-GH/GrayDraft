package nexus

import (
	"bufio"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
)

// ChatStreamRequest is the BFF-facing chat body (mapped onto Agent v2 Run).
type ChatStreamRequest struct {
	Query            string          `json:"query"`
	TenantID         string          `json:"tenant_id,omitempty"`
	KBID             string          `json:"kb_id,omitempty"`
	KBType           string          `json:"kb_type,omitempty"`
	TopK             int             `json:"top_k,omitempty"`
	SessionID        string          `json:"session_id,omitempty"`
	ChatMode         string          `json:"chat_mode,omitempty"`
	SkipCache        bool            `json:"skip_cache,omitempty"`
	SkipAnswerCache  bool            `json:"skip_answer_cache,omitempty"`
	IncludeCold      bool            `json:"include_cold,omitempty"`
	LLMOverride      *LLMOverride    `json:"llm_override,omitempty"`
	ThinkingMode     *bool           `json:"thinking_mode,omitempty"`
	WorkspaceContext string          `json:"workspace_context,omitempty"`
	ContextProfile   *ContextProfile `json:"context_profile,omitempty"`
}

type createAgentRunRequest struct {
	AgentID         string `json:"agent_id"`
	ConversationID  string `json:"conversation_id,omitempty"`
	PolicyVersion   string `json:"policy_version"`
	IdempotencyKey  string `json:"idempotency_key"`
	DeadlineSeconds int    `json:"deadline_seconds,omitempty"`
	Prompt          string `json:"prompt"`
	ThinkingMode    *bool  `json:"thinking_mode,omitempty"`
}

type createAgentRunResponse struct {
	RunID  string `json:"run_id"`
	Status string `json:"status"`
}

type agentRunEvent struct {
	RunID    string `json:"run_id"`
	TenantID string `json:"tenant_id"`
	Seq      int64  `json:"seq"`
	Type     string `json:"type"`
	Payload  []byte `json:"payload"`
}

type agentEventPayload struct {
	Message  string `json:"message"`
	State    string `json:"state"`
	ToolName string `json:"tool_name"`
	TraceID  string `json:"trace_id"`
	StepID   int    `json:"step_id"`
	CallID   string `json:"call_id"`
}

type agentGateDetail struct {
	AnswerConfidence float64              `json:"answer_confidence"`
	RetrievalGrade   string               `json:"retrieval_grade,omitempty"`
	CoverageScore    float64              `json:"coverage_score,omitempty"`
	HighRisk         bool                 `json:"high_risk,omitempty"`
	SentenceLabels   []sentenceLabel      `json:"sentence_labels,omitempty"`
	HitSummaries     []evidenceHitSummary `json:"hit_summaries,omitempty"`
}

type sentenceLabel struct {
	Sentence string `json:"sentence"`
	Source   string `json:"source"`
	Warning  string `json:"warning,omitempty"`
}

type evidenceHitSummary struct {
	ChunkID  string `json:"chunk_id,omitempty"`
	DocID    string `json:"doc_id,omitempty"`
	Title    string `json:"title,omitempty"`
	MediaURL string `json:"media_url,omitempty"`
}

// legacySSEEvent is the frontend-compatible SSE payload.
type legacySSEEvent struct {
	Type             string           `json:"type"`
	Delta            string           `json:"delta,omitempty"`
	Message          string           `json:"message,omitempty"`
	Done             bool             `json:"done,omitempty"`
	Strategy         string           `json:"strategy,omitempty"`
	ChatMode         string           `json:"chat_mode,omitempty"`
	RetrievalGrade   string           `json:"retrieval_grade,omitempty"`
	CoverageScore    float64          `json:"coverage_score,omitempty"`
	AnswerConfidence float64          `json:"answer_confidence,omitempty"`
	Confidence       float64          `json:"confidence,omitempty"`
	ChunksUsed       int              `json:"chunks_used,omitempty"`
	ThinkingPhase    string           `json:"thinking_phase,omitempty"`
	ToolName         string           `json:"tool_name,omitempty"`
	StepID           int              `json:"step_id,omitempty"`
	Sources          []legacySource   `json:"sources,omitempty"`
	Citations        []legacyCitation `json:"citations,omitempty"`
	SentenceLabels   []sentenceLabel  `json:"sentence_labels,omitempty"`
}

type legacySource struct {
	DocID    string `json:"doc_id"`
	DocTitle string `json:"doc_title,omitempty"`
}

type legacyCitation struct {
	DocID    string   `json:"doc_id"`
	DocTitle string   `json:"doc_title,omitempty"`
	KBType   string   `json:"kb_type,omitempty"`
	ChunkIDs []string `json:"chunk_ids,omitempty"`
	MediaURL string   `json:"media_url,omitempty"`
}

// ProxyChatStream forwards Agent v2 SSE (mapped to legacy meta/thinking/chunk/done) to the response writer.
func (c *Client) ProxyChatStream(ctx context.Context, req ChatStreamRequest, w http.ResponseWriter) error {
	return c.ProxyChatStreamWithHook(ctx, req, w, nil)
}

// ProxyChatStreamWithHook creates an Agent Run and maps v2 events to legacy SSE lines.
func (c *Client) ProxyChatStreamWithHook(ctx context.Context, req ChatStreamRequest, w http.ResponseWriter, onLine func(string)) error {
	if c == nil {
		return fmt.Errorf("nexus client not configured")
	}
	flusher, ok := w.(http.Flusher)
	if !ok {
		return fmt.Errorf("streaming not supported")
	}
	h := w.Header()
	h.Set("Content-Type", "text/event-stream; charset=utf-8")
	h.Set("Cache-Control", "no-cache")
	h.Set("Connection", "keep-alive")
	h.Set("X-Accel-Buffering", "no")
	w.WriteHeader(http.StatusOK)

	writeEvent := func(ev legacySSEEvent) error {
		b, err := json.Marshal(ev)
		if err != nil {
			return err
		}
		line := "data: " + string(b)
		if onLine != nil {
			onLine(line)
		}
		if _, err := fmt.Fprintf(w, "%s\n\n", line); err != nil {
			return err
		}
		flusher.Flush()
		return nil
	}

	runID, err := c.createAgentRun(ctx, req)
	if err != nil {
		_ = writeEvent(legacySSEEvent{Type: "error", Message: err.Error()})
		return err
	}
	return c.streamAgentRunEvents(ctx, req, runID, writeEvent)
}

// ChatCompletion runs Agent v2 and returns the concatenated answer text.
func (c *Client) ChatCompletion(ctx context.Context, req ChatStreamRequest) (string, error) {
	if c == nil {
		return "", fmt.Errorf("nexus client not configured")
	}
	runID, err := c.createAgentRun(ctx, req)
	if err != nil {
		return "", err
	}
	var full strings.Builder
	err = c.streamAgentRunEvents(ctx, req, runID, func(ev legacySSEEvent) error {
		if ev.Type == "chunk" && ev.Delta != "" {
			full.WriteString(ev.Delta)
		}
		if ev.Type == "error" {
			return fmt.Errorf("%s", ev.Message)
		}
		return nil
	})
	return strings.TrimSpace(full.String()), err
}

func (c *Client) createAgentRun(ctx context.Context, req ChatStreamRequest) (string, error) {
	body, err := json.Marshal(createAgentRunRequest{
		AgentID:         c.agentID(),
		ConversationID:  normalizeConversationID(req.SessionID),
		PolicyVersion:   c.agentPolicyVersion(),
		IdempotencyKey:  newIdempotencyKey(req.SessionID),
		DeadlineSeconds: 90,
		Prompt:          buildAgentRunPrompt(req),
		ThinkingMode:    req.ThinkingMode,
	})
	if err != nil {
		return "", fmt.Errorf("nexus: marshal agent run: %w", err)
	}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.base()+"/api/v2/agent-runs", strings.NewReader(string(body)))
	if err != nil {
		return "", err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	if err := c.applyAgentHeaders(httpReq, req); err != nil {
		return "", err
	}
	applyTraceHeaders(httpReq, ctx)

	res, err := c.http.Do(httpReq)
	if err != nil {
		return "", fmt.Errorf("nexus: agent run request: %w", err)
	}
	defer res.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(res.Body, 64*1024))
	if res.StatusCode != http.StatusAccepted && res.StatusCode != http.StatusOK {
		return "", fmt.Errorf("nexus agent run %d: %s", res.StatusCode, strings.TrimSpace(string(raw)))
	}
	var out createAgentRunResponse
	if err := json.Unmarshal(raw, &out); err != nil {
		return "", fmt.Errorf("nexus: parse agent run: %w", err)
	}
	if out.RunID == "" {
		return "", fmt.Errorf("nexus: agent run missing run_id")
	}
	return out.RunID, nil
}

func buildAgentRunPrompt(req ChatStreamRequest) string {
	var b strings.Builder
	if req.TopK > 0 {
		fmt.Fprintf(&b, "top_k=%d\n", req.TopK)
	}
	if mode := strings.TrimSpace(req.ChatMode); mode != "" {
		fmt.Fprintf(&b, "chat_mode=%s\n", mode)
	}
	if wc := strings.TrimSpace(req.WorkspaceContext); wc != "" {
		b.WriteString("workspace_context:\n")
		b.WriteString(wc)
		b.WriteString("\n")
	}
	b.WriteString(req.Query)
	return b.String()
}

func (c *Client) streamAgentRunEvents(ctx context.Context, req ChatStreamRequest, runID string, emit func(legacySSEEvent) error) error {
	url := c.base() + "/api/v2/agent-runs/" + runID + "/events/stream?after_seq=0"
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	httpReq.Header.Set("Accept", "text/event-stream")
	if err := c.applyAgentHeaders(httpReq, req); err != nil {
		return err
	}
	applyTraceHeaders(httpReq, ctx)

	client := c.httpSlow
	if client == nil {
		client = c.http
	}
	res, err := client.Do(httpReq)
	if err != nil {
		return fmt.Errorf("nexus: agent stream: %w", err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		raw, _ := io.ReadAll(io.LimitReader(res.Body, 4096))
		return fmt.Errorf("nexus agent stream %d: %s", res.StatusCode, strings.TrimSpace(string(raw)))
	}

	thinkingMode := true
	if req.ThinkingMode != nil {
		thinkingMode = *req.ThinkingMode
	}
	mapper := newAgentEventMapper(req.KBType, req.ChatMode, thinkingMode, emit)
	sc := bufio.NewScanner(res.Body)
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for sc.Scan() {
		line := sc.Text()
		if !strings.HasPrefix(line, "data: ") {
			continue
		}
		payload := strings.TrimPrefix(line, "data: ")
		var ev agentRunEvent
		if err := json.Unmarshal([]byte(payload), &ev); err != nil {
			continue
		}
		if done, streamErr := mapper.handle(ev); done {
			return streamErr
		}
	}
	return sc.Err()
}

func (c *Client) applyAgentHeaders(httpReq *http.Request, req ChatStreamRequest) error {
	httpReq.Header.Set("X-Tenant-ID", strings.TrimSpace(req.TenantID))
	httpReq.Header.Set("X-Actor-ID", c.agentActorID())
	kb := strings.TrimSpace(req.KBType)
	if kb == "" {
		kb = strings.TrimSpace(req.KBID)
	}
	if kb != "" {
		httpReq.Header.Set("X-Authorized-KBs", kb)
	}
	if c.tokens != nil {
		token, err := c.tokens.Bearer()
		if err != nil {
			return fmt.Errorf("nexus: agent token: %w", err)
		}
		if token != "" {
			httpReq.Header.Set("Authorization", "Bearer "+token)
		}
	}
	return nil
}

func (c *Client) agentID() string {
	if c != nil && strings.TrimSpace(c.cfg.AgentID) != "" {
		return strings.TrimSpace(c.cfg.AgentID)
	}
	return "edu-tutor"
}

func (c *Client) agentPolicyVersion() string {
	if c != nil && strings.TrimSpace(c.cfg.AgentPolicyVersion) != "" {
		return strings.TrimSpace(c.cfg.AgentPolicyVersion)
	}
	return "edu_workspace_v1"
}

func (c *Client) agentActorID() string {
	if c != nil && strings.TrimSpace(c.cfg.AgentActorID) != "" {
		return strings.TrimSpace(c.cfg.AgentActorID)
	}
	return "graydraft-bff"
}

func newIdempotencyKey(sessionID string) string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	prefix := strings.TrimSpace(sessionID)
	if prefix == "" {
		prefix = "gd"
	}
	return fmt.Sprintf("graydraft:%s:%d:%s", prefix, time.Now().UnixNano(), hex.EncodeToString(b))
}

// conversationIDNamespace matches core agent.NormalizeConversationID.
var conversationIDNamespace = uuid.MustParse("7c9e6679-7425-40de-944b-e07fc1f90ae7")

func normalizeConversationID(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	if _, err := uuid.Parse(raw); err == nil {
		return raw
	}
	return uuid.NewSHA1(conversationIDNamespace, []byte(raw)).String()
}

type agentEventMapper struct {
	kbType         string
	chatMode       string
	thinkingMode   bool
	emit           func(legacySSEEvent) error
	metaSent       bool
	gradeSent      bool
	streamedAnswer bool
	gate           *agentGateDetail
}

func newAgentEventMapper(kbType, chatMode string, thinkingMode bool, emit func(legacySSEEvent) error) *agentEventMapper {
	return &agentEventMapper{kbType: kbType, chatMode: chatMode, thinkingMode: thinkingMode, emit: emit}
}

func (m *agentEventMapper) handle(ev agentRunEvent) (done bool, err error) {
	var p agentEventPayload
	if len(ev.Payload) > 0 {
		_ = json.Unmarshal(ev.Payload, &p)
	}
	switch ev.Type {
	case "run.started":
		if !m.metaSent {
			m.metaSent = true
			if err := m.emit(legacySSEEvent{Type: "meta", Strategy: "agent_v2", ChatMode: m.chatModeOrDefault()}); err != nil {
				return true, err
			}
		}
		return false, m.emitThinking("run", "已接入智能体，正在准备回答…", "", 0)
	case "step.started":
		step := p.StepID
		if step <= 0 {
			step = 1
		}
		return false, m.emitThinking("step", fmt.Sprintf("第 %d 步：分析问题并规划下一步…", step), "", step)
	case "tool.started":
		name := toolDisplayName(p.ToolName)
		return false, m.emitThinking("tool", fmt.Sprintf("正在调用「%s」…", name), p.ToolName, p.StepID)
	case "tool.completed":
		name := toolDisplayName(p.ToolName)
		msg := fmt.Sprintf("「%s」完成", name)
		if strings.TrimSpace(p.Message) != "" {
			msg = fmt.Sprintf("「%s」未完全成功，继续推理…", name)
		}
		return false, m.emitThinking("tool", msg, p.ToolName, p.StepID)
	case "tool.approval_required":
		name := toolDisplayName(p.ToolName)
		return false, m.emitThinking("tool", fmt.Sprintf("「%s」需要人工审批，请稍候…", name), p.ToolName, p.StepID)
	case "answer.generating":
		msg := strings.TrimSpace(p.Message)
		if msg == "" {
			msg = "正在生成回答…"
		}
		return false, m.emitThinking("answer", msg, "", 0)
	case "planner.thought":
		msg := strings.TrimSpace(p.Message)
		if msg == "" {
			return false, nil
		}
		return false, m.emitThinking("plan", msg, "", p.StepID)
	case "llm.reasoning.delta":
		// Raw model CoT stays in agent event audit only — never forward to end users.
		return false, nil
	case "answer.delta":
		if p.Message == "" {
			return false, nil
		}
		m.streamedAnswer = true
		return false, m.emit(legacySSEEvent{Type: "chunk", Delta: p.Message})
	case "answer.validated", "answer.rejected":
		var gate agentGateDetail
		if json.Unmarshal([]byte(p.Message), &gate) == nil {
			m.gate = &gate
			if !m.gradeSent && gate.RetrievalGrade != "" {
				m.gradeSent = true
				if err := m.emit(legacySSEEvent{
					Type: "meta", Strategy: "agent_v2", ChatMode: m.chatModeOrDefault(),
					RetrievalGrade: gate.RetrievalGrade, CoverageScore: gate.CoverageScore,
				}); err != nil {
					return true, err
				}
			}
		}
		// Only surface validate thinking when the answer was actually refused/replaced.
		if ev.Type == "answer.rejected" {
			return false, m.emitThinking("validate", "答案未通过可信度门禁，生成安全回复…", "", 0)
		}
		return false, nil
	case "run.completed":
		answer := strings.TrimSpace(p.Message)
		if answer != "" && !m.streamedAnswer {
			if err := m.emit(legacySSEEvent{Type: "chunk", Delta: answer}); err != nil {
				return true, err
			}
		}
		doneEv := legacySSEEvent{
			Type: "done", Done: true, Message: answer, ChatMode: m.chatModeOrDefault(),
		}
		if m.gate != nil {
			doneEv.AnswerConfidence = m.gate.AnswerConfidence
			doneEv.Confidence = m.gate.AnswerConfidence
			doneEv.RetrievalGrade = m.gate.RetrievalGrade
			doneEv.CoverageScore = m.gate.CoverageScore
			doneEv.Citations = gateCitations(m.kbType, m.gate.HitSummaries)
			doneEv.Sources = gateSources(m.gate.HitSummaries)
			doneEv.SentenceLabels = m.gate.SentenceLabels
			doneEv.ChunksUsed = len(m.gate.HitSummaries)
		}
		return true, m.emit(doneEv)
	case "run.failed", "run.cancelled", "run.expired", "run.no_progress", "run.invalid_decision",
		"run.budget_exceeded", "run.tool_budget_exceeded":
		msg := strings.TrimSpace(p.Message)
		if msg == "" {
			msg = ev.Type
		}
		_ = m.emit(legacySSEEvent{Type: "error", Message: msg})
		return true, fmt.Errorf("nexus: agent run %s: %s", ev.Type, msg)
	}
	return false, nil
}

func (m *agentEventMapper) emitThinking(phase, message, toolName string, stepID int) error {
	if !m.thinkingMode {
		return nil
	}
	return m.emit(legacySSEEvent{
		Type: "thinking", Message: message, ThinkingPhase: phase, ToolName: toolName, StepID: stepID,
	})
}

func (m *agentEventMapper) chatModeOrDefault() string {
	if strings.TrimSpace(m.chatMode) != "" {
		return m.chatMode
	}
	return "hybrid"
}

func toolDisplayName(name string) string {
	switch strings.TrimSpace(name) {
	case "knowledge.lexical":
		return "词法检索"
	case "knowledge.semantic":
		return "语义检索"
	case "knowledge.read":
		return "阅读相邻章节"
	case "knowledge.expand":
		return "沿实体扩展"
	case "note.draft":
		return "保存学习笔记"
	default:
		if name == "" {
			return "工具"
		}
		return name
	}
}

func gateCitations(kbType string, hits []evidenceHitSummary) []legacyCitation {
	if len(hits) == 0 {
		return nil
	}
	out := make([]legacyCitation, 0, len(hits))
	seen := map[string]bool{}
	for _, h := range hits {
		docID := h.DocID
		if docID == "" {
			docID = h.Title
		}
		if docID == "" && h.ChunkID == "" {
			continue
		}
		key := h.ChunkID
		if key == "" {
			key = docID
		}
		if seen[key] {
			continue
		}
		seen[key] = true
		c := legacyCitation{DocID: docID, DocTitle: h.Title, KBType: kbType, MediaURL: h.MediaURL}
		if h.ChunkID != "" {
			c.ChunkIDs = []string{h.ChunkID}
		}
		out = append(out, c)
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func gateSources(hits []evidenceHitSummary) []legacySource {
	if len(hits) == 0 {
		return nil
	}
	out := make([]legacySource, 0, len(hits))
	seen := map[string]bool{}
	for _, h := range hits {
		key := h.ChunkID
		if key == "" {
			key = h.DocID + ":" + h.Title
		}
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, legacySource{DocID: h.DocID, DocTitle: h.Title})
	}
	return out
}

// isClientDisconnect reports whether the error is caused by the client
// closing the connection (broken pipe, connection reset). These should
// be treated as clean exits, not server-side errors.
func isClientDisconnect(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, "broken pipe") || strings.Contains(msg, "connection reset") || strings.Contains(msg, "EOF")
}
