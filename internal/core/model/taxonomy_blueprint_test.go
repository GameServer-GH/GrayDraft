package model

import "testing"

func TestBuildFromBlueprint(t *testing.T) {
	st := &WorkspaceTaxonomyState{
		Domain:          "影视解说",
		BlueprintStatus: BlueprintStatusReady,
		Scenes:          []LabelDefinition{{Label: "剧情复盘"}},
		Styles:          []LabelDefinition{{Label: "影评分析"}},
	}
	tax := BuildFromBlueprint(st)
	if tax == nil || tax.Domain != "影视解说" {
		t.Fatalf("got %+v", tax)
	}
	if len(tax.Scenes) != 1 || tax.Scenes[0].Label != "剧情复盘" {
		t.Fatal("scenes")
	}
}

func TestBuildEffectiveTaxonomy_blueprintWins(t *testing.T) {
	seed := DefaultCatalog().Get(UniversalDomainKey)
	st := &WorkspaceTaxonomyState{
		Domain:          "影视解说",
		BlueprintStatus: BlueprintStatusReady,
		Scenes:          []LabelDefinition{{Label: "剧情复盘"}},
	}
	tax := BuildEffectiveTaxonomy(seed, st, GlobalStyleScaffold())
	if tax.Domain != "影视解说" {
		t.Fatalf("domain %q", tax.Domain)
	}
	for _, s := range tax.Scenes {
		if s.Label == "参数建议" {
			t.Fatal("unrelated scene should not appear when blueprint ready")
		}
	}
}
