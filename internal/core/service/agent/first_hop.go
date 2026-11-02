package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
)

var (
	reSnakeIdent  = regexp.MustCompile(`\b[A-Za-z][A-Za-z0-9]*_[A-Za-z0-9_]+\b`)
	reHeaderIdent = regexp.MustCompile(`\b[A-Z][A-Z0-9]+(?:-[A-Z0-9]+){1,}\b`)
	reCamelIdent  = regexp.MustCompile(`\b[A-Z][a-z0-9]+(?:[A-Z][A-Za-z0-9]*)+\b`)
	reCapsIdent   = regexp.MustCompile(`\b[A-Z]{3,}\b`)
	reTickIdent   = regexp.MustCompile("`[^`\\n]{2,}`")
	reIDWord      = regexp.MustCompile(`(?i)\b(?:id|taxonomy)\b`)
	reLatinWord   = regexp.MustCompile(`\b[A-Za-z][A-Za-z0-9]{3,}\b`)
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
		if PrefersLexical(query) {
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
	if hasChunkEvidence(in.Observations) && !hasToolObservation(in.Observations, "knowledge.read") {
		id := topChunkID(in.Observations)
		if id != "" {
			raw, _ := json.Marshal(map[string]any{
				"kb_type": kb, "chunk_id": id, "span": "group", "query": q,
			})
			return Decision{
				Decision:    DecisionTools,
				ThoughtNote: "读同一篇材料的相邻段",
				ToolCalls: []ToolCall{{
					CallID: "group_read_1", Name: "knowledge.read", Arguments: raw,
				}},
			}, true
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
func CurrentUserQuery(prompt string) string {
	s := strings.TrimSpace(prompt)
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
	if reSnakeIdent.MatchString(q) || reHeaderIdent.MatchString(q) || reCamelIdent.MatchString(q) || reCapsIdent.MatchString(q) || reTickIdent.MatchString(q) {
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
		// Multi-doc / theme questions start from the workspace catalog (P2-4).
		return "knowledge.catalog"
	case "concept", "unknown", "":
		return "knowledge.semantic"
	default:
		return "knowledge.semantic"
	}
}
