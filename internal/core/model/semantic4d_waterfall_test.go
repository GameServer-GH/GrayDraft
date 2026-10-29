package model

import "testing"

func TestWaterfallTiers_entityMinDiffersFromCore(t *testing.T) {
	q := QuerySemantic4D{
		Domain:   "教程资料",
		Scene:    "基础介绍",
		Style:    "概念解释",
		Entities: []string{"莫特斐", "介绍一下 InternVL"},
	}
	core := q.AtCore()
	if core == nil || core.Scene != "" || core.Style != "" {
		t.Fatalf("core=%+v", core)
	}
	if len(core.Entities) < 2 {
		t.Fatalf("core should keep all sanitized entities: %v", core.Entities)
	}
	em := q.AtEntityMin()
	if em == nil || len(em.Entities) != 1 || em.Entities[0] != "莫特斐" {
		t.Fatalf("entity min=%+v", em)
	}
}
