package taxonomyevolve

import "strings"

// SimilarityRatio returns normalized similarity in [0,1] (1 = identical).
// Uses rune-level Levenshtein; cheap pre-filter before optional LLM merge.
func SimilarityRatio(a, b string) float64 {
	a = strings.TrimSpace(strings.ToLower(a))
	b = strings.TrimSpace(strings.ToLower(b))
	if a == "" || b == "" {
		return 0
	}
	if a == b {
		return 1
	}
	if strings.Contains(a, b) || strings.Contains(b, a) {
		minLen := len([]rune(a))
		if l := len([]rune(b)); l < minLen {
			minLen = l
		}
		maxLen := len([]rune(a))
		if l := len([]rune(b)); l > maxLen {
			maxLen = l
		}
		if maxLen == 0 {
			return 0
		}
		return float64(minLen) / float64(maxLen)
	}
	ra, rb := []rune(a), []rune(b)
	dist := levenshtein(ra, rb)
	maxLen := len(ra)
	if lb := len(rb); lb > maxLen {
		maxLen = lb
	}
	if maxLen == 0 {
		return 0
	}
	return 1 - float64(dist)/float64(maxLen)
}

func levenshtein(a, b []rune) int {
	if len(a) == 0 {
		return len(b)
	}
	if len(b) == 0 {
		return len(a)
	}
	prev := make([]int, len(b)+1)
	cur := make([]int, len(b)+1)
	for j := range prev {
		prev[j] = j
	}
	for i := 1; i <= len(a); i++ {
		cur[0] = i
		for j := 1; j <= len(b); j++ {
			cost := 1
			if a[i-1] == b[j-1] {
				cost = 0
			}
			cur[j] = min3(cur[j-1]+1, prev[j]+1, prev[j-1]+cost)
		}
		prev, cur = cur, prev
	}
	return prev[len(b)]
}

func min3(a, b, c int) int {
	if a <= b && a <= c {
		return a
	}
	if b <= c {
		return b
	}
	return c
}

// BestKeywordMatch finds the highest similarity against existing keywords for a label.
func BestKeywordMatch(phrase string, keywords []string) (best string, ratio float64) {
	for _, kw := range keywords {
		r := SimilarityRatio(phrase, kw)
		if r > ratio {
			ratio = r
			best = kw
		}
	}
	return best, ratio
}
