package model

import "strings"

// GlobalStyleScaffold returns cross-industry style labels (from catalog default domain).
func GlobalStyleScaffold() []LabelDefinition {
	t := DefaultCatalog().Get(DefaultCatalog().DefaultDomainKey())
	if t == nil {
		return nil
	}
	return cloneLabelDefs(t.Styles)
}

// BuildFromBlueprint constructs authoritative taxonomy from a ready workspace blueprint.
func BuildFromBlueprint(st *WorkspaceTaxonomyState) *DomainTaxonomy {
	if st == nil || !st.BlueprintReady() {
		return nil
	}
	out := &DomainTaxonomy{
		Domain:         strings.TrimSpace(st.Domain),
		Scenes:         cloneLabelDefs(st.Scenes),
		Styles:         mergeLabelDefinitions(GlobalStyleScaffold(), st.Styles),
		Aliases:        []string{strings.TrimSpace(st.Domain)},
		DetectKeywords: detectKeywordsFromBlueprint(st),
		FromBlueprint:  true,
	}
	return out
}

func detectKeywordsFromBlueprint(st *WorkspaceTaxonomyState) []string {
	seen := map[string]bool{}
	var out []string
	add := func(s string) {
		s = strings.TrimSpace(s)
		if s == "" || seen[s] {
			return
		}
		seen[s] = true
		out = append(out, s)
	}
	add(st.Domain)
	for _, defs := range [][]LabelDefinition{st.Scenes, st.Styles} {
		for _, d := range defs {
			add(d.Label)
			for _, kw := range d.Keywords {
				add(kw)
			}
		}
	}
	return out
}

// mergeLabelDefinitions merges extra labels into base (by label name, union keywords).
func mergeLabelDefinitions(base, extra []LabelDefinition) []LabelDefinition {
	out := cloneLabelDefs(base)
	tmp := &DomainTaxonomy{Scenes: out}
	mergeLabelDefinitionsInto(&tmp.Scenes, extra)
	return tmp.Scenes
}

// BuildEffectiveTaxonomy merges scaffold + YAML seed + workspace state (blueprint or keyword overlay).
func BuildEffectiveTaxonomy(seed *DomainTaxonomy, st *WorkspaceTaxonomyState, scaffoldStyles []LabelDefinition) *DomainTaxonomy {
	if st != nil && st.BlueprintReady() {
		if tax := BuildFromBlueprint(st); tax != nil {
			return tax
		}
	}
	if seed == nil {
		seed = DefaultCatalog().Default()
	}
	out := &DomainTaxonomy{
		Domain:         seed.Domain,
		Aliases:        append([]string(nil), seed.Aliases...),
		DetectKeywords: append([]string(nil), seed.DetectKeywords...),
		Scenes:         cloneLabelDefs(seed.Scenes),
		Styles:         mergeLabelDefinitions(scaffoldStyles, seed.Styles),
	}
	if st == nil {
		return out
	}
	if d := strings.TrimSpace(st.Domain); d != "" {
		out.Domain = d
	}
	mergeLabelDefinitionsInto(&out.Scenes, st.Scenes)
	mergeLabelDefinitionsInto(&out.Styles, st.Styles)
	return out
}

// mergeLabelDefinitionsInto appends or merges label defs (full labels allowed, not only keywords).
func mergeLabelDefinitionsInto(target *[]LabelDefinition, extra []LabelDefinition) {
	if target == nil || len(extra) == 0 {
		return
	}
	byLabel := map[string]int{}
	for i, d := range *target {
		byLabel[d.Label] = i
	}
	for _, e := range extra {
		if e.Label == "" {
			continue
		}
		idx, ok := byLabel[e.Label]
		if !ok {
			*target = append(*target, LabelDefinition{
				Label:    e.Label,
				Keywords: uniqueStrings(e.Keywords),
			})
			byLabel[e.Label] = len(*target) - 1
			continue
		}
		(*target)[idx].Keywords = uniqueStrings(append((*target)[idx].Keywords, e.Keywords...))
	}
}
