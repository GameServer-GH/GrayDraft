package classifier

import "testing"

func TestBuildAliasIndex_linksSharedTerms(t *testing.T) {
	idx := BuildAliasIndex([]DocAliasSource{
		{Title: "书生多模态接口速查", Extensions: map[string]any{"parent_title": "InternVL"}},
	})
	terms := idx.ExpandTerms("InternVL 接口介绍")
	foundInternVL := false
	foundTitle := false
	for _, t := range terms {
		if t == "InternVL" {
			foundInternVL = true
		}
		if t == "书生多模态接口速查" {
			foundTitle = true
		}
	}
	if !foundInternVL {
		t.Fatal("expected InternVL in expansion")
	}
	if !foundTitle {
		t.Fatalf("expected linked title in expansion, got %v", terms)
	}
}

func TestAliasIndex_EntityMatched(t *testing.T) {
	idx := BuildAliasIndex([]DocAliasSource{
		{Title: "书生多模态接口速查", Extensions: map[string]any{"parent_title": "InternVL"}},
	})
	if !idx.EntityMatched("InternVL 接口", "书生多模态接口速查", map[string]any{"parent_title": "InternVL"}) {
		t.Fatal("expected entity match via alias group")
	}
}

func TestExpandTerms_intentSynonyms(t *testing.T) {
	idx := BuildAliasIndex(nil)
	terms := idx.ExpandTerms("InternVL 接口介绍")
	hasSkill := false
	hasIntro := false
	for _, t := range terms {
		if t == "教程" {
			hasSkill = true
		}
		if t == "介绍" {
			hasIntro = true
		}
	}
	if !hasSkill || !hasIntro {
		t.Fatalf("expected intent synonyms, got %v", terms)
	}
}
