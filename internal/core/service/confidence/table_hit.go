package confidence

import (
	"strings"

	"github.com/graydraft/core/model"
)

const structuredTableBonus = 0.10

// TableKeywordHit reports whether query terms appear inside a markdown table in chunk content.
func TableKeywordHit(query string, content string) bool {
	content = strings.TrimSpace(content)
	if content == "" || !looksLikeFactTable(content) {
		return false
	}
	lower := strings.ToLower(content)
	for _, term := range QueryTerms(query) {
		if len([]rune(term)) < 2 {
			continue
		}
		if strings.Contains(lower, strings.ToLower(term)) {
			return true
		}
	}
	return false
}

func looksLikeFactTable(s string) bool {
	return strings.Contains(s, "|") && (strings.Contains(s, "| 主题 |") || strings.Contains(s, "| :---"))
}

// StructuredTableBonus adds confidence when top chunks carry fact-table rows matching the query.
func StructuredTableBonus(query string, chunks []model.Chunk) float64 {
	if len(chunks) == 0 {
		return 0
	}
	// Prefer the highest-scoring chunk first.
	best := chunks[0]
	for _, c := range chunks[1:] {
		if c.RerankScore > best.RerankScore || (best.RerankScore <= 0 && c.Score > best.Score) {
			best = c
		}
	}
	if TableKeywordHit(query, best.Content) {
		return structuredTableBonus
	}
	for _, c := range chunks {
		if TableKeywordHit(query, c.Content) {
			return structuredTableBonus
		}
	}
	return 0
}
