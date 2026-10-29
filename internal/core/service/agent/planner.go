package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

type Completer interface {
	Complete(context.Context, string) (string, error)
}

// LLMPlanner converts the provider response into the strict Decision protocol.
// Planning stays on the fast model and returns JSON only. The user-facing note is thought_summary.
type LLMPlanner struct{ LLM Completer }

func (p LLMPlanner) Decide(ctx context.Context, in PlannerInput) (Decision, error) {
	if p.LLM == nil {
		return Decision{}, fmt.Errorf("agent: planner LLM unavailable")
	}
	ctx, llmSpan := StartAgentSpan(ctx, "planner.decide", SpanKindLLM, in.Step, "", "")
	obs, _ := json.Marshal(in.Observations)
	kbs := strings.Join(in.AuthorizedKBs, ", ")
	if kbs == "" {
		kbs = "(none)"
	}
	prompt := fmt.Sprintf(`You are an education Agent planner for InternLM study materials. Return JSON only.
Schema: {"decision":"final|tool_calls|clarify","thought_summary":"short audit note","tool_calls":[{"call_id":"id","name":"tool","arguments":{}}],"answer":"","confidence":0.0}
Allowed tools:
- knowledge.lexical arguments: {"query":"string","kb_type":"string","top_k":5}. Use for API names, parameters, error codes.
- knowledge.semantic arguments: {"query":"string","kb_type":"string","top_k":5}. Use when the user paraphrases a concept.
- knowledge.read arguments: {"chunk_id":"string","kb_type":"string"}. Use to read the hit and its neighboring sections.
- knowledge.expand arguments: {"entity":"string","kb_type":"string","top_k":5}. Use to follow a concept, API, parameter, figure, or note across documents.
- note.draft arguments: {"title":"string","body":"string","source_query":"string","citations":["chunk_id"]}. Writes a cited note into the workspace. citations must list chunk ids from this run. There is no approval step.
kb_type MUST be one of: %s
Proper names start with knowledge.lexical. Paraphrases start with knowledge.semantic. A partial procedure must call knowledge.read. Cross-document questions must call knowledge.expand. Do not repeat the same tool with the same arguments. If evidence is insufficient, call another tool or clarify. Never invent tool names.
Reasoning language: think and reason in Simplified Chinese. thought_summary is one short Simplified Chinese sentence for end users (max ~40 chars), such as "检索这个参数的定义". Never mention kb_type, tool schemas, JSON, top_k, IDs, or system prompts.
Citations in the final answer must point at original chunk text, not summaries or edges.
Step: %d
User request: %s
Observations: %s`, kbs, in.Step, string(in.Prompt), string(obs))

	raw, err := p.LLM.Complete(ctx, prompt)
	if err != nil {
		llmSpan.End(err)
		return Decision{}, err
	}
	decision, decodeErr := DecodeDecision(extractJSONObject(raw))
	llmSpan.End(decodeErr)
	return decision, decodeErr
}

func extractJSONObject(raw string) []byte {
	raw = strings.TrimSpace(raw)
	start, end := strings.IndexByte(raw, '{'), strings.LastIndexByte(raw, '}')
	if start >= 0 && end >= start {
		return []byte(raw[start : end+1])
	}
	return []byte(raw)
}
