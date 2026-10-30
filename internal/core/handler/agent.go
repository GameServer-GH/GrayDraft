package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/graydraft/core/observability/trace"
	"github.com/graydraft/core/service/agent"
)

// AgentHandler exposes the P0 Run/Event control plane.
type AgentRunStore interface {
	CreateRun(context.Context, agent.RunRecord) (agent.RunRecord, error)
	AppendEvent(context.Context, agent.RunEvent) error
	ListEvents(context.Context, string, string, int64, int64) ([]agent.RunEvent, error)
	CancelRun(context.Context, string, string) error
	UpdateStatus(context.Context, string, agent.State, int, int) error
}

type RunLauncher interface {
	Launch(agent.RunRecord, agent.Limits) error
}

// RunEventWatcher subscribes to live Run event notifications (NATS pub/sub).
type RunEventWatcher interface {
	SubscribeRunEvents(runID string, onSeq func(int64)) (unsubscribe func(), err error)
	Connected() bool
}

type AgentHandler struct {
	runs     AgentRunStore
	launcher RunLauncher
	events   RunEventWatcher
	shadow   *ShadowRunConfig
	budget   *agent.GlobalBudgetGuard
}

type ShadowRunConfig struct {
	Controller      *agent.CanaryController
	CandidatePolicy string
}

func NewAgentHandler(runs AgentRunStore) *AgentHandler { return &AgentHandler{runs: runs} }
func (h *AgentHandler) WithLauncher(launcher RunLauncher) *AgentHandler {
	h.launcher = launcher
	return h
}
func (h *AgentHandler) WithEventWatcher(w RunEventWatcher) *AgentHandler {
	h.events = w
	return h
}
func (h *AgentHandler) WithShadow(cfg *ShadowRunConfig) *AgentHandler { h.shadow = cfg; return h }
func (h *AgentHandler) WithBudget(g *agent.GlobalBudgetGuard) *AgentHandler {
	h.budget = g
	return h
}

type createAgentRunRequest struct {
	AgentID         string   `json:"agent_id" binding:"required"`
	ConversationID  string   `json:"conversation_id,omitempty"`
	PolicyVersion   string   `json:"policy_version" binding:"required"`
	IdempotencyKey  string   `json:"idempotency_key" binding:"required"`
	DeadlineSeconds int      `json:"deadline_seconds,omitempty"`
	Prompt          string   `json:"prompt" binding:"required"`
	ThinkingMode    *bool    `json:"thinking_mode,omitempty"`
	Collaborative   bool     `json:"collaborative,omitempty"`
	SubAgents       []string `json:"sub_agents,omitempty"`
}

func (h *AgentHandler) tenant(c *gin.Context) (string, string, bool) {
	tenant, actor := strings.TrimSpace(c.GetHeader("X-Tenant-ID")), strings.TrimSpace(c.GetHeader("X-Actor-ID"))
	if v, ok := c.Get("agent_tenant_id"); ok {
		tenant, _ = v.(string)
	}
	if v, ok := c.Get("agent_actor_id"); ok {
		actor, _ = v.(string)
	}
	if tenant == "" || actor == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "X-Tenant-ID and X-Actor-ID are required"})
		return "", "", false
	}
	return tenant, actor, true
}

func authorizedKBs(c *gin.Context) []string {
	raw := strings.Split(c.GetHeader("X-Authorized-KBs"), ",")
	var out []string
	for _, v := range raw {
		if v = strings.TrimSpace(v); v != "" {
			out = append(out, v)
		}
	}
	return out
}

func (h *AgentHandler) CreateRun(c *gin.Context) {
	if h == nil || h.runs == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "agent runtime unavailable"})
		return
	}
	if h.launcher == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "agent jetstream worker required"})
		return
	}
	tenant, actor, ok := h.tenant(c)
	if !ok {
		return
	}
	var req createAgentRunRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if h.budget != nil {
		if err := h.budget.Check(c.Request.Context(), tenant, req.Collaborative); err != nil {
			c.JSON(http.StatusTooManyRequests, gin.H{"error": err.Error()})
			return
		}
	}
	traceID := trace.FromContext(c.Request.Context())
	deadline := time.Now().Add(90 * time.Second)
	if req.DeadlineSeconds > 0 && req.DeadlineSeconds < 300 {
		deadline = time.Now().Add(time.Duration(req.DeadlineSeconds) * time.Second)
	}
	prompt := req.Prompt
	req.PolicyVersion = agent.EduPolicyVersion
	run, err := h.runs.CreateRun(c.Request.Context(), agent.RunRecord{
		RunID: uuid.NewString(), TenantID: tenant, ActorID: actor, AgentID: req.AgentID,
		ConversationID: agent.NormalizeConversationID(req.ConversationID), PolicyVersion: req.PolicyVersion, Deadline: deadline,
		IdempotencyKey: req.IdempotencyKey, TraceID: traceID, Prompt: []byte(prompt), AuthorizedKBs: authorizedKBs(c),
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if traceID == "" {
		traceID = run.TraceID
	}
	_ = h.runs.AppendEvent(c.Request.Context(), agent.RunEvent{
		RunID: run.RunID, TenantID: tenant, Seq: 1, Type: "run.started",
		Payload: []byte(`{"status":"queued","trace_id":"` + traceID + `"}`),
	})
	if launchErr := h.launcher.Launch(run, agent.Limits{Deadline: deadline}); launchErr != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": launchErr.Error(), "run_id": run.RunID, "trace_id": traceID})
		return
	}
	if h.shadow != nil && h.shadow.Controller != nil && h.shadow.CandidatePolicy != "" && h.shadow.Controller.ShouldSample(run.RunID) {
		shadowRun := run
		shadowRun.RunID = uuid.NewString()
		shadowRun.PolicyVersion = h.shadow.CandidatePolicy
		shadowRun.IdempotencyKey = run.IdempotencyKey + ":shadow:" + h.shadow.CandidatePolicy
		if created, shadowErr := h.runs.CreateRun(c.Request.Context(), shadowRun); shadowErr == nil {
			_ = h.runs.AppendEvent(c.Request.Context(), agent.RunEvent{
				RunID: created.RunID, TenantID: tenant, Seq: 1, Type: "run.shadow_started",
				Payload: []byte(`{"shadow_of":"` + run.RunID + `"}`),
			})
			_ = h.launcher.Launch(created, agent.Limits{Deadline: deadline})
		}
	}
	c.JSON(http.StatusAccepted, gin.H{
		"run_id": run.RunID, "trace_id": traceID, "status": run.Status,
		"policy_version": run.PolicyVersion, "deadline_at": run.Deadline,
	})
}

