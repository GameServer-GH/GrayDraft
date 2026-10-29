package model

import "testing"

func TestWorkspacePendingTaxonomy_noYAMLSeed(t *testing.T) {
	tax := WorkspacePendingTaxonomy("学科资料", nil)
	if tax == nil || tax.Domain != "学科资料" {
		t.Fatalf("domain=%q", tax.Domain)
	}
	if len(tax.Scenes) != 0 {
		t.Fatalf("expected no YAML industry scenes, got %d", len(tax.Scenes))
	}
}

func TestWorkspacePendingTaxonomy_overlayState(t *testing.T) {
	st := &WorkspaceTaxonomyState{
		Domain: "影视解说",
		Scenes: []LabelDefinition{{Label: "剧情解析"}},
	}
	tax := WorkspacePendingTaxonomy("ignored", st)
	if tax.Domain != "影视解说" {
		t.Fatalf("domain=%q", tax.Domain)
	}
	if len(tax.Scenes) != 1 || tax.Scenes[0].Label != "剧情解析" {
		t.Fatalf("scenes=%v", tax.Scenes)
	}
}
