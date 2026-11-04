package postgres

import (
	"regexp"
	"sort"
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
	// Revision-aware documents must not be merged by title or UUID order.
	for _, anchor := range all {
		if anchor.ID != anchorID {
			continue
		}
		revision, _ := anchor.DocExtensions["source_revision_sha256"].(string)
		if revision != "" {
			return pickRevisionSpan(all, anchor, revision, limit)
		}
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
	if hasAnchor {
		rest := make([]model.Chunk, 0, len(picked))
		for _, c := range picked {
			if c.ID != anchor.ID {
				rest = append(rest, c)
			}
		}
		picked = append([]model.Chunk{anchor}, rest...)
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
		}
	}
	ordered := append([]model.Chunk(nil), all...)
	segment := func(c model.Chunk) float64 {
		if v, ok := c.DocExtensions["segment_index"].(float64); ok {
			return v
		}
		return 0
	}
	sort.SliceStable(ordered, func(i, j int) bool {
		a, b := ordered[i], ordered[j]
		if segment(a) != segment(b) {
			return segment(a) < segment(b)
		}
		if a.DocID != b.DocID {
			return a.DocID < b.DocID
		}
		return a.ChunkIndex < b.ChunkIndex
	})
	out := make([]model.Chunk, 0)
	for _, g := range groups {
		perDoc := map[string]int{}
		count := 0
		for _, c := range ordered {
			part, _ := slicePart(c.DocTitle)
			if sliceGroupTitle(c.DocTitle) == g && part == bestPart[g] && perDoc[c.DocID] < 4 && count < 12 {
				out = append(out, c)
				perDoc[c.DocID]++
				count++
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

// Reserve source openings alongside the anchor neighborhood. Openings contain
// definitions and scope that a later table or local observation may omit.
func pickRevisionSpan(all []model.Chunk, anchor model.Chunk, revision string, limit int) []model.Chunk {
	ordered := make([]model.Chunk, 0)
	for _, c := range all {
		if c.DocExtensions["source_revision_sha256"] == revision {
			ordered = append(ordered, c)
		}
	}
	segment := func(c model.Chunk) float64 { v, _ := c.DocExtensions["segment_index"].(float64); return v }
	sort.SliceStable(ordered, func(i, j int) bool {
		a, b := ordered[i], ordered[j]
		if segment(a) != segment(b) {
			return segment(a) < segment(b)
		}
		if a.DocID != b.DocID {
			return a.DocID < b.DocID
		}
		if a.ChunkIndex != b.ChunkIndex {
			return a.ChunkIndex < b.ChunkIndex
		}
		return a.ID < b.ID
	})
	out := []model.Chunk{}
	seen := map[string]bool{}
	add := func(c model.Chunk) {
		if len(out) < limit && !seen[c.ID] {
			out = append(out, c)
			seen[c.ID] = true
		}
	}
	add(anchor)
	openings := limit / 2
	for i := 0; i < len(ordered) && i < openings; i++ {
		add(ordered[i])
	}
	at := 0
	for i, c := range ordered {
		if c.ID == anchor.ID {
			at = i
			break
		}
	}
	for distance := 1; distance < len(ordered) && len(out) < limit; distance++ {
		if at-distance >= 0 {
			add(ordered[at-distance])
		}
		if at+distance < len(ordered) {
			add(ordered[at+distance])
		}
	}
	return out
}
