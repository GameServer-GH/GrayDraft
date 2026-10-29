package model

import (
	"sort"
	"strings"
)

// QuerySemantic4D is extracted from a user retrieval/chat query for metadata pre-filtering.
type QuerySemantic4D struct {
	Domain    string   `json:"domain,omitempty"`
	Entities  []string `json:"entities,omitempty"`
	Scene     string   `json:"scene,omitempty"`
	Style     string   `json:"style,omitempty"`
	Method    string   `json:"method,omitempty"` // rules | llm | ""
	Reasoning string   `json:"reasoning,omitempty"` // LLM NER rationale (optional)
}

// Active reports whether any dimension is set for filtering.
func (q QuerySemantic4D) Active() bool {
	return q.Domain != "" || q.Scene != "" || q.Style != "" || len(q.Entities) > 0
}

// CacheFingerprint returns a stable key segment for retrieval cache when 4D pre-filter is active.
// Empty when no dimensions are set. Method is excluded (rules vs llm must not split cache).
func (q QuerySemantic4D) CacheFingerprint() string {
	if !q.Active() {
		return ""
	}
	ents := make([]string, 0, len(q.Entities))
	for _, e := range q.Entities {
		e = strings.ToLower(strings.TrimSpace(e))
		if e != "" {
			ents = append(ents, e)
		}
	}
	sort.Strings(ents)
	return strings.Join([]string{
		strings.ToLower(strings.TrimSpace(q.Domain)),
		strings.Join(ents, ","),
		strings.ToLower(strings.TrimSpace(q.Scene)),
		strings.ToLower(strings.TrimSpace(q.Style)),
	}, "|")
}

// AtCore returns domain + all sanitized entities without scene/style (waterfall tier 2).
func (q QuerySemantic4D) AtCore() *QuerySemantic4D {
	if !q.Active() {
		return nil
	}
	out := q
	out.Scene = ""
	out.Style = ""
	if !out.Active() {
		return nil
	}
	return &out
}

// AtEntityMin returns domain + single shortest entity (waterfall tier 3).
func (q QuerySemantic4D) AtEntityMin() *QuerySemantic4D {
	if strings.TrimSpace(q.Domain) == "" {
		return nil
	}
	ents := SlimEntities(q.Entities)
	if len(ents) == 0 {
		return nil
	}
	out := q
	out.Scene = ""
	out.Style = ""
	out.Entities = ents
	return &out
}

// AtDomainOnly returns a copy with only domain (waterfall tier 4 — vector/fulltext with domain gate).
func (q QuerySemantic4D) AtDomainOnly() *QuerySemantic4D {
	if strings.TrimSpace(q.Domain) == "" {
		return nil
	}
	out := q
	out.Scene = ""
	out.Style = ""
	out.Entities = nil
	return &out
}
