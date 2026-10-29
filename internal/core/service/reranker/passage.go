package reranker

import (
	"fmt"
	"strings"

	"github.com/graydraft/core/model"
)

// BuildRerankPassage formats generic semantic identity for rerank (GrayDraft 3.5).
func BuildRerankPassage(c model.Chunk) string {
	title := strings.TrimSpace(c.DocTitle)
	if title == "" {
		title = c.DocID
	}
	anchors := formatSearchAnchors(c.Extensions)
	body := strings.TrimSpace(c.Content)
	return fmt.Sprintf("[主题：%s] [关联词：%s] [正文：%s]", title, anchors, body)
}

func formatSearchAnchors(ext model.Extensions) string {
	if len(ext) == 0 {
		return ""
	}
	parts := append([]string{}, scalarAsString(ext["search_anchors"])...)
	if v, ok := ext["aliases"]; ok {
		parts = append(parts, scalarAsString(v)...)
	}
	if len(parts) == 0 {
		return ""
	}
	seen := make(map[string]struct{})
	var out []string
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		key := strings.ToLower(p)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, p)
	}
	return strings.Join(out, ", ")
}

func scalarAsString(v any) []string {
	switch t := v.(type) {
	case string:
		if strings.TrimSpace(t) != "" {
			return []string{strings.TrimSpace(t)}
		}
	case []any:
		var parts []string
		for _, item := range t {
			parts = append(parts, scalarAsString(item)...)
		}
		return parts
	case []string:
		return t
	}
	return nil
}
