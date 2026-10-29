package nexus

import "strings"

// ContextProfile is structured workspace background synced to GrayDraft kb_types.context_profile.
type ContextProfile struct {
	KBType          string   `json:"kb_type,omitempty"`
	Description     string   `json:"description"`
	Entities        []string `json:"entities,omitempty"`
	Rules           []string `json:"rules,omitempty"`
	NegativeContext []string `json:"negative_context,omitempty"`
}

// WorkspaceContextText returns flattened text for retrieval embedding prefix.
func (p ContextProfile) WorkspaceContextText() string {
	if p.Description == "" && len(p.Entities) == 0 && len(p.Rules) == 0 {
		return ""
	}
	var b strings.Builder
	if d := strings.TrimSpace(p.Description); d != "" {
		b.WriteString(d)
		b.WriteByte('\n')
	}
	if len(p.Entities) > 0 {
		b.WriteString("核心实体：")
		for i, e := range p.Entities {
			if i > 0 {
				b.WriteString("、")
			}
			b.WriteString(strings.TrimSpace(e))
		}
		b.WriteByte('\n')
	}
	for _, r := range p.Rules {
		r = strings.TrimSpace(r)
		if r == "" {
			continue
		}
		b.WriteString("规则：")
		b.WriteString(r)
		b.WriteByte('\n')
	}
	return strings.TrimSpace(b.String())
}
