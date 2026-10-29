package classifier

import (
	"strings"
	"unicode"
)

// Intent synonym groups for alias alignment (参数/接口/说明 …).
var intentSynonymGroups = [][]string{
	{"参数", "接口", "介绍", "说明", "速查", "教程"},
	{"公式", "推导", "定义", "定理", "证明"},
	{"部署", "配置", "调用", "示例"},
}

// AliasIndex maps equivalent entity terms within a knowledge partition.
type AliasIndex struct {
	groups [][]string
}

// DocAliasSource is one document's identity fields for alias grouping.
type DocAliasSource struct {
	Title      string
	Tags       []string
	Extensions map[string]any
}

// BuildAliasIndex unions overlapping identity terms across documents.
func BuildAliasIndex(docs []DocAliasSource) *AliasIndex {
	var groups [][]string
	for _, doc := range docs {
		terms := collectDocTerms(doc)
		if len(terms) == 0 {
			continue
		}
		merged := terms
		var next [][]string
		for _, g := range groups {
			if setsOverlap(g, merged) {
				merged = unionStrings(g, merged)
				continue
			}
			next = append(next, g)
		}
		groups = append(next, merged)
	}
	return &AliasIndex{groups: groups}
}

// ExpandTerms returns deduplicated query tokens plus alias/intent expansions.
func (idx *AliasIndex) ExpandTerms(query string) []string {
	base := tokenizeQueryTerms(query)
	if idx == nil {
		return appendIntentSynonyms(base, query)
	}
	seen := make(map[string]struct{}, len(base)+8)
	out := make([]string, 0, len(base)+8)
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
		out = append(out, s)
	}
	for _, t := range base {
		add(t)
	}
	for _, t := range base {
		for _, g := range idx.groups {
			if termInGroup(t, g) {
				for _, alias := range g {
					add(alias)
				}
			}
		}
	}
	for _, extra := range appendIntentSynonyms(nil, query) {
		add(extra)
	}
	return out
}

// CanonicalEntity returns the shortest term in the alias group containing term (atomic ID preference).
func (idx *AliasIndex) CanonicalEntity(term string) string {
	term = strings.TrimSpace(term)
	if idx == nil || term == "" {
		return term
	}
	for _, g := range idx.groups {
		if !termInGroup(term, g) {
			continue
		}
		best := term
		for _, m := range g {
			m = strings.TrimSpace(m)
			if m == "" {
				continue
			}
			if len([]rune(m)) < len([]rune(best)) {
				best = m
			}
		}
		return best
	}
	return term
}

// CanonicalizeEntities deduplicates entities after alias canonicalization.
func (idx *AliasIndex) CanonicalizeEntities(entities []string) []string {
	if idx == nil || len(entities) == 0 {
		return entities
	}
	seen := map[string]bool{}
	out := make([]string, 0, len(entities))
	for _, e := range entities {
		canon := strings.TrimSpace(idx.CanonicalEntity(e))
		if canon == "" {
			continue
		}
		key := strings.ToLower(canon)
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, canon)
	}
	if len(out) == 0 {
		return entities
	}
	return out
}

// EntityMatched reports whether query hits an alias group represented in doc identity.
func (idx *AliasIndex) EntityMatched(query, docTitle string, extensions map[string]any) bool {
	if idx == nil {
		return entityDirectMatch(query, docTitle, extensions)
	}
	qTerms := tokenizeQueryTerms(query)
	docTerms := collectDocTerms(DocAliasSource{
		Title:      docTitle,
		Extensions: extensions,
	})
	for _, qt := range qTerms {
		for _, g := range idx.groups {
			if !termInGroup(qt, g) {
				continue
			}
			for _, dt := range docTerms {
				if termInGroup(dt, g) {
					return true
				}
			}
		}
	}
	return entityDirectMatch(query, docTitle, extensions)
}

func entityDirectMatch(query, docTitle string, extensions map[string]any) bool {
	title := strings.ToLower(strings.TrimSpace(docTitle))
	if title == "" {
		return false
	}
	for _, t := range tokenizeQueryTerms(query) {
		if strings.Contains(title, strings.ToLower(t)) {
			return true
		}
	}
	for _, v := range extensionValues(extensions) {
		lv := strings.ToLower(v)
		for _, t := range tokenizeQueryTerms(query) {
			if strings.Contains(lv, strings.ToLower(t)) {
				return true
			}
		}
	}
	return false
}

func collectDocTerms(doc DocAliasSource) []string {
	seen := make(map[string]struct{})
	var out []string
	add := func(s string) {
		s = strings.TrimSpace(s)
		if s == "" {
			return
		}
		key := strings.ToLower(s)
		if _, ok := seen[key]; ok {
			return
		}
		seen[key] = struct{}{}
		out = append(out, s)
	}
	add(doc.Title)
	for _, tag := range doc.Tags {
		add(tag)
	}
	for _, v := range extensionValues(doc.Extensions) {
		add(v)
	}
	return out
}

func extensionValues(ext map[string]any) []string {
	if len(ext) == 0 {
		return nil
	}
	var out []string
	for _, key := range []string{"search_anchors", "aliases", "auto_keywords", "parent_title"} {
		if v, ok := ext[key]; ok {
			out = append(out, scalarStrings(v)...)
		}
	}
	if raw, ok := ext["semantic_tags_4d"].(map[string]any); ok {
		out = append(out, scalarStrings(raw["entities"])...)
	}
	return out
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

func appendIntentSynonyms(base []string, query string) []string {
	seen := make(map[string]struct{})
	for _, b := range base {
		seen[strings.ToLower(b)] = struct{}{}
	}
	var out []string
	if base != nil {
		out = append(out, base...)
	}
	q := strings.ToLower(query)
	for _, group := range intentSynonymGroups {
		hit := false
		for _, w := range group {
			if strings.Contains(q, strings.ToLower(w)) {
				hit = true
				break
			}
		}
		if !hit {
			continue
		}
		for _, w := range group {
			if _, ok := seen[strings.ToLower(w)]; ok {
				continue
			}
			seen[strings.ToLower(w)] = struct{}{}
			out = append(out, w)
		}
	}
	return out
}

func tokenizeQueryTerms(query string) []string {
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

func termInGroup(term string, group []string) bool {
	lt := strings.ToLower(strings.TrimSpace(term))
	for _, g := range group {
		g = strings.ToLower(strings.TrimSpace(g))
		if g == "" {
			continue
		}
		if lt == g || strings.Contains(lt, g) || strings.Contains(g, lt) {
			return true
		}
	}
	return false
}

func setsOverlap(a, b []string) bool {
	for _, x := range a {
		for _, y := range b {
			if strings.EqualFold(strings.TrimSpace(x), strings.TrimSpace(y)) {
				return true
			}
		}
	}
	return false
}

func unionStrings(a, b []string) []string {
	seen := make(map[string]struct{})
	var out []string
	for _, list := range [][]string{a, b} {
		for _, s := range list {
			s = strings.TrimSpace(s)
			if s == "" {
				continue
			}
			key := strings.ToLower(s)
			if _, ok := seen[key]; ok {
				continue
			}
			seen[key] = struct{}{}
			out = append(out, s)
		}
	}
	return out
}
