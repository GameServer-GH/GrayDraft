package postgres

import (
	"strings"
	"unicode"
)

// extractSearchTerms splits a query into tokens for Chinese-friendly ILIKE fallback.
func extractSearchTerms(query string) []string {
	q := strings.TrimSpace(query)
	if q == "" {
		return nil
	}
	seen := make(map[string]struct{})
	var terms []string
	add := func(s string) {
		s = strings.TrimSpace(s)
		if len([]rune(s)) < 2 {
			return
		}
		if _, ok := seen[s]; ok {
			return
		}
		seen[s] = struct{}{}
		terms = append(terms, s)
	}

	add(q)
	for _, part := range strings.FieldsFunc(q, func(r rune) bool {
		return unicode.IsSpace(r) || unicode.IsPunct(r)
	}) {
		add(part)
	}

	runes := []rune(q)
	if len(runes) > 4 {
		for i := 0; i <= len(runes)-2; i++ {
			add(string(runes[i : i+2]))
		}
	}
	return terms
}
