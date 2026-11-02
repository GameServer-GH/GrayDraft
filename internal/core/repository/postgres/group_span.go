package postgres

import (
	"regexp"
	"strings"
	"unicode"

	"github.com/graydraft/core/model"
)

var reSlicePart = regexp.MustCompile(`[（(](\d+)/\d+[）)]`)
var reDecimalPercent = regexp.MustCompile(`\d+\.\d+\s*%`)

// pickGroupSpan keeps the anchor chunk and the chunks in the same slice or the
// next two slices that share a query token the group title does not already contain.
func pickGroupSpan(all []model.Chunk, anchorTitle, anchorID, query string, limit int) []model.Chunk {
	if len(all) == 0 {
		return nil
	}
	if limit <= 0 {
		limit = 8
	}
	group := sliceGroupTitle(anchorTitle)
	part, _ := slicePart(anchorTitle)
	terms := residualTerms(query, group)
	wantDecimal := strings.Contains(query, "分数") || strings.Contains(query, "提升") || strings.Contains(query, "多少")
	partScore := map[int]int{}
	byPart := map[int][]model.Chunk{}
	var anchor model.Chunk
	hasAnchor := false
	for _, c := range all {
		p, pok := slicePart(c.DocTitle)
		if !pok {
			p = part
		}
		byPart[p] = append(byPart[p], c)
		if c.ID == anchorID {
			anchor = c
			hasAnchor = true
		}
		if chunkHasTerm(c.Content, terms) {
			partScore[p]++
		}
		if wantDecimal && strings.Contains(c.Content, "%") && reDecimalPercent.MatchString(c.Content) {
			partScore[p] += 3
		}
	}
	bestPart := part
	best := -1
	for p, s := range partScore {
		if s > best || (s == best && s > 0 && p < bestPart) {
			best = s
			bestPart = p
		}
	}
	picked := append([]model.Chunk{}, byPart[bestPart]...)
	if hasAnchor && anchor.DocTitle != "" && slicePartOr(anchor.DocTitle, bestPart) != bestPart {
		picked = append([]model.Chunk{anchor}, picked...)
	}
	if len(picked) > limit {
		// The selected slice's opening defines the term. Later repetitions stay out.
		picked = picked[:limit]
	}
	return picked
}

func slicePartOr(title string, fallback int) int {
	if p, ok := slicePart(title); ok {
		return p
	}
	return fallback
}

func sliceGroupTitle(title string) string {
	t := strings.TrimSpace(title)
	t = strings.TrimPrefix(t, "论文 ")
	t = strings.TrimSpace(reSlicePart.ReplaceAllString(t, ""))
	return t
}

func slicePart(title string) (int, bool) {
	m := reSlicePart.FindStringSubmatch(title)
	if len(m) < 2 {
		return 0, false
	}
	n := 0
	for _, r := range m[1] {
		n = n*10 + int(r-'0')
	}
	return n, true
}

func residualTerms(query, group string) []string {
	group = strings.ToLower(group)
	var terms []string
	seen := map[string]bool{}
	for _, raw := range strings.FieldsFunc(query, func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsDigit(r)
	}) {
		t := strings.TrimSpace(raw)
		if len([]rune(t)) < 2 {
			continue
		}
		low := strings.ToLower(t)
		if strings.Contains(group, low) || seen[low] {
			continue
		}
		seen[low] = true
		terms = append(terms, low)
	}
	return terms
}

func pickGroupOpenings(all []model.Chunk, groups []string) []model.Chunk {
	want := map[string]bool{}
	for _, g := range groups {
		g = strings.TrimSpace(g)
		if g != "" {
			want[g] = true
		}
	}
	bestPart := map[string]int{}
	bestDoc := map[string]string{}
	for _, c := range all {
		g := sliceGroupTitle(c.DocTitle)
		if !want[g] {
			continue
		}
		part, ok := slicePart(c.DocTitle)
		if !ok {
			part = 0
		}
		cur, seen := bestPart[g]
		if !seen || part < cur {
			bestPart[g] = part
			bestDoc[g] = c.DocID
		}
	}
	out := make([]model.Chunk, 0, len(bestDoc)*4)
	for _, g := range groups {
		docID := bestDoc[strings.TrimSpace(g)]
		if docID == "" {
			continue
		}
		for _, c := range all {
			if c.DocID == docID {
				out = append(out, c)
			}
		}
	}
	return out
}

func chunkHasTerm(content string, terms []string) bool {
	low := strings.ToLower(content)
	for _, t := range terms {
		if strings.Contains(low, t) {
			return true
		}
	}
	return false
}
