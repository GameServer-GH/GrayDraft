package taxonomyevolve

import (
	"strings"

	"github.com/graydraft/core/model"
)

// workspaceDomainHint picks domain label from DB overlay / profile before YAML catalog matching.
func workspaceDomainHint(profile *model.ContextProfile, workspaceText string, st *model.WorkspaceTaxonomyState) string {
	if st != nil {
		if d := strings.TrimSpace(st.Domain); d != "" {
			return d
		}
	}
	if profile != nil {
		if d := strings.TrimSpace(profile.Description); d != "" {
			if line := firstLine(d); line != "" {
				return trimDomainLabel(line)
			}
		}
	}
	if t := strings.TrimSpace(workspaceText); t != "" {
		if line := firstLine(t); line != "" {
			return trimDomainLabel(line)
		}
	}
	return ""
}

func firstLine(s string) string {
	s = strings.TrimSpace(s)
	if i := strings.IndexAny(s, "\n\r"); i >= 0 {
		s = s[:i]
	}
	return strings.TrimSpace(s)
}

func trimDomainLabel(s string) string {
	s = strings.TrimSpace(s)
	if len([]rune(s)) > 48 {
		s = string([]rune(s)[:48])
	}
	return s
}
