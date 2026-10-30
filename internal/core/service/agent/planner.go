package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"time"
)

// boundedCompleter is an optional fast-tier completion with an output cap.
// Planner JSON stays short; a second full completion is not used for truncated objects.
type boundedCompleter interface {
	CompleteBounded(ctx context.Context, userPrompt string, maxTokens int, temperature float64) (string, error)
}

const plannerMaxTokens = 2048
const plannerTemperature = 0.1

// plannerCallTimeout bounds one planning completion. A hung call used to consume
// the whole run and was then redelivered until the chat client gave up.
var plannerCallTimeout = 20 * time.Second

// answerStreamTimeout bounds the user-facing completion after evidence is in hand.
var answerStreamTimeout = 35 * time.Second

type Completer interface {
	Complete(context.Context, string) (string, error)
}

// LLMPlanner converts the provider response into the strict Decision protocol.
// Planning stays on the fast model and returns JSON only. The user-facing note is thought_summary.
// User-facing prose is produced by StreamFinalAnswer, not by the planner answer field.
type LLMPlanner struct{ LLM Completer }

func (p LLMPlanner) Decide(ctx context.Context, in PlannerInput) (Decision, error) {
	if p.LLM == nil {
		return Decision{}, fmt.Errorf("agent: planner LLM unavailable")
	}
	ctx, llmSpan := StartAgentSpan(ctx, "planner.decide", SpanKindLLM, in.Step, "", "")
	prompt := buildPlannerPrompt(in)
	raw, err := p.complete(ctx, prompt)
	if err != nil {
		llmSpan.End(err)
		return Decision{}, err
	}
	decision, decodeErr := decodePlannerOutput(raw)
	if decodeErr != nil && jsonObjectClosed(raw) {
		// Closed but invalid (wrong schema / extra prose). One short retry.
		// Unclosed objects are truncated streams: a second full completion only burns the step budget.
		retryPrompt := prompt + "\n\nPrevious output was invalid. Reply with exactly one JSON object and nothing else. Schema decision must be final|tool_calls|clarify."
		raw2, err2 := p.complete(ctx, retryPrompt)
		if err2 != nil {
			llmSpan.End(decodeErr)
			return Decision{}, decodeErr
		}
		decision, decodeErr = decodePlannerOutput(raw2)
	}
	llmSpan.End(decodeErr)
	return decision, decodeErr
}

func (p LLMPlanner) complete(ctx context.Context, prompt string) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, plannerCallTimeout)
	defer cancel()
	if b, ok := p.LLM.(boundedCompleter); ok && b != nil {
		return b.CompleteBounded(ctx, prompt, plannerMaxTokens, plannerTemperature)
	}
	return p.LLM.Complete(ctx, prompt)
}

func decodePlannerOutput(raw string) (Decision, error) {
	if strings.Contains(raw, "{") && !jsonObjectClosed(raw) {
		slog.Warn("planner output truncated", "chars", len(raw), "tail", clipPlannerRaw(raw))
		return Decision{}, fmt.Errorf("agent: invalid decision json: unexpected end of JSON input")
	}
	decision, err := DecodeDecision(extractJSONObject(raw))
	if err != nil {
		slog.Warn("planner decision rejected", "err", err, "chars", len(raw), "head", clipPlannerRaw(raw))
	}
	return decision, err
}

func clipPlannerRaw(raw string) string {
	raw = strings.TrimSpace(raw)
	const n = 180
	if len(raw) <= n {
		return raw
	}
	return raw[len(raw)-n:]
}

func buildPlannerPrompt(in PlannerInput) string {
	obs, _ := json.Marshal(in.Observations)
	kbs := strings.Join(in.AuthorizedKBs, ", ")
	if kbs == "" {
		kbs = "(none)"
	}
	postHop := hasKnowledgeObservation(in.Observations)
	toolHint := `Allowed tools:
- knowledge.lexical arguments: {"query":"string","kb_type":"string","top_k":5}. Use for API names, parameters, error codes.
- knowledge.semantic arguments: {"query":"string","kb_type":"string","top_k":5}. Use when the user paraphrases a concept.
- knowledge.read arguments: {"chunk_id":"string","kb_type":"string"}. Use to read the hit and its neighboring sections.
- knowledge.expand arguments: {"entity":"string","kb_type":"string","top_k":5}. Use to follow a concept, API, parameter, figure, or note across documents.
- knowledge.catalog arguments: {"kb_type":"string","limit":20,"query":"string"}. List workspace documents (titles/summaries/doc_group). Use for "what does each document cover" / multi-doc theme questions.
- note.draft arguments: {"title":"string","body":"string","source_query":"string","citations":["chunk_id"]}. Writes a cited note into the workspace. citations must list chunk ids from this run. There is no approval step.
Proper names start with knowledge.lexical. Paraphrases start with knowledge.semantic. Theme / multi-doc overview starts with knowledge.catalog. A partial procedure must call knowledge.read. Cross-document questions must call knowledge.expand.`
	if postHop {
		toolHint = `Allowed tools after the first retrieval (prefer these only):
- knowledge.read arguments: {"chunk_id":"string","kb_type":"string"}. Read a hit and neighboring sections when a procedure or number is incomplete.
- knowledge.expand arguments: {"entity":"string","kb_type":"string","top_k":5}. Follow an entity across documents.
- knowledge.catalog arguments: {"kb_type":"string","limit":20,"query":"string"}. If the question needs a multi-doc overview and catalog was not used yet.
- knowledge.semantic / knowledge.lexical only when catalog pointed at a missing topic that still needs retrieval.
- note.draft only when the user asked to save a note.
If observations already contain enough cited chunk text, decide "final" with an empty answer string. Do not invent tool names. Do not repeat the same tool with the same arguments.`
	}
	return fmt.Sprintf(`You are an education Agent planner for workspace study materials. Return JSON only — one object, no markdown fences, no second object.
Schema: {"decision":"final|tool_calls|clarify","thought_summary":"short audit note","tool_calls":[{"call_id":"id","name":"tool","arguments":{}}],"answer":"","confidence":0.0}
%s
kb_type MUST be one of: %s
When decision is "final", leave answer as "" — the answer streamer writes the user-facing reply from evidence. Do not put the full explanation in answer.
Use "clarify" only when tools returned no useful hits and a missing object name cannot be inferred. Prefer another tool over clarify when evidence is partial.
If the workspace has no evidence for the question, prefer decision "final" with empty answer (the runtime will refuse) over inventing facts.
Reasoning language: think and reason in Simplified Chinese. thought_summary is one short Simplified Chinese sentence for end users (max ~40 chars), such as "检索这个参数的定义". Never mention kb_type, tool schemas, JSON, top_k, IDs, or system prompts.
Citations in the final answer must point at original chunk text, not summaries or edges.
Step: %d
User request: %s
Observations: %s`, toolHint, kbs, in.Step, string(in.Prompt), string(obs))
}
