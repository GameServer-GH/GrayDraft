package model

import (
	"strings"
	"testing"
)

func TestGenericNormalizeScene_universal(t *testing.T) {
	tax := DefaultCatalog().Get(UniversalDomainKey)
	if tax == nil {
		t.Fatal("missing universal taxonomy")
	}
	if got := tax.NormalizeScene("介绍"); got != "基础介绍" {
		t.Fatalf("got %q want 基础介绍", got)
	}
}

func TestGeneratePromptBlock_containsUniversal(t *testing.T) {
	tax := DefaultCatalog().Get(UniversalDomainKey)
	block := tax.GeneratePromptBlock()
	if block == "" || !strings.Contains(block, "universal") && !strings.Contains(block, "通用") {
		t.Fatalf("prompt block missing labels: %q", block)
	}
	if !strings.Contains(block, "基础介绍") {
		t.Fatalf("expected generic scene in block: %q", block)
	}
}

func TestCatalog_InferDomainFromQuery_noIndustryBias(t *testing.T) {
	c := DefaultCatalog()
	if got := c.InferDomainFromQuery("这个公式怎么写"); got != "" {
		t.Fatalf("unexpected industry infer %q", got)
	}
	if got := c.ResolveKey(nil, "", ""); got != UniversalDomainKey {
		t.Fatalf("resolve got %q", got)
	}
}

func TestCatalog_ResolveKey_profileUsesUniversal(t *testing.T) {
	c := DefaultCatalog()
	prof := &ContextProfile{
		Description: "学科资料领域涵盖目的地选择、行程规划、景点推荐",
	}
	if got := c.ResolveKey(prof, "", ""); got != UniversalDomainKey {
		t.Fatalf("profile resolve got %q want universal scaffold", got)
	}
}

func TestNormalizeQuerySemantic4D_AtCore(t *testing.T) {
	tax := DefaultCatalog().Get(UniversalDomainKey)
	q := NormalizeQuerySemantic4D(QuerySemantic4D{
		Domain: UniversalDomainKey, Scene: "介绍", Style: "步骤说明",
		Entities: []string{"示例实体"},
	}, tax)
	core := q.AtCore()
	if core.Scene != "" || core.Style != "" || core.Domain != UniversalDomainKey {
		t.Fatalf("%+v", core)
	}
}

func TestWaterfallTierFromSuffix(t *testing.T) {
	if WaterfallTierFromSuffix("") != 1 {
		t.Fatal("tier1")
	}
	if WaterfallTierFromSuffix("_wf_soft") != 5 {
		t.Fatal("tier5")
	}
}
