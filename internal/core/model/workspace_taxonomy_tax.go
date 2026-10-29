package model

import "strings"

// WorkspacePendingTaxonomy is used while Blueprint is pending: domain from workspace, no industry YAML seed.
func WorkspacePendingTaxonomy(domain string, st *WorkspaceTaxonomyState) *DomainTaxonomy {
	domain = strings.TrimSpace(domain)
	if domain == "" {
		domain = "通用知识"
	}
	out := &DomainTaxonomy{
		Domain:         domain,
		Aliases:        []string{domain},
		DetectKeywords: []string{domain},
		Scenes:         nil,
		Styles:         GlobalStyleScaffold(),
	}
	if st != nil {
		if d := strings.TrimSpace(st.Domain); d != "" {
			out.Domain = d
			out.Aliases = []string{d}
			out.DetectKeywords = detectKeywordsFromBlueprint(st)
		}
		if len(st.Scenes) > 0 {
			out.Scenes = cloneLabelDefs(st.Scenes)
		}
		if len(st.Styles) > 0 {
			out.Styles = mergeLabelDefinitions(GlobalStyleScaffold(), st.Styles)
		}
	}
	return out
}
