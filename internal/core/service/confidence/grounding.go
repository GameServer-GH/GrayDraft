package confidence

import (
	"strings"
	"unicode"

	"github.com/graydraft/core/model"
)

// PassesHighQualityGrounding allows RAG when top chunk quality and retrieval signals align with the query.
// Domain-specific keyword lists are intentionally avoided; use TitleMatched/EntityMatched or token overlap.
func PassesHighQualityGrounding(query string, chunks []model.Chunk) bool {
	return passesHighQualityGrounding(query, chunks)
}

// passesHighQualityGrounding is the internal implementation (used by gate.go).
func passesHighQualityGrounding(query string, chunks []model.Chunk) bool {
	if len(chunks) == 0 {
		return false
	}
	c := chunks[0]
	if c.QualityScore < 0.72 {
		return false
	}
	if c.TitleMatched || c.EntityMatched {
		return true
	}
	if c.QualityScore >= 0.82 && queryGroundedInChunk(query, c) {
		return true
	}
	return false
}

func queryGroundedInChunk(query string, c model.Chunk) bool {
	text := strings.ToLower(c.Content + " " + c.DocTitle)
	for _, t := range groundingTerms(query) {
		if len([]rune(t)) < 2 {
			continue
		}
		if strings.Contains(text, strings.ToLower(t)) {
			return true
		}
	}
	return false
}

func groundingTerms(query string) []string {
	var out []string
	seen := map[string]bool{}
	add := func(s string) {
		s = strings.TrimSpace(s)
		if s == "" || seen[s] {
			return
		}
		seen[s] = true
		out = append(out, s)
	}
	for _, seg := range strings.FieldsFunc(query, func(r rune) bool {
		return unicode.IsSpace(r) || r == '，' || r == '。' || r == '？' || r == '！' || r == ','
	}) {
		add(seg)
	}
	for _, core := range model.FinalSanitize(nil, query) {
		add(core)
	}
	return out
}