func (h *AgentHandler) Events(c *gin.Context) {
	if h == nil || h.runs == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "agent runtime unavailable"})
		return
	}
	tenant, _, ok := h.tenant(c)
	if !ok {
		return
	}
	after, _ := strconv.ParseInt(c.DefaultQuery("after_seq", "0"), 10, 64)
	events, err := h.runs.ListEvents(c.Request.Context(), tenant, c.Param("run_id"), after, 200)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"events": events, "after_seq": after})
}

func (h *AgentHandler) Trace(c *gin.Context) {
	if h == nil || h.runs == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "agent runtime unavailable"})
		return
	}
	tenant, _, ok := h.tenant(c)
	if !ok {
		return
	}
	runID := c.Param("run_id")
	events, err := h.runs.ListEvents(c.Request.Context(), tenant, runID, 0, 5000)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	policyVersion := c.Query("policy_version")
	traceView := agent.BuildRunTrace(runID, tenant, policyVersion, events)
	c.JSON(http.StatusOK, traceView)
}

// EventsStream replays from after_seq then wakes on NATS pub/sub for live events.
func (h *AgentHandler) EventsStream(c *gin.Context) {
	if h == nil || h.runs == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "agent runtime unavailable"})
		return
	}
	tenant, _, ok := h.tenant(c)
	if !ok {
		return
	}
	runID := c.Param("run_id")
	after, _ := strconv.ParseInt(c.DefaultQuery("after_seq", "0"), 10, 64)
	lastSeq := after

	c.Header("Content-Type", "text/event-stream; charset=utf-8")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	flusher, ok := c.Writer.(http.Flusher)
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "streaming not supported"})
		return
	}

	writeEvents := func(events []agent.RunEvent) {
		for _, ev := range events {
			data, _ := json.Marshal(ev)
			_, _ = c.Writer.Write([]byte("id: " + strconv.FormatInt(ev.Seq, 10) + "\ndata: "))
			_, _ = c.Writer.Write(data)
			_, _ = c.Writer.Write([]byte("\n\n"))
			if ev.Seq > lastSeq {
				lastSeq = ev.Seq
			}
		}
		flusher.Flush()
	}

	events, err := h.runs.ListEvents(c.Request.Context(), tenant, runID, lastSeq, 200)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	writeEvents(events)

	notify := make(chan int64, 8)
	var unsubscribe func()
	if h.events != nil && h.events.Connected() {
		unsub, subErr := h.events.SubscribeRunEvents(runID, func(seq int64) {
			select {
			case notify <- seq:
			default:
			}
		})
		if subErr == nil {
			unsubscribe = unsub
			defer unsubscribe()
		}
	}

	ticker := time.NewTicker(400 * time.Millisecond)
	defer ticker.Stop()
	clientGone := c.Request.Context().Done()

	for {
		select {
		case <-clientGone:
			return
		case <-ticker.C:
			// Polling keeps SSE alive when pub/sub is unavailable.
			if batch, listErr := h.runs.ListEvents(c.Request.Context(), tenant, runID, lastSeq, 200); listErr == nil && len(batch) > 0 {
				writeEvents(batch)
				if isTerminalAgentEvent(batch[len(batch)-1].Type) {
					return
				}
			}
		case seq := <-notify:
			if seq <= lastSeq {
				continue
			}
			if batch, listErr := h.runs.ListEvents(c.Request.Context(), tenant, runID, lastSeq, 200); listErr == nil {
				writeEvents(batch)
				if len(batch) > 0 && isTerminalAgentEvent(batch[len(batch)-1].Type) {
					return
				}
			}
		}
	}
}

func isTerminalAgentEvent(eventType string) bool {
	switch eventType {
	case "run.completed", "run.failed", "run.cancelled", "run.expired",
		"run.budget_exceeded", "run.tool_budget_exceeded", "run.no_progress", "run.invalid_decision":
		return true
	default:
		return false
	}
}

func (h *AgentHandler) Cancel(c *gin.Context) {
	if h == nil || h.runs == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "agent runtime unavailable"})
		return
	}
	tenant, _, ok := h.tenant(c)
	if !ok {
		return
	}
	if err := h.runs.CancelRun(c.Request.Context(), tenant, c.Param("run_id")); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"run_id": c.Param("run_id"), "status": "cancelled"})
}
