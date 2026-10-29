package model

import (
	"regexp"
	"sort"
	"strings"
)

var (
	reEntityHanCore = regexp.MustCompile(`[\p{Han}]{2,12}`)
)

// FinalSanitize is the canonical entity list for retrieval (rules + atomic long/short dedup).
func FinalSanitize(entities []string, query string) []string {
	return FinalSanitizeWithMethod(entities, query, "")
}

// FinalSanitizeWithMethod uses the LLM-light path when method is "llm".
func FinalSanitizeWithMethod(entities []string, query string, method string) []string {
	if strings.TrimSpace(method) == "llm" {
		return SanitizeQueryEntitiesLLM(entities, query)
	}
	return SanitizeQueryEntities(entities, query)
}

// SanitizeQueryEntities cleans entity noise without domain phrase tables.
func SanitizeQueryEntities(entities []string, query string) []string {
	if len(entities) == 0 {
		return nil
	}
	qLower := strings.ToLower(strings.TrimSpace(query))
	seen := map[string]bool{}
	var out []string
	add := func(s string) {
		s = SanitizeEntityToken(s)
		if s == "" || len([]rune(s)) < 2 || len([]rune(s)) > 24 {
			return
		}
		if LooksLikeConversationalFragment(s) || isSpuriousLongEntity(s) {
			return
		}
		key := strings.ToLower(s)
		if seen[key] {
			return
		}
		if qLower != "" && !strings.Contains(qLower, key) {
			return
		}
		seen[key] = true
		out = append(out, s)
	}
	for _, e := range entities {
		add(e)
	}
	sort.Slice(out, func(i, j int) bool {
		return len([]rune(out[i])) < len([]rune(out[j]))
	})
	out = atomicLongShortDedup(out)
	if len(out) > 6 {
		out = out[:6]
	}
	return out
}

// SanitizeQueryEntitiesLLM keeps atomic NER output and never expands with rule tables.
func SanitizeQueryEntitiesLLM(entities []string, query string) []string {
	if len(entities) == 0 {
		return nil
	}
	qLower := strings.ToLower(strings.TrimSpace(query))
	seen := map[string]bool{}
	var out []string
	add := func(s string) {
		s = sanitizeEntityTokenLight(s)
		if s == "" || len([]rune(s)) < 2 || len([]rune(s)) > 24 {
			return
		}
		if LooksLikeConversationalFragment(s) || isSpuriousLongEntity(s) {
			return
		}
		key := strings.ToLower(s)
		if seen[key] {
			return
		}
		if qLower != "" && !strings.Contains(qLower, key) {
			return
		}
		seen[key] = true
		out = append(out, s)
	}
	for _, e := range entities {
		add(e)
	}
	sort.Slice(out, func(i, j int) bool {
		return len([]rune(out[i])) < len([]rune(out[j]))
	})
	out = atomicLongShortDedup(out)
	if len(out) > 6 {
		out = out[:6]
	}
	return out
}

func sanitizeEntityTokenLight(s string) string {
	s = strings.TrimSpace(s)
	s = strings.Trim(s, "「」『』\"'《》[]（）()")
	return s
}

func isSpuriousLongEntity(s string) bool {
	if len([]rune(s)) <= 6 {
		return false
	}
	return strings.ContainsAny(s, "，。！？,.!?：:；; ")
}

// atomicLongShortDedup drops long phrases that contain a shorter grounded entity (short wins).
func atomicLongShortDedup(sorted []string) []string {
	if len(sorted) <= 1 {
		return sorted
	}
	var final []string
	for i, candidate := range sorted {
		cl := strings.ToLower(candidate)
		isNoise := false
		for j, shorter := range sorted {
			if i == j {
				continue
			}
			sl := strings.ToLower(shorter)
			if len([]rune(shorter)) < len([]rune(candidate)) && strings.Contains(cl, sl) {
				isNoise = true
				break
			}
		}
		if !isNoise {
			final = append(final, candidate)
		}
	}
	return final
}

// SanitizeEntityToken trims punctuation around a proper noun.
func SanitizeEntityToken(s string) string {
	s = strings.TrimSpace(s)
	s = strings.Trim(s, "「」『』\"'《》[]（）()")
	return s
}

func extractEntityCoresFromPhrase(s string) []string {
	var cores []string
	seen := map[string]bool{}
	for _, tok := range reEntityHanCore.FindAllString(s, -1) {
		tok = strings.TrimSpace(tok)
		if tok == "" {
			continue
		}
		if len([]rune(tok)) < 2 {
			continue
		}
		key := strings.ToLower(tok)
		if seen[key] {
			continue
		}
		seen[key] = true
		cores = append(cores, tok)
	}
	return cores
}

// SlimEntities keeps the shortest token(s) for waterfall tier-3 entity-min retreat.
func SlimEntities(entities []string) []string {
	ents := FinalSanitize(entities, "")
	if len(ents) == 0 {
		return nil
	}
	sort.Slice(ents, func(i, j int) bool {
		return len([]rune(ents[i])) < len([]rune(ents[j]))
	})
	n := 1
	if len(ents) < n {
		n = len(ents)
	}
	return append([]string(nil), ents[:n]...)
}

// LooksLikeConversationalFragment filters sentence-like spans mistaken as retrieval entities.
func LooksLikeConversationalFragment(s string) bool {
	s = strings.TrimSpace(s)
	if s == "" {
		return false
	}
	runes := []rune(s)
	if len(runes) < 8 {
		return false
	}
	if strings.ContainsAny(s, "，。！？,.!?：:；; ") {
		return true
	}
	return len(reEntityHanCore.FindAllString(s, -1)) == 1
}

// IsStopEntity reports structurally invalid non-entity tokens.
func IsStopEntity(s string) bool {
	s = strings.TrimSpace(s)
	if s == "" {
		return true
	}
	return LooksLikeConversationalFragment(s) || isSpuriousLongEntity(s)
}
