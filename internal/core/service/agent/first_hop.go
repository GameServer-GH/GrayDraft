package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
)

var (
	reScientificIdent = regexp.MustCompile(`\b[A-Za-z]{1,}[0-9]{2,}[A-Za-z0-9]*\b`)
	reSnakeIdent      = regexp.MustCompile(`\b[A-Za-z][A-Za-z0-9]*_[A-Za-z0-9_]+\b`)
	reHeaderIdent     = regexp.MustCompile(`\b[A-Z][A-Z0-9]+(?:-[A-Z0-9]+){1,}\b`)
	reCamelIdent      = regexp.MustCompile(`\b[A-Z][a-z0-9]+(?:[A-Z][A-Za-z0-9]*)+\b`)
	reCapsIdent       = regexp.MustCompile(`\b[A-Z]{3,}\b`)
	reTickIdent       = regexp.MustCompile("`[^`\\n]{2,}`")
	reIDWord          = regexp.MustCompile(`(?i)\b(?:id|taxonomy)\b`)
	reLatinWord       = regexp.MustCompile(`\b[A-Za-z][A-Za-z0-9]{3,}\b`)
)

// FirstHopSelector picks the first retrieval tool from the user query without an LLM call.
// Intent routing stays domain-agnostic (proper name → lexical, paraphrase → semantic).
type FirstHopSelector func(query string, authorizedKBs []string) (toolName string, ok bool)

// FirstHopPlanner runs a deterministic first retrieval hop, then delegates to Inner.
type FirstHopPlanner struct {
	Inner  Planner
	Select FirstHopSelector
}

func (p FirstHopPlanner) Decide(ctx context.Context, in PlannerInput) (Decision, error) {
	// After the first retrieval, allow one planner step so a missing number or a
	// second document can still be read. Further steps write up what was found.
	// A hung planner is bounded elsewhere and falls back to the evidence in hand.
	if in.Step > 2 && hasUsefulEvidence(in.Observations) {
		return Decision{Decision: DecisionFinal, ThoughtNote: "已有证据，整理回答"}, nil
	}
	if d, ok := followUpDecision(in); ok {
		return d, nil
	}
	if p.Select != nil && in.Step == 1 && !hasKnowledgeObservation(in.Observations) {
		query := CurrentUserQuery(string(in.Prompt))
		tool, ok := "", false
		// Identifiers, headers, and function names stay on lexical even when the
		// classifier labels the sentence as a concept. Concept boost must not replace this hop.
		if isContentOverview(query) || isCatalogListQuestion(query) {
			tool, ok = "knowledge.catalog", true
		} else if PrefersLexical(query) {
			tool, ok = "knowledge.lexical", true
		} else {
			tool, ok = p.Select(query, in.AuthorizedKBs)
		}
		if ok && tool != "" {
			kb := ""
			if len(in.AuthorizedKBs) > 0 {
				kb = in.AuthorizedKBs[0]
			}
			args := map[string]any{"kb_type": kb}
			switch tool {
			case "knowledge.catalog":
				args["query"] = query
				args["limit"] = 20
			default:
				args["query"] = query
				args["top_k"] = 5
				if in.TopK > 0 && in.TopK <= 20 {
					args["top_k"] = in.TopK
				}
			}
			raw, _ := json.Marshal(args)
			return Decision{
				Decision:    DecisionTools,
				ThoughtNote: firstHopThought(tool),
				ToolCalls: []ToolCall{{
					CallID:    "first_hop_1",
					Name:      tool,
					Arguments: raw,
				}},
			}, nil
		}
	}
	if p.Inner == nil {
		return Decision{}, fmt.Errorf("agent: first-hop inner planner unavailable")
	}
	return p.Inner.Decide(ctx, in)
}

