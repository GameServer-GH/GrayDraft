// Package agent contains the provider-independent Agent Runtime.
//
// The runtime deliberately knows nothing about PostgreSQL, HTTP or a specific
// LLM provider. Those concerns are ports supplied by the application layer.
package agent

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"
)

type State string

const (
	StateQueued     State = "queued"
	StateDeciding   State = "deciding"
	StateValidating State = "validating_tool"
	StateExecuting  State = "executing_tool"
	StateObserving  State = "observing"
	StateFinalizing State = "finalizing"
	StateCompleted  State = "completed"
	StateFailed     State = "failed"
	StateCancelled  State = "cancelled"
	StateExpired    State = "expired"
)

type DecisionType string

const (
	DecisionFinal   DecisionType = "final"
	DecisionTools   DecisionType = "tool_calls"
	DecisionClarify DecisionType = "clarify"
)

// Limits are fixed per Run by AgentProfile, making execution deterministic.
type Limits struct {
	MaxSteps            int
	MaxToolCalls        int
	MaxSameActionRepeat int
	MaxNoProgressSteps  int
	Deadline            time.Time
}

func (l Limits) normalized() Limits {
	if l.MaxSteps <= 0 {
		l.MaxSteps = 8
	}
	if l.MaxToolCalls <= 0 {
		l.MaxToolCalls = 12
	}
	if l.MaxSameActionRepeat <= 0 {
		l.MaxSameActionRepeat = 2
	}
	if l.MaxNoProgressSteps <= 0 {
		l.MaxNoProgressSteps = 2
	}
	if l.Deadline.IsZero() {
		l.Deadline = time.Now().Add(60 * time.Second)
	}
	return l
}

type ToolCall struct {
	CallID    string          `json:"call_id"`
	Name      string          `json:"name"`
	Arguments json.RawMessage `json:"arguments"`
}

type Decision struct {
	Decision    DecisionType `json:"decision"`
	ThoughtNote string       `json:"thought_summary,omitempty"`
	ToolCalls   []ToolCall   `json:"tool_calls,omitempty"`
	Answer      string       `json:"answer,omitempty"`
	Confidence  float64      `json:"confidence,omitempty"`
}

type Observation struct {
	CallID, Name string
	Output       json.RawMessage
	Err          error
}

type Event struct {
	Seq          int64
	Type         string
	State        State
	CallID       string
	ToolName     string
	Message      string
	At           time.Time
	TraceID      string
	SpanID       string
	ParentSpanID string
	SpanKind     string
	SpanName     string
	StepID       int
	LatencyMS    int64
	SpanStatus   string
	SpanError    string
}

type PlannerInput struct {
	Prompt        []byte
	Step          int
	Observations  []Observation
	AuthorizedKBs []string
}

type Planner interface {
	Decide(context.Context, PlannerInput) (Decision, error)
}
type ToolExecutor interface {
	Execute(context.Context, ToolCall) (Observation, error)
}
type EventSink interface {
	Append(context.Context, Event) error
}
type AnswerValidator interface {
	Validate(context.Context, string, []Observation) (ValidationResult, error)
}

type ValidationResult struct {
	Answer   string
	Rejected bool
	Gate     AnswerGateResult
}

type Request struct {
	Prompt        []byte
	Limits        Limits
	AuthorizedKBs []string
	// DisableThinking hides planner reasoning events. The education loop, tools, and AnswerGate still run.
	DisableThinking bool
	// SeedObservations are injected before the first decide (e.g. long-term memory).
	// They must not be mistaken for the user prompt.
	SeedObservations []Observation
}

type Result struct {
	State        State
	Answer       string
	Steps        int
	ToolCalls    int
	Observations []Observation
}

type RunRecord struct {
	RunID          string    `json:"run_id"`
	TenantID       string    `json:"tenant_id"`
	ActorID        string    `json:"actor_id"`
	AgentID        string    `json:"agent_id"`
	ConversationID string    `json:"conversation_id,omitempty"`
	Status         State     `json:"status"`
	PolicyVersion  string    `json:"policy_version"`
	Deadline       time.Time `json:"deadline_at"`
	IdempotencyKey string    `json:"-"`
	TraceID        string    `json:"trace_id,omitempty"`
	// Prompt/AuthorizedKBs must serialize for JetStream workers (QueueLauncher).
	Prompt        []byte   `json:"prompt,omitempty"`
	AuthorizedKBs []string `json:"authorized_kbs,omitempty"`
	// DisableThinking is carried only through the runtime queue; it is not a DB column.
	DisableThinking bool `json:"disable_thinking,omitempty"`
}

