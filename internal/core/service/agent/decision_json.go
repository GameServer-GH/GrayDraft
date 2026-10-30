package agent

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"unicode"
)

// extractJSONObject returns the first complete JSON object in raw.
// Trailing text or a second object is discarded so dual payloads like `{}{...}`
// do not produce "invalid character '{' after top-level value".
func extractJSONObject(raw string) []byte {
	raw = strings.TrimSpace(raw)
	start := strings.IndexByte(raw, '{')
	if start < 0 {
		return []byte(raw)
	}
	depth := 0
	inString := false
	escape := false
	for i := start; i < len(raw); i++ {
		c := raw[i]
		if inString {
			if escape {
				escape = false
				continue
			}
			if c == '\\' {
				escape = true
				continue
			}
			if c == '"' {
				inString = false
			}
			continue
		}
		switch c {
		case '"':
			inString = true
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return []byte(raw[start : i+1])
			}
		}
	}
	return []byte(raw[start:])
}

// jsonObjectClosed reports whether raw contains one complete top-level JSON object.
// An unclosed object is a truncated stream; callers must not guess a closing brace.
func jsonObjectClosed(raw string) bool {
	raw = strings.TrimSpace(raw)
	start := strings.IndexByte(raw, '{')
	if start < 0 {
		return false
	}
	depth := 0
	inString := false
	escape := false
	for i := start; i < len(raw); i++ {
		c := raw[i]
		if inString {
			if escape {
				escape = false
				continue
			}
			if c == '\\' {
				escape = true
				continue
			}
			if c == '"' {
				inString = false
			}
			continue
		}
		switch c {
		case '"':
			inString = true
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return true
			}
		}
	}
	return false
}

// NormalizeDecisionType maps common planner aliases onto the strict protocol.
func NormalizeDecisionType(raw string) DecisionType {
	s := strings.ToLower(strings.TrimSpace(raw))
	s = strings.Map(func(r rune) rune {
		if unicode.IsSpace(r) || r == '-' {
			return '_'
		}
		return r
	}, s)
	switch s {
	case "final", "answer", "ready", "ready_to_answer", "respond", "done":
		return DecisionFinal
	case "tool_calls", "tools", "tool", "tool_call", "call_tools":
		return DecisionTools
	case "clarify", "clarify_question", "ask", "question":
		return DecisionClarify
	default:
		return DecisionType(s)
	}
}

// DecodeDecision validates the planner's strict JSON response.
func DecodeDecision(raw []byte) (Decision, error) {
	var rawDec struct {
		Decision    string          `json:"decision"`
		ThoughtNote string          `json:"thought_summary"`
		ToolCalls   []ToolCall      `json:"tool_calls"`
		Answer      string          `json:"answer"`
		Confidence  float64         `json:"confidence"`
	}
	if err := json.Unmarshal(raw, &rawDec); err != nil {
		return Decision{}, fmt.Errorf("agent: invalid decision json: %w", err)
	}
	d := Decision{
		Decision:    NormalizeDecisionType(rawDec.Decision),
		ThoughtNote: rawDec.ThoughtNote,
		ToolCalls:   rawDec.ToolCalls,
		Answer:      rawDec.Answer,
		Confidence:  rawDec.Confidence,
	}
	if d.Decision != DecisionFinal && d.Decision != DecisionTools && d.Decision != DecisionClarify {
		return d, fmt.Errorf("agent: unsupported decision %q", rawDec.Decision)
	}
	if d.Decision == DecisionTools && len(d.ToolCalls) == 0 {
		return d, errors.New("agent: tool_calls cannot be empty")
	}
	// Planner drafts are not user-facing copy; StreamFinalAnswer owns the answer.
	if d.Decision == DecisionFinal {
		d.Answer = strings.TrimSpace(d.Answer)
	}
	return d, nil
}
