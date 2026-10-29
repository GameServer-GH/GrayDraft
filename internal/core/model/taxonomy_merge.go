package model

import "strings"

// MergeTaxonomyOverlay clones base taxonomy and merges workspace overlay (deprecated: use BuildEffectiveTaxonomy).
func MergeTaxonomyOverlay(base *DomainTaxonomy, overlay *WorkspaceTaxonomyState) *DomainTaxonomy {
	return BuildEffectiveTaxonomy(base, overlay, GlobalStyleScaffold())
}

func cloneLabelDefs(in []LabelDefinition) []LabelDefinition {
	out := make([]LabelDefinition, len(in))
	for i, d := range in {
		out[i] = LabelDefinition{
			Label:    d.Label,
			Keywords: append([]string(nil), d.Keywords...),
		}
	}
	return out
}

func mergeLabelKeywords(target *[]LabelDefinition, extra []LabelDefinition) {
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
		if len(e.Keywords) == 0 {
			idx, ok := byLabel[e.Label]
			if !ok {
				*target = append(*target, LabelDefinition{Label: e.Label})
				byLabel[e.Label] = len(*target) - 1
			} else if (*target)[idx].Label == "" {
				(*target)[idx].Label = e.Label
			}
			continue
		}
		idx, ok := byLabel[e.Label]
		if !ok {
			*target = append(*target, LabelDefinition{Label: e.Label, Keywords: uniqueStrings(e.Keywords)})
			byLabel[e.Label] = len(*target) - 1
			continue
		}
		(*target)[idx].Keywords = uniqueStrings(append((*target)[idx].Keywords, e.Keywords...))
	}
}

func uniqueStrings(in []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, s := range in {
		s = strings.TrimSpace(s)
		if s == "" || seen[s] {
			continue
		}
		seen[s] = true
		out = append(out, s)
	}
	return out
}

// HasKeyword reports whether phrase already exists in taxonomy labels (exact or substring).
func (t *DomainTaxonomy) HasKeyword(phrase string) bool {
	if t == nil {
		return false
	}
	phrase = strings.TrimSpace(phrase)
	if phrase == "" {
		return false
	}
	lower := strings.ToLower(phrase)
	for _, defs := range [][]LabelDefinition{t.Scenes, t.Styles} {
		for _, d := range defs {
			if strings.ToLower(d.Label) == lower {
				return true
			}
			for _, kw := range d.Keywords {
				kw = strings.TrimSpace(kw)
				if kw == "" {
					continue
				}
				kl := strings.ToLower(kw)
				if kl == lower || strings.Contains(lower, kl) || strings.Contains(kl, lower) {
					return true
				}
			}
		}
	}
	return false
}
