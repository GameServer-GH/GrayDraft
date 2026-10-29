package model

import (
	"fmt"
	"strings"
)

// LabelDefinition is one canonical 4D label plus synonym keywords for normalization.
type LabelDefinition struct {
	Label    string   `json:"label" yaml:"label"`
	Keywords []string `json:"keywords,omitempty" yaml:"keywords,omitempty"`
}

// DomainTaxonomy is the scene/style taxonomy for one business domain.
type DomainTaxonomy struct {
	Domain         string            `json:"domain" yaml:"-"`
	Aliases        []string          `json:"aliases,omitempty" yaml:"aliases,omitempty"`
	DetectKeywords []string          `json:"detect_keywords,omitempty" yaml:"detect_keywords,omitempty"`
	Scenes         []LabelDefinition `json:"scenes,omitempty" yaml:"scenes,omitempty"`
	Styles         []LabelDefinition `json:"styles,omitempty" yaml:"styles,omitempty"`
	// FromBlueprint marks taxonomy built from workspace_4d_taxonomy (prefer embedding align over keyword Contains).
	FromBlueprint bool `json:"-" yaml:"-"`
}

// GenericNormalize maps free text to a canonical label using keyword definitions.
func GenericNormalize(input string, definitions []LabelDefinition) string {
	input = strings.TrimSpace(input)
	if input == "" || len(definitions) == 0 {
		return input
	}
	for _, def := range definitions {
		if input == def.Label {
			return def.Label
		}
	}
	lowerInput := strings.ToLower(input)
	for _, def := range definitions {
		for _, kw := range def.Keywords {
			kw = strings.TrimSpace(kw)
			if kw == "" {
				continue
			}
			if strings.Contains(lowerInput, strings.ToLower(kw)) {
				return def.Label
			}
		}
	}
	return input
}

// NormalizeScene normalizes a scene label within this domain taxonomy.
func (t *DomainTaxonomy) NormalizeScene(s string) string {
	if t == nil {
		return strings.TrimSpace(s)
	}
	return GenericNormalize(s, t.Scenes)
}

// NormalizeStyle normalizes a style label within this domain taxonomy.
func (t *DomainTaxonomy) NormalizeStyle(s string) string {
	if t == nil {
		return strings.TrimSpace(s)
	}
	return GenericNormalize(s, t.Styles)
}

// NormalizeQuerySemantic4D applies domain-scoped taxonomy normalization.
func NormalizeQuerySemantic4D(q QuerySemantic4D, tax *DomainTaxonomy) QuerySemantic4D {
	q.Domain = strings.TrimSpace(q.Domain)
	if tax == nil {
		tax = DefaultCatalog().Default()
	}
	if q.Domain == "" && tax != nil {
		q.Domain = tax.Domain
	}
	if tax != nil {
		if key := strings.TrimSpace(q.Domain); key != "" {
			if resolved := DefaultCatalog().Get(key); resolved != nil {
				tax = resolved
			}
		}
		q.Scene = tax.NormalizeScene(q.Scene)
		q.Style = tax.NormalizeStyle(q.Style)
	}
	return q
}

// GeneratePromptBlock builds the LLM prompt dictionary for this domain.
func (t *DomainTaxonomy) GeneratePromptBlock() string {
	if t == nil {
		return ""
	}
	var sceneLabels, styleLabels, mappings []string
	for _, s := range t.Scenes {
		if s.Label == "" {
			continue
		}
		sceneLabels = append(sceneLabels, s.Label)
		if len(s.Keywords) > 0 {
			mappings = append(mappings, strings.Join(s.Keywords, "、")+" → scene="+s.Label)
		}
	}
	for _, s := range t.Styles {
		if s.Label == "" {
			continue
		}
		styleLabels = append(styleLabels, s.Label)
		if len(s.Keywords) > 0 {
			mappings = append(mappings, strings.Join(s.Keywords, "、")+" → style="+s.Label)
		}
	}
	block := "【4D 标签字典 — 本工作区 domain=" + t.Domain + "；scene/style 必须从下列择一，禁止自造词】\n"
	if len(sceneLabels) > 0 {
		block += "scene 可选：" + strings.Join(sceneLabels, " | ") + "\n"
	}
	if len(styleLabels) > 0 {
		block += "style 可选：" + strings.Join(styleLabels, " | ") + "\n"
	}
	if len(mappings) > 0 {
		block += "映射建议：" + strings.Join(mappings, "；") + "\n"
	}
	return block
}

// ApplyQueryHeuristics fills domain/scene/style from query text using this taxonomy only.
func (t *DomainTaxonomy) ApplyQueryHeuristics(q string, out *QuerySemantic4D) {
	if t == nil || out == nil || strings.TrimSpace(q) == "" {
		return
	}
	lower := strings.ToLower(q)
	for _, kw := range t.DetectKeywords {
		kw = strings.TrimSpace(kw)
		if kw != "" && strings.Contains(lower, strings.ToLower(kw)) {
			out.Domain = t.Domain
			break
		}
	}
	for _, alias := range t.Aliases {
		if alias != "" && strings.Contains(lower, strings.ToLower(alias)) {
			out.Domain = t.Domain
			break
		}
	}
	if out.Scene == "" {
		out.Scene = firstLabelMatchInQuery(q, t.Scenes)
	}
	if out.Style == "" {
		out.Style = firstLabelMatchInQuery(q, t.Styles)
	}
}

func firstLabelMatchInQuery(q string, defs []LabelDefinition) string {
	lower := strings.ToLower(q)
	for _, def := range defs {
		label := strings.TrimSpace(def.Label)
		if label != "" && strings.Contains(lower, strings.ToLower(label)) {
			return label
		}
		for _, kw := range def.Keywords {
			kw = strings.TrimSpace(kw)
			if kw != "" && strings.Contains(lower, strings.ToLower(kw)) {
				return def.Label
			}
		}
	}
	return ""
}

// SceneLabels returns canonical scene labels for this domain.
func (t *DomainTaxonomy) SceneLabels() []string {
	if t == nil {
		return nil
	}
	out := make([]string, 0, len(t.Scenes))
	for _, s := range t.Scenes {
		if s.Label != "" {
			out = append(out, s.Label)
		}
	}
	return out
}

// StyleLabels returns canonical style labels for this domain.
func (t *DomainTaxonomy) StyleLabels() []string {
	if t == nil {
		return nil
	}
	out := make([]string, 0, len(t.Styles))
	for _, s := range t.Styles {
		if s.Label != "" {
			out = append(out, s.Label)
		}
	}
	return out
}

// String for logging.
func (t *DomainTaxonomy) String() string {
	if t == nil {
		return ""
	}
	return fmt.Sprintf("taxonomy{%s}", t.Domain)
}