type RunEvent struct {
	RunID      string    `json:"run_id"`
	TenantID   string    `json:"tenant_id"`
	Seq        int64     `json:"seq"`
	EventID    string    `json:"event_id"`
	Type       string    `json:"type"`
	Payload    []byte    `json:"payload"`
	OccurredAt time.Time `json:"occurred_at"`
}

type Runtime struct {
	planner   Planner
	tools     ToolExecutor
	sink      EventSink
	now       func() time.Time
	validator AnswerValidator
	streamer  TokenStreamer
}

func (r *Runtime) WithAnswerValidator(v AnswerValidator) *Runtime { r.validator = v; return r }
func (r *Runtime) WithAnswerStreamer(s TokenStreamer) *Runtime    { r.streamer = s; return r }

func NewRuntime(planner Planner, tools ToolExecutor, sink EventSink) *Runtime {
	return &Runtime{planner: planner, tools: tools, sink: sink, now: time.Now}
}

func (r *Runtime) emit(ctx context.Context, ev Event, seq *int64) error {
	*seq = *seq + 1
	ev.Seq = *seq
	if ev.At.IsZero() {
		ev.At = r.now()
	}
	if r.sink != nil {
		return r.sink.Append(ctx, ev)
	}
	return nil
}

func (r *Runtime) Run(ctx context.Context, req Request) (Result, error) {
	if len(req.Prompt) == 0 {
		return Result{State: StateFailed}, errors.New("agent: prompt is empty")
	}
	if r == nil || r.planner == nil {
		return Result{State: StateFailed}, errors.New("agent: planner is required")
	}
	limits := req.Limits.normalized()
	var out Result
	if len(req.SeedObservations) > 0 {
		out.Observations = append(out.Observations, req.SeedObservations...)
	}
	state, seq := StateQueued, int64(0)
	ctx, runSpan := StartAgentSpan(ctx, "run", SpanKindRun, 0, "", "")
	runStarted := Event{Type: "run.started", State: state}
	runSpan.ApplyEvent(&runStarted, nil)
	if err := r.emit(ctx, runStarted, &seq); err != nil {
		return out, err
	}
	var fingerprints []string
	var noProgress int
	for {
		if err := ctx.Err(); err != nil {
			out.State = StateCancelled
			_ = r.emit(ctx, Event{Type: "run.cancelled", State: out.State, Message: err.Error()}, &seq)
			return out, err
		}
		if r.now().After(limits.Deadline) {
			out.State = StateExpired
			_ = r.emit(ctx, Event{Type: "run.expired", State: out.State}, &seq)
			return out, context.DeadlineExceeded
		}
		if out.Steps >= limits.MaxSteps {
			out.State = StateExpired
			_ = r.emit(ctx, Event{Type: "run.budget_exceeded", State: out.State}, &seq)
			return out, fmt.Errorf("agent: max steps %d exceeded", limits.MaxSteps)
		}
		out.Steps++
		stepEv := Event{Type: "step.started", State: StateDeciding, StepID: out.Steps}
		stepID := out.Steps
		_ = r.emit(ctx, stepEv, &seq)
		ctx, stepSpan := StartAgentSpan(ctx, "decide", SpanKindStep, out.Steps, "", "")
		decideCtx := ctx
		if !req.DisableThinking {
			decideCtx = WithReasoningSink(ctx, func(delta string) error {
				if delta == "" {
					return nil
				}
				return r.emit(ctx, Event{Type: "llm.reasoning.delta", State: StateDeciding, Message: delta, StepID: stepID}, &seq)
			})
		}
		decision, err := r.planner.Decide(decideCtx, PlannerInput{
			Prompt: req.Prompt, Step: out.Steps, Observations: out.Observations, AuthorizedKBs: req.AuthorizedKBs,
		})
		if err != nil && hasUsefulEvidence(out.Observations) && (errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled)) {
			decision = Decision{Decision: DecisionFinal, ThoughtNote: "已有证据，整理回答"}
			err = nil
		}
		stepSpan.ApplyEvent(&stepEv, err)
		if err != nil {
			out.State = StateFailed
			_ = r.emit(ctx, Event{Type: "run.failed", State: out.State, Message: err.Error()}, &seq)
			return out, err
		}
		if !req.DisableThinking {
			if note := SanitizeUserThought(decision.ThoughtNote); note != "" {
				_ = r.emit(ctx, Event{Type: "planner.thought", State: StateDeciding, Message: note, StepID: stepID}, &seq)
			}
		}
		decision = resolveClarify(decision, out.Observations)
		switch decision.Decision {
		case DecisionFinal, DecisionClarify:
			out.State = StateFinalizing
			out.Answer = decision.Answer
			failFinal := func(err error) (Result, error) {
				out.Answer = ""
				out.State = StateFailed
				event := "run.failed"
				if errors.Is(err, context.DeadlineExceeded) {
					out.State, event = StateExpired, "run.expired"
				} else if errors.Is(err, context.Canceled) {
					out.State, event = StateCancelled, "run.cancelled"
				}
				_ = r.emit(ctx, Event{Type: event, State: out.State, Message: err.Error()}, &seq)
				return out, err
			}
			isRefuseTemplate := strings.TrimSpace(out.Answer) == refuseNoEvidenceTemplate
			catalogOnly := false
			if decision.Decision == DecisionFinal && !isRefuseTemplate {
				if isCatalogListQuestion(CurrentUserQuery(string(req.Prompt))) && !hasChunkEvidence(out.Observations) {
					if overview := catalogOverviewAnswer(out.Observations); overview != "" {
						out.Answer = overview
						catalogOnly = true
					}
				}
				// The model produces a candidate only. No answer.delta may be
				// emitted until transport completion and the answer gate pass.
				if !catalogOnly && r.streamer != nil {
					if !req.DisableThinking {
						if err := r.emit(ctx, Event{Type: "answer.generating", State: StateFinalizing, Message: "正在生成回答…"}, &seq); err != nil {
							return failFinal(err)
						}
					}
					streamCtx := ctx
					if !req.DisableThinking {
						streamCtx = WithReasoningSink(ctx, func(delta string) error {
							if delta == "" {
								return nil
							}
							return r.emit(ctx, Event{Type: "llm.reasoning.delta", State: StateFinalizing, Message: delta, StepID: stepID}, &seq)
						})
					}
					answerCtx, answerCancel := context.WithTimeout(streamCtx, answerStreamTimeout)
					answer, streamErr := StreamFinalAnswer(answerCtx, r.streamer, req.Prompt, out.Observations, out.Answer, nil)
					answerCancel()
					if streamErr != nil {
						return failFinal(&AnswerGenerationError{Cause: streamErr})
					}
					out.Answer = answer
				}
				if r.validator != nil && !catalogOnly {
					vr, err := r.validator.Validate(ctx, out.Answer, out.Observations)
					if err != nil {
						return failFinal(err)
					}
					if vr.Rejected {
						if err := r.emit(ctx, Event{Type: "answer.rejected", State: StateFinalizing, Message: string(GateDetailJSON(vr.Gate)), StepID: stepID}, &seq); err != nil {
							return failFinal(err)
						}
						evidenceGap := vr.Gate.RefuseReason == "no_evidence" || vr.Gate.RefuseReason == "insufficient_citations"
						if evidenceGap && !hasToolObservation(out.Observations, "knowledge.read") && out.Steps < limits.MaxSteps {
							gap, _ := json.Marshal(map[string]any{
								"gap":                vr.Gate.RefuseReason,
								"hint":               "证据不足。请改用 knowledge.lexical、knowledge.semantic、knowledge.read 或 knowledge.expand。不要用相同参数重复调用。",
								"required_citations": vr.Gate.RequiredCitations, "citation_count": vr.Gate.CitationCount,
							})
							out.Observations = append(out.Observations, Observation{CallID: "answer_gate", Name: "answer.gap", Output: gap})
							continue
						}
						out.Answer = strings.TrimSpace(vr.Answer)
						if out.Answer == "" {
							out.Answer = refuseNoEvidenceTemplate
						}
						// The validator's safe replacement is final. Regenerating
						// it would introduce a second, unvalidated answer.
					} else {
						if strings.TrimSpace(vr.Answer) != "" {
							out.Answer = vr.Answer
						}
						if err := r.emit(ctx, Event{Type: "answer.validated", State: StateFinalizing, Message: string(GateDetailJSON(vr.Gate)), StepID: stepID}, &seq); err != nil {
							return failFinal(err)
						}
					}
				}
			}
			if decision.Decision == DecisionClarify && strings.TrimSpace(out.Answer) == "" {
				out.Answer = clarifyAskTemplate
			}
			if strings.TrimSpace(out.Answer) == "" {
				if evidenceIsEmpty(out.Observations) {
					out.Answer = refuseNoEvidenceTemplate
				} else {
					return failFinal(errors.New("agent: final answer is empty"))
				}
			}
			if err := ctx.Err(); err != nil {
				return failFinal(err)
			}
			if err := r.emit(ctx, Event{Type: "answer.delta", State: StateFinalizing, Message: out.Answer}, &seq); err != nil {
				return failFinal(err)
			}
			out.State = StateCompleted
			if err := r.emit(ctx, Event{Type: "run.completed", State: out.State, Message: out.Answer}, &seq); err != nil {
				return failFinal(err)
			}
			return out, nil

		case DecisionTools:
			if len(decision.ToolCalls) == 0 {
				out.State = StateFailed
				return out, errors.New("agent: tool_calls decision has no calls")
			}
			if out.ToolCalls+len(decision.ToolCalls) > limits.MaxToolCalls {
				out.State = StateExpired
				_ = r.emit(ctx, Event{Type: "run.tool_budget_exceeded", State: out.State}, &seq)
				return out, fmt.Errorf("agent: max tool calls %d exceeded", limits.MaxToolCalls)
			}
			progress := false
			for _, call := range decision.ToolCalls {
				fp := Fingerprint(call)
				if count(fingerprints, fp) >= limits.MaxSameActionRepeat {
					out.State = StateFailed
					_ = r.emit(ctx, Event{Type: "run.no_progress", State: out.State, CallID: call.CallID, ToolName: call.Name}, &seq)
					return out, fmt.Errorf("agent: repeated tool action %s", call.Name)
				}
				fingerprints = append(fingerprints, fp)
				toolStart := Event{Type: "tool.started", State: StateExecuting, CallID: call.CallID, ToolName: call.Name, StepID: out.Steps}
				toolCtx, toolSpan := StartAgentSpan(ctx, call.Name, SpanKindTool, out.Steps, call.CallID, call.Name)
				toolSpan.StampEvent(&toolStart)
				_ = r.emit(toolCtx, toolStart, &seq)
				if r.tools == nil {
					return out, errors.New("agent: tool executor is required")
				}
				obs, execErr := r.tools.Execute(toolCtx, call)
				out.ToolCalls++
				if execErr != nil {
					obs.Err = execErr
				}
				metricStatus := "success"
				if obs.Err != nil {
					metricStatus = "error"
				}
				RecordToolMetric(call.Name, metricStatus)
				out.Observations = append(out.Observations, obs)
				if obs.Err == nil && len(obs.Output) > 0 {
					progress = true
				}
				toolDone := Event{Type: "tool.completed", State: StateObserving, CallID: call.CallID, ToolName: call.Name, Message: errorText(obs.Err), StepID: out.Steps}
				if obs.Err != nil && errors.Is(obs.Err, ErrApprovalRequired) {
					_ = r.emit(toolCtx, Event{
						Type: "tool.approval_required", State: StateValidating, CallID: call.CallID, ToolName: call.Name,
						Message: obs.Err.Error(), StepID: out.Steps,
					}, &seq)
				}
				toolSpan.FinishEvent(&toolDone, obs.Err)
				_ = r.emit(toolCtx, toolDone, &seq)
			}
			if progress {
				noProgress = 0
			} else {
				noProgress++
			}
			if noProgress >= limits.MaxNoProgressSteps {
				out.State = StateFailed
				_ = r.emit(ctx, Event{Type: "run.no_progress", State: out.State}, &seq)
				return out, errors.New("agent: no progress")
			}
			// The application planner receives observations through its own context adapter.
		default:
			out.State = StateFailed
			_ = r.emit(ctx, Event{Type: "run.invalid_decision", State: out.State}, &seq)
			return out, fmt.Errorf("agent: invalid decision %q", decision.Decision)
		}
	}
}

func Fingerprint(c ToolCall) string {
	h := sha256.Sum256(append([]byte(c.Name+":"), c.Arguments...))
	return hex.EncodeToString(h[:])
}
func count(xs []string, needle string) int {
	n := 0
	for _, x := range xs {
		if x == needle {
			n++
		}
	}
	return n
}
func errorText(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

// StableArgumentKeys is useful to canonicalize argument maps before hashing.
func StableArgumentKeys(m map[string]any) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