func followUpDecision(in PlannerInput) (Decision, bool) {
	if in.Step != 2 {
		return Decision{}, false
	}
	q := CurrentUserQuery(string(in.Prompt))
	kb := ""
	if len(in.AuthorizedKBs) > 0 {
		kb = in.AuthorizedKBs[0]
	}
	if isCatalogListQuestion(q) && catalogHasDocs(in.Observations) {
		return Decision{Decision: DecisionFinal, ThoughtNote: "已有证据，整理回答"}, true
	}
	if isContentOverview(q) && !hasChunkEvidence(in.Observations) {
		groups := openingGroups(in.Observations, 3)
		if len(groups) > 0 {
			raw, _ := json.Marshal(map[string]any{"kb_type": kb, "chunk_id": "openings", "groups": groups})
			return Decision{
				Decision:    DecisionTools,
				ThoughtNote: "读每篇材料的开头",
				ToolCalls: []ToolCall{{
					CallID: "group_read_1", Name: "knowledge.read", Arguments: raw,
				}},
			}, true
		}
	}
	// Multi-source questions need evidence for each explicitly named identifier.
	// This is a bounded retrieval subquery, never a rewrite of the user question.
	ids := reScientificIdent.FindAllString(q, -1)
	if len(ids) > 1 && hasChunkEvidence(in.Observations) {
		hits := CollectEvidence(in.Observations).Hits
		seen := map[string]bool{}
		calls := []ToolCall{}
		for _, id := range ids {
			if seen[id] {
				continue
			}
			seen[id] = true
			covered := false
			for _, hit := range hits {
				if strings.Contains(strings.ToLower(hit.Title), strings.ToLower(id)) {
					covered = true
					break
				}
			}
			if !covered && len(calls) < 3 {
				raw, _ := json.Marshal(map[string]any{"kb_type": kb, "query": id, "top_k": 3})
				calls = append(calls, ToolCall{CallID: fmt.Sprintf("source_cover_%d", len(calls)), Name: "knowledge.lexical", Arguments: raw})
			}
		}
		if len(calls) > 0 {
			return Decision{Decision: DecisionTools, ThoughtNote: "补查问题中尚未覆盖的来源", ToolCalls: calls}, true
		}
	}
	if hasChunkEvidence(in.Observations) && !hasToolObservation(in.Observations, "knowledge.read") {
		id := topChunkID(in.Observations)
		if id != "" {
			raw, _ := json.Marshal(map[string]any{
				"kb_type": kb, "chunk_id": id, "span": "group", "query": q,
			})
			calls := []ToolCall{{CallID: "group_read_1", Name: "knowledge.read", Arguments: raw}}
			if reScientificIdent.MatchString(q) && hasToolObservation(in.Observations, "knowledge.lexical") {
				semanticArgs, _ := json.Marshal(map[string]any{"kb_type": kb, "query": q, "top_k": 5})
				calls = append(calls, ToolCall{CallID: "identifier_semantic_1", Name: "knowledge.semantic", Arguments: semanticArgs})
			}
			return Decision{Decision: DecisionTools, ThoughtNote: "补读来源并核对问题相关正文", ToolCalls: calls}, true

		}
	}
	return Decision{}, false
}

func hasToolObservation(obs []Observation, name string) bool {
	for _, o := range obs {
		if o.Name == name {
			return true
		}
	}
	return false
}

func topChunkID(obs []Observation) string {
	for _, h := range CollectEvidence(obs).Hits {
		if strings.TrimSpace(h.ChunkID) != "" {
			return h.ChunkID
		}
	}
	return ""
}

func hasKnowledgeObservation(obs []Observation) bool {
	for _, o := range obs {
		if strings.HasPrefix(o.Name, "knowledge.") {
			return true
		}
	}
	return false
}

func firstHopThought(tool string) string {
	switch tool {
	case "knowledge.lexical":
		return "按专名检索资料"
	case "knowledge.semantic":
		return "按语义检索资料"
	case "knowledge.catalog":
		return "查看工作区文档目录"
	default:
		return "检索工作区资料"
	}
}

// CurrentUserQuery keeps the latest user utterance. Conversation history and
// transport lines (top_k, chat_mode, workspace_context) must not pick the first hop.
const literalQuestionPrefix = "\x1egraydraft-question-v1\n"

func CurrentUserQuery(prompt string) string {
	if strings.HasPrefix(prompt, literalQuestionPrefix) {
		return strings.TrimPrefix(prompt, literalQuestionPrefix)
	}
	s := strings.TrimSpace(prompt)
	legacy := strings.HasPrefix(s, "top_k=") || strings.HasPrefix(s, "chat_mode=") || strings.HasPrefix(s, "workspace_context:") || strings.Contains(s, "\nuser: ")
	if !legacy {
		return s
	}
	if idx := strings.LastIndex(s, "\nuser: "); idx >= 0 {
		s = strings.TrimSpace(s[idx+len("\nuser: "):])
	}
	lines := strings.Split(s, "\n")
	for i := len(lines) - 1; i >= 0; i-- {
		t := strings.TrimSpace(lines[i])
		if t == "" || strings.HasPrefix(t, "top_k=") || strings.HasPrefix(t, "chat_mode=") || t == "workspace_context:" {
			continue
		}
		return t
	}
	return s
}

// PrefersLexical reports whether the question names an identifier, header, or function
// that fulltext should retrieve before any concept ranking.
func PrefersLexical(q string) bool {
	if reScientificIdent.MatchString(q) || reSnakeIdent.MatchString(q) || reHeaderIdent.MatchString(q) || reCamelIdent.MatchString(q) || reCapsIdent.MatchString(q) || reTickIdent.MatchString(q) {
		return true
	}
	return reIDWord.MatchString(q) && reLatinWord.MatchString(q)
}

// FirstHopFromIntent maps classifier-style intent labels onto education tools.
func FirstHopFromIntent(intent string) string {
	switch strings.ToLower(strings.TrimSpace(intent)) {
	case "proper_noun", "fact":
		return "knowledge.lexical"
	case "hybrid":
		// Hybrid facts require content retrieval; explicit overview/list requests
		// are selected by FirstHopPlanner before this intent mapping.
		return "knowledge.semantic"
	case "concept", "unknown", "":
		return "knowledge.semantic"
	default:
		return "knowledge.semantic"
	}
}
