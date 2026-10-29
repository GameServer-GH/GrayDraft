package model

import (
	"strings"
	"time"
)

// Blueprint status values for workspace-driven taxonomy.
const (
	BlueprintStatusPending = "pending"
	BlueprintStatusReady   = "ready"
	BlueprintStatusFailed  = "failed"
)

// DocSummarySnippet is a short document summary for blueprint generation.
type DocSummarySnippet struct {
	DocID   string `json:"doc_id"`
	Title   string `json:"title"`
	Summary string `json:"summary"`
}

// MappingDelta is a local blueprint from one document ingest (Loop A input).
type MappingDelta struct {
	Scene      string   `json:"scene,omitempty"`
	Style      string   `json:"style,omitempty"`
	KeyPhrases []string `json:"key_phrases,omitempty"`
	DocID      string   `json:"doc_id,omitempty"`
}

// KeywordCandidate is a buffered phrase awaiting promotion (cooling zone).
type KeywordCandidate struct {
	Phrase    string    `json:"phrase"`
	Dimension string    `json:"dimension"` // scene | style
	Label     string    `json:"label"`
	DocCount  int       `json:"doc_count"`
	FirstSeen time.Time `json:"first_seen,omitempty"`
	LastSeen  time.Time `json:"last_seen,omitempty"`
}

// WorkspaceTaxonomyState is persisted per kb_type partition.
type WorkspaceTaxonomyState struct {
	KBType          string             `json:"kb_type"`
	Domain          string             `json:"domain"`
	Version         int                `json:"version"`
	Scenes          []LabelDefinition  `json:"scenes,omitempty"`
	Styles          []LabelDefinition  `json:"styles,omitempty"`
	Candidates      []KeywordCandidate `json:"candidates,omitempty"`
	KeywordHits     map[string]int     `json:"keyword_hits,omitempty"`
	BlueprintStatus          string             `json:"blueprint_status,omitempty"`
	BlueprintError           string             `json:"blueprint_error,omitempty"`
	BlueprintAt              time.Time          `json:"blueprint_at,omitempty"`
	BlueprintBackfillVersion int                `json:"blueprint_backfill_version,omitempty"`
}

// BlueprintReady reports whether workspace blueprint can drive retrieval/import alone.
func (s *WorkspaceTaxonomyState) BlueprintReady() bool {
	if s == nil {
		return false
	}
	if s.BlueprintStatus != BlueprintStatusReady {
		return false
	}
	return strings.TrimSpace(s.Domain) != "" && (len(s.Scenes) > 0 || len(s.Styles) > 0)
}

// BlueprintPending reports async blueprint generation in flight.
func (s *WorkspaceTaxonomyState) BlueprintPending() bool {
	return s != nil && s.BlueprintStatus == BlueprintStatusPending
}

// OverlayKeywords returns extra keywords for a label in dimension scene|style.
func (s *WorkspaceTaxonomyState) OverlayKeywords(dimension, label string) []string {
	if s == nil {
		return nil
	}
	defs := s.Scenes
	if dimension == "style" {
		defs = s.Styles
	}
	for _, d := range defs {
		if d.Label == label {
			return append([]string(nil), d.Keywords...)
		}
	}
	return nil
}
