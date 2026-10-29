package agent

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"
)

type GoldenCase struct {
	ID                string   `json:"id"`
	Category          string   `json:"category"`
	Query             string   `json:"query"`
	TenantID          string   `json:"tenant_id"`
	Version           string   `json:"version,omitempty"`
	ExpectedKeywords  []string `json:"expected_keywords,omitempty"`
	ExpectedTools     []string `json:"expected_tools,omitempty"`
	MustRefuse        bool     `json:"must_refuse,omitempty"`
	RequiredCitations int      `json:"required_citations,omitempty"`
}
type ReplayResult struct {
	CaseID        string
	Answer        string
	Tools         []string
	CitationCount int
	Refused       bool
}
type CaseScore struct {
	CaseID        string   `json:"case_id"`
	Category      string   `json:"category,omitempty"`
	Passed        bool     `json:"passed"`
	KeywordScore  float64  `json:"keyword_score"`
	ToolScore     float64  `json:"tool_score"`
	CitationScore float64  `json:"citation_score"`
	Errors        []string `json:"errors,omitempty"`
}
type CategoryScore struct {
	Category string  `json:"category"`
	Total    int     `json:"total"`
	Passed   int     `json:"passed"`
	PassRate float64 `json:"pass_rate"`
}
type EvalMetrics struct {
	ToolSuccessRate    float64 `json:"tool_success_rate"`
	RefusalAccuracy    float64 `json:"refusal_accuracy"`
	AvgCitationScore   float64 `json:"avg_citation_score"`
	AvgKeywordScore    float64 `json:"avg_keyword_score"`
}
type EvaluationReport struct {
	Total      int             `json:"total"`
	Passed     int             `json:"passed"`
	PassRate   float64         `json:"pass_rate"`
	Cases      []CaseScore     `json:"cases"`
	Categories []CategoryScore `json:"categories,omitempty"`
	Metrics    EvalMetrics     `json:"metrics,omitempty"`
}
type ReplayExecutor func(GoldenCase) (ReplayResult, error)

func ReplayDataset(cases []GoldenCase, execute ReplayExecutor) (EvaluationReport, error) {
	var results []ReplayResult
	for _, c := range cases {
		r, err := execute(c)
		if err != nil {
			return EvaluationReport{}, err
		}
		results = append(results, r)
	}
	return BuildReport(cases, results), nil
}

func LoadGoldenDataset(path string) ([]GoldenCase, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var out []GoldenCase
	if err := json.Unmarshal(b, &out); err != nil {
		return nil, err
	}
	for _, c := range out {
		if c.ID == "" || c.Query == "" || c.TenantID == "" {
			return nil, fmt.Errorf("invalid golden case")
		}
	}
	return out, nil
}
func ScoreCase(c GoldenCase, r ReplayResult) CaseScore {
	s := CaseScore{CaseID: c.ID, Category: c.Category, Passed: true, KeywordScore: 1, ToolScore: 1, CitationScore: 1}
	if c.MustRefuse {
		if !r.Refused {
			s.Passed = false
			s.Errors = append(s.Errors, "expected refusal")
		}
		return s
	}
	ans := strings.ToLower(r.Answer)
	for _, k := range c.ExpectedKeywords {
		if !strings.Contains(ans, strings.ToLower(k)) {
			s.KeywordScore = 0
			s.Errors = append(s.Errors, "missing keyword: "+k)
		}
	}
	for _, want := range c.ExpectedTools {
		found := false
		for _, got := range r.Tools {
			if got == want {
				found = true
			}
		}
		if !found {
			s.ToolScore = 0
			s.Errors = append(s.Errors, "missing tool: "+want)
		}
	}
	if r.CitationCount < c.RequiredCitations {
		s.CitationScore = 0
		s.Errors = append(s.Errors, "insufficient citations")
	}
	s.Passed = len(s.Errors) == 0
	return s
}
func BuildReport(cases []GoldenCase, results []ReplayResult) EvaluationReport {
	by := map[string]ReplayResult{}
	for _, r := range results {
		by[r.CaseID] = r
	}
	rep := EvaluationReport{Total: len(cases)}
	catTotals := map[string]*CategoryScore{}
	var toolOK, toolTotal, refuseOK, refuseTotal int
	var citationSum, keywordSum float64
	for _, c := range cases {
		s := ScoreCase(c, by[c.ID])
		rep.Cases = append(rep.Cases, s)
		if s.Passed {
			rep.Passed++
		}
		citationSum += s.CitationScore
		keywordSum += s.KeywordScore
		if len(c.ExpectedTools) > 0 {
			toolTotal++
			if s.ToolScore >= 1 {
				toolOK++
			}
		}
		if c.MustRefuse {
			refuseTotal++
			if by[c.ID].Refused {
				refuseOK++
			}
		}
		cat := c.Category
		if cat == "" {
			cat = "uncategorized"
		}
		if catTotals[cat] == nil {
			catTotals[cat] = &CategoryScore{Category: cat}
		}
		catTotals[cat].Total++
		if s.Passed {
			catTotals[cat].Passed++
		}
	}
	if rep.Total > 0 {
		rep.PassRate = float64(rep.Passed) / float64(rep.Total)
		rep.Metrics.AvgCitationScore = citationSum / float64(rep.Total)
		rep.Metrics.AvgKeywordScore = keywordSum / float64(rep.Total)
	}
	if toolTotal > 0 {
		rep.Metrics.ToolSuccessRate = float64(toolOK) / float64(toolTotal)
	}
	if refuseTotal > 0 {
		rep.Metrics.RefusalAccuracy = float64(refuseOK) / float64(refuseTotal)
	}
	for _, cat := range sortedCategoryKeys(catTotals) {
		row := *catTotals[cat]
		if row.Total > 0 {
			row.PassRate = float64(row.Passed) / float64(row.Total)
		}
		rep.Categories = append(rep.Categories, row)
	}
	return rep
}

func sortedCategoryKeys(m map[string]*CategoryScore) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
