package model

import "testing"

func TestMergeTaxonomyOverlay_appendsKeywords(t *testing.T) {
	base := DefaultCatalog().Get(UniversalDomainKey)
	overlay := &WorkspaceTaxonomyState{
		Domain:  UniversalDomainKey,
		Version: 1,
		Scenes:  []LabelDefinition{{Label: "模型使用", Keywords: []string{"共鸣回路"}}},
	}
	merged := MergeTaxonomyOverlay(base, overlay)
	if !merged.HasKeyword("共鸣回路") {
		t.Fatal("overlay keyword missing")
	}
}
