// Package smartimport provides rule-based confidence labeling that replaces
// pseudo-precise continuous scores (0.25/0.20/0.15...) with explainable
// three-tier labels: Pass, Warning, Reject.
package smartimport

import (
	"strings"
)

// ConfidenceLabel is the rule-based document quality tier.
type ConfidenceLabel string

const (
	// LabelPass means all checks passed; document can be submitted.
	LabelPass ConfidenceLabel = "pass"
	// LabelWarning means 1-2 non-critical checks failed; submit with frontend warning.
	LabelWarning ConfidenceLabel = "warning"
	// LabelReject means critical checks failed or >=3 checks failed; reject submission.
	LabelReject ConfidenceLabel = "reject"
)

// ConfidenceCheckResult holds results for all five quality checks.
type ConfidenceCheckResult struct {
	Label          ConfidenceLabel `json:"label"`
	StructureOk    bool            `json:"structure_ok"`
	EntityOk       bool            `json:"entity_ok"`
	NoiseOk        bool            `json:"noise_ok"`
	TitleOk        bool            `json:"title_ok"`
	EntityTraceOk  bool            `json:"entity_trace_ok"`
	ShortExempt    bool            `json:"short_exempt"`
	FailedChecks   []string        `json:"failed_checks,omitempty"`
	Explain        string          `json:"explain"`
}

// EvaluateConfidence runs all five rule-based checks and returns a label.
//
// Checks:
//   - StructureOk: skeleton has >= 1 heading.
//   - EntityOk: info density >= 0.06.
//   - NoiseOk: ngram repeat rate <= 0.5.
//   - TitleOk: title passes substring/edit-distance/LCS validation.
//   - EntityTraceOk: >= 50% entities found in whitelist or source text.
//
// Short-knowledge exemption: < 80 chars AND >= 1 proper noun, if TitleOk
// and EntityTraceOk both pass, downgrades to Warning regardless of density.
func EvaluateConfidence(sk DocumentSkeleton, content, title string, entityTraceRatio float64, ngramRepeatRate float64) ConfidenceCheckResult {
	r := ConfidenceCheckResult{}

	r.StructureOk = hasStructureHeadings(sk)
	r.EntityOk = !IsNoiseByInfoDensity(content)
	r.NoiseOk = ngramRepeatRate <= 0.5
	r.TitleOk = titlePassesValidation(title, content)
	r.EntityTraceOk = entityTraceRatio >= 0.5

	// Short-knowledge exemption
	runeLen := ContentRuneLen(content)
	r.ShortExempt = runeLen < 80 && EntityCountFromContent(content) >= 1 && r.TitleOk && r.EntityTraceOk

	var fails []string
	if !r.StructureOk {
		fails = append(fails, "structure")
	}
	if !r.EntityOk {
		fails = append(fails, "entity")
	}
	if !r.NoiseOk {
		fails = append(fails, "noise")
	}
	if !r.TitleOk {
		fails = append(fails, "title")
	}
	if !r.EntityTraceOk {
		fails = append(fails, "entity_trace")
	}
	r.FailedChecks = fails

	failCount := len(fails)

	switch {
	case r.ShortExempt:
		r.Label = LabelWarning
		r.Explain = "short knowledge exemption: low char count but has proper nouns"
	case failCount == 0:
		r.Label = LabelPass
		r.Explain = "all checks passed"
	case r.EntityOk && r.NoiseOk && failCount <= 2:
		r.Label = LabelWarning
		r.Explain = "non-critical checks failed: " + strings.Join(fails, ", ")
	default:
		r.Label = LabelReject
		r.Explain = "critical or multiple checks failed: " + strings.Join(fails, ", ")
	}

	return r
}

// hasStructureHeadings returns true if the skeleton has at least one heading.
func hasStructureHeadings(sk DocumentSkeleton) bool {
	for _, sl := range sk.Lines {
		switch sl.Kind {
		case "h1", "h2", "h3", "h4":
			return true
		}
	}
	return false
}

// titlePassesValidation checks the title against content using multi-condition OR matching.
func titlePassesValidation(title, content string) bool {
	if title == "" {
		return false
	}
	if titleAppearsInContent(title, content) {
		return true
	}
	if titleEditDistanceMatch(title, content) {
		return true
	}
	if titleLCSMatch(title, content) {
		return true
	}
	return false
}

// titleEditDistanceMatch returns true if edit distance between title and any content line <= 3.
func titleEditDistanceMatch(title, content string) bool {
	for _, line := range strings.Split(content, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if levenshteinDistance([]rune(title), []rune(line)) <= 3 {
			return true
		}
	}
	return false
}

// titleLCSMatch returns true if LCS ratio >= 0.70.
func titleLCSMatch(title, content string) bool {
	titleRunes := []rune(title)
	if len(titleRunes) == 0 {
		return false
	}
	for _, line := range strings.Split(content, "\n") {
		lineRunes := []rune(strings.TrimSpace(line))
		if len(lineRunes) == 0 {
			continue
		}
		lcsLen := lcsLength(titleRunes, lineRunes)
		shorter := len(titleRunes)
		if len(lineRunes) < shorter {
			shorter = len(lineRunes)
		}
		if float64(lcsLen)/float64(shorter) >= 0.70 {
			return true
		}
	}
	return false
}

// levenshteinDistance computes the Levenshtein edit distance between two rune slices.
func levenshteinDistance(a, b []rune) int {
	la, lb := len(a), len(b)
	if la == 0 {
		return lb
	}
	if lb == 0 {
		return la
	}
	dp := make([][]int, la+1)
	for i := range dp {
		dp[i] = make([]int, lb+1)
		dp[i][0] = i
	}
	for j := 0; j <= lb; j++ {
		dp[0][j] = j
	}
	for i := 1; i <= la; i++ {
		for j := 1; j <= lb; j++ {
			cost := 1
			if a[i-1] == b[j-1] {
				cost = 0
			}
			dp[i][j] = minInt3(dp[i-1][j]+1, dp[i][j-1]+1, dp[i-1][j-1]+cost)
		}
	}
	return dp[la][lb]
}

// lcsLength returns the length of the longest common subsequence.
func lcsLength(a, b []rune) int {
	la, lb := len(a), len(b)
	if la == 0 || lb == 0 {
		return 0
	}
	prev := make([]int, lb+1)
	curr := make([]int, lb+1)
	for i := 1; i <= la; i++ {
		for j := 1; j <= lb; j++ {
			if a[i-1] == b[j-1] {
				curr[j] = prev[j-1] + 1
			} else {
				curr[j] = prev[j]
				if curr[j-1] > curr[j] {
					curr[j] = curr[j-1]
				}
			}
		}
		prev, curr = curr, prev
	}
	return prev[lb]
}

func minInt3(a, b, c int) int {
	if a < b {
		if a < c {
			return a
		}
		return c
	}
	if b < c {
		return b
	}
	return c
}

// NgramRepeatRate computes the n-gram repeat rate as a signal for OCR error loops.
// Uses bigrams; rate above 0.5 suggests repetitive garbage text.
func NgramRepeatRate(content string) float64 {
	runes := []rune(strings.TrimSpace(content))
	if len(runes) < 4 {
		return 0
	}
	ngrams := map[string]int{}
	total := 0
	for i := 0; i < len(runes)-1; i++ {
		ng := string(runes[i : i+2])
		ngrams[ng]++
		total++
	}
	if total == 0 {
		return 0
	}
	repeated := 0
	for _, c := range ngrams {
		if c > 1 {
			repeated += c - 1
		}
	}
	return float64(repeated) / float64(total)
}
