package confidence

import (
	"strings"
	"unicode"
)

const titleHitBonus = 0.15

// TitleHit reports whether any query keyword appears in the document title or search anchors.
func TitleHit(query, docTitle string, extensions map[string]any) bool {
	title := strings.TrimSpace(docTitle)
	if title == "" {
		return false
	}
	titleLower := strings.ToLower(title)
	for _, term := range QueryTerms(query) {
		lt := strings.ToLower(term)
		if strings.Contains(titleLower, lt) {
			return true
		}
	}
	for _, anchor := range anchorTerms(extensions) {
		al := strings.ToLower(anchor)
		for _, term := range QueryTerms(query) {
			lt := strings.ToLower(term)
			if lt == al || strings.Contains(al, lt) || strings.Contains(lt, al) {
				return true
			}
		}
	}
	return false
}

func anchorTerms(ext map[string]any) []string {
	if len(ext) == 0 {
		return nil
	}
	var out []string
	if v, ok := ext["search_anchors"]; ok {
		out = append(out, scalarStrings(v)...)
	}
	if v, ok := ext["aliases"]; ok {
		out = append(out, scalarStrings(v)...)
	}
	return out
}

// QueryTerms splits a query into searchable tokens (CJK bigrams + words).
func QueryTerms(query string) []string {
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
		key := strings.ToLower(s)
		if _, ok := seen[key]; ok {
			return
		}
		seen[key] = struct{}{}
		terms = append(terms, s)
	}
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

func scalarStrings(v any) []string {
	switch t := v.(type) {
	case string:
		if strings.TrimSpace(t) != "" {
			return []string{strings.TrimSpace(t)}
		}
	case []any:
		var out []string
		for _, item := range t {
			out = append(out, scalarStrings(item)...)
		}
		return out
	case []string:
		var out []string
		for _, s := range t {
			if strings.TrimSpace(s) != "" {
				out = append(out, strings.TrimSpace(s))
			}
		}
		return out
	}
	return nil
}
