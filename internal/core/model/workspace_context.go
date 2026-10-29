package model

// ContextProfile is structured workspace background stored on kb_types.context_profile.
type ContextProfile struct {
	KBType          string   `json:"kb_type,omitempty"`
	Description     string   `json:"description"`
	Entities        []string `json:"entities,omitempty"`
	Rules           []string `json:"rules,omitempty"`
	NegativeContext []string `json:"negative_context,omitempty"`
}

// IsEmpty reports whether the profile carries any usable background.
func (p ContextProfile) IsEmpty() bool {
	return p.Description == "" && len(p.Entities) == 0 && len(p.Rules) == 0 && len(p.NegativeContext) == 0
}
