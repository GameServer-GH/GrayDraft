package classifier

import "testing"

func TestAliasIndex_CanonicalEntity_shortestWins(t *testing.T) {
	idx := BuildAliasIndex([]DocAliasSource{
		{Title: "莫特斐选型说明", Extensions: map[string]any{"aliases": []any{"莫特斐", "Mortefi"}}},
	})
	got := idx.CanonicalEntity("Mortefi")
	if got != "莫特斐" && got != "Mortefi" {
		t.Fatalf("canonical=%q", got)
	}
	if len([]rune(idx.CanonicalEntity("莫特斐"))) > len([]rune(got)) {
		t.Fatalf("expected shortest in group, got %q", got)
	}
}

func TestAliasIndex_CanonicalizeEntities_dedup(t *testing.T) {
	idx := BuildAliasIndex([]DocAliasSource{{Title: "InternVL", Extensions: map[string]any{"aliases": []any{"书生多模态", "InternVL"}}}})
	out := idx.CanonicalizeEntities([]string{"书生多模态", "InternVL"})
	if len(out) != 1 {
		t.Fatalf("want 1 entity, got %v", out)
	}
}
