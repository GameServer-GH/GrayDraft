package smartimport

import (
	"strings"

	"github.com/graydraft/core/model"
)

// DeriveSegmentSemantic4D clones document 4D and enriches entities from segment metadata.
func DeriveSegmentSemantic4D(docExt model.Extensions, seg PlannedSegment) map[string]any {
	if docExt == nil {
		return nil
	}
	raw, ok := docExt["semantic_tags_4d"].(map[string]any)
	if !ok || len(raw) == 0 {
		return nil
	}
	out := map[string]any{
		"domain": strings.TrimSpace(strField(raw["domain"])),
		"scene":  strings.TrimSpace(strField(raw["scene"])),
		"style":  strings.TrimSpace(strField(raw["style"])),
	}
	seen := map[string]bool{}
	var ents []string
	addEnt := func(s string) {
		s = strings.TrimSpace(s)
		if s == "" || seen[s] || len([]rune(s)) < 2 {
			return
		}
		seen[s] = true
		ents = append(ents, s)
	}
	for _, e := range strSlice(raw["entities"]) {
		addEnt(e)
	}
	for _, t := range seg.Tags {
		addEnt(t)
	}
	if seg.Title != "" {
		addEnt(seg.Title)
	}
	if seg.SemanticSummary != "" {
		addEnt(seg.SemanticSummary)
	}
	if len(ents) > 8 {
		ents = ents[:8]
	}
	if len(ents) > 0 {
		out["entities"] = ents
	}
	if out["domain"] == "" && out["scene"] == "" && out["style"] == "" && len(ents) == 0 {
		return nil
	}
	return out
}

func strField(v any) string {
	s, _ := v.(string)
	return s
}

func strSlice(v any) []string {
	switch t := v.(type) {
	case []string:
		return t
	case []any:
		var out []string
		for _, x := range t {
			if s, ok := x.(string); ok {
				out = append(out, s)
			}
		}
		return out
	default:
		return nil
	}
}
