package agent

import (
	"regexp"
	"strings"
)

type GroundednessResult struct {
	Claims            int     `json:"claims"`
	Supported         int     `json:"supported"`
	Groundedness      float64 `json:"groundedness"`
	CitationPrecision float64 `json:"citation_precision"`
	CitationRecall    float64 `json:"citation_recall"`
	ShouldRefuse      bool    `json:"should_refuse"`
}

var sentenceRE = regexp.MustCompile(`[。！？!?；;\n]+`)

func EvaluateGroundedness(answer string, evidence []string, citationCount, requiredCitations int) GroundednessResult {
	parts := sentenceRE.Split(strings.TrimSpace(answer), -1)
	var claims []string
	for _, p := range parts {
		if strings.TrimSpace(p) != "" {
			claims = append(claims, strings.TrimSpace(p))
		}
	}
	r := GroundednessResult{Claims: len(claims)}
	for _, claim := range claims {
		words := contentWords(claim)
		supported := false
		for _, e := range evidence {
			if overlap(words, contentWords(e)) >= .35 {
				supported = true
				break
			}
		}
		if supported {
			r.Supported++
		}
	}
	if r.Claims > 0 {
		r.Groundedness = float64(r.Supported) / float64(r.Claims)
	}
	if citationCount > 0 {
		r.CitationPrecision = r.Groundedness
	}
	if r.Claims > 0 {
		r.CitationRecall = float64(citationCount) / float64(r.Claims)
		if r.CitationRecall > 1 {
			r.CitationRecall = 1
		}
	}
	r.ShouldRefuse = r.Claims > 0 && r.Groundedness < .62
	if requiredCitations > 0 && citationCount < requiredCitations {
		r.ShouldRefuse = true
	}
	return r
}
func contentWords(s string) map[string]bool {
	out := map[string]bool{}
	clean := strings.NewReplacer("。", "", "，", "", "！", "", "？", "", ".", "", ",", "", "!", "", "?", "").Replace(strings.ToLower(s))
	fields := strings.Fields(clean)
	for _, w := range fields {
		r := []rune(w)
		if len(r) >= 2 {
			out[w] = true
		}
		if len(fields) == 1 {
			for i := 0; i+1 < len(r); i++ {
				out[string(r[i:i+2])] = true
			}
		}
	}
	return out
}
func overlap(a, b map[string]bool) float64 {
	if len(a) == 0 {
		return 0
	}
	n := 0
	for w := range a {
		if b[w] {
			n++
		}
	}
	return float64(n) / float64(len(a))
}
