package agent

import (
	"encoding/json"
	"strings"
)

// EvidenceHit is one retrieved chunk exposed to the Agent answer gate.
type EvidenceHit struct {
	ChunkID     string  `json:"id"`
	DocID       string  `json:"doc_id,omitempty"`
	Title       string  `json:"title,omitempty"`
	MediaURL    string  `json:"media_url,omitempty"`
	Content     string  `json:"content"`
	Score       float64 `json:"score"`
	RerankScore float64 `json:"rerank_score,omitempty"`
	RecallLane  string  `json:"recall_lane,omitempty"`
	Source      string  `json:"source,omitempty"`
}

// EvidenceBundle aggregates retrieval output from tool observations.
type EvidenceBundle struct {
	Query          string        `json:"query,omitempty"`
	Hits           []EvidenceHit `json:"hits"`
	RetrievalGrade string        `json:"retrieval_grade,omitempty"`
	CoverageScore  float64       `json:"coverage_score,omitempty"`
	RetrievalScore float64       `json:"retrieval_score,omitempty"`
	Strategy       string        `json:"strategy,omitempty"`
	Total          int           `json:"total,omitempty"`
}

type knowledgeSearchPayload struct {
	Query          string        `json:"query"`
	Total          int           `json:"total"`
	Strategy       string        `json:"strategy,omitempty"`
	RetrievalGrade string        `json:"retrieval_grade,omitempty"`
	CoverageScore  float64       `json:"coverage_score,omitempty"`
	RetrievalScore float64       `json:"retrieval_score,omitempty"`
	Hits           []EvidenceHit `json:"hits"`
}

// CollectEvidence parses knowledge.* observations into a unified bundle.
func CollectEvidence(obs []Observation) EvidenceBundle {
	var out EvidenceBundle
	for _, o := range obs {
		if o.Err != nil || len(o.Output) == 0 || !strings.HasPrefix(o.Name, "knowledge.") {
			continue
		}
		var payload knowledgeSearchPayload
		if json.Unmarshal(o.Output, &payload) != nil {
			out.Hits = append(out.Hits, EvidenceHit{Content: string(o.Output)})
			continue
		}
		if out.Query == "" {
			out.Query = payload.Query
		}
		if out.Strategy == "" {
			out.Strategy = payload.Strategy
		}
		if out.RetrievalGrade == "" {
			out.RetrievalGrade = payload.RetrievalGrade
		}
		if out.CoverageScore == 0 {
			out.CoverageScore = payload.CoverageScore
		}
		if out.RetrievalScore == 0 {
			out.RetrievalScore = payload.RetrievalScore
		}
		if out.Total == 0 {
			out.Total = payload.Total
		}
		out.Hits = append(out.Hits, payload.Hits...)
	}
	return out
}

func (b EvidenceBundle) texts() []string {
	texts := make([]string, 0, len(b.Hits))
	for _, h := range b.Hits {
		if t := strings.TrimSpace(h.Content); t != "" {
			texts = append(texts, t)
		}
	}
	return texts
}

func normalizeRecallLane(source string) string {
	switch strings.ToLower(strings.TrimSpace(source)) {
	case "fulltext":
		return "keyword"
	case "vector":
		return "vector"
	case "structured":
		return "structured"
	case "hybrid", "":
		return "hybrid"
	default:
		return source
	}
}
