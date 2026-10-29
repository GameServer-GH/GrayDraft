package postgres

import (
	"encoding/json"
	"time"

	"github.com/graydraft/core/model"
)

type workspaceTaxonomyStats struct {
	KeywordHits              map[string]int `json:"keyword_hits,omitempty"`
	BlueprintStatus          string         `json:"blueprint_status,omitempty"`
	BlueprintError           string         `json:"blueprint_error,omitempty"`
	BlueprintAt              string         `json:"blueprint_at,omitempty"` // RFC3339
	BlueprintBackfillVersion int            `json:"blueprint_backfill_version,omitempty"`
}

func marshalWorkspaceStats(st *model.WorkspaceTaxonomyState) []byte {
	s := workspaceTaxonomyStats{
		KeywordHits:              st.KeywordHits,
		BlueprintStatus:          st.BlueprintStatus,
		BlueprintError:           st.BlueprintError,
		BlueprintBackfillVersion: st.BlueprintBackfillVersion,
	}
	if !st.BlueprintAt.IsZero() {
		s.BlueprintAt = st.BlueprintAt.UTC().Format("2006-01-02T15:04:05Z")
	}
	if st.KeywordHits == nil && s.BlueprintStatus == "" {
		raw, _ := json.Marshal(map[string]any{})
		return raw
	}
	raw, _ := json.Marshal(s)
	return raw
}

func unmarshalWorkspaceStats(raw []byte, st *model.WorkspaceTaxonomyState) {
	if len(raw) == 0 || string(raw) == "{}" {
		return
	}
	var s workspaceTaxonomyStats
	if err := json.Unmarshal(raw, &s); err == nil && (s.BlueprintStatus != "" || s.KeywordHits != nil) {
		st.KeywordHits = s.KeywordHits
		st.BlueprintStatus = s.BlueprintStatus
		st.BlueprintError = s.BlueprintError
		st.BlueprintBackfillVersion = s.BlueprintBackfillVersion
		if s.BlueprintAt != "" {
			if t, err := time.Parse(time.RFC3339, s.BlueprintAt); err == nil {
				st.BlueprintAt = t
			}
		}
		return
	}
	// Legacy: stats column was only keyword_hits map.
	var legacy map[string]int
	if json.Unmarshal(raw, &legacy) == nil {
		st.KeywordHits = legacy
	}
}
