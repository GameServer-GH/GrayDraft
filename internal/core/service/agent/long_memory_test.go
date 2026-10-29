package agent

import "testing"

func TestValidateMemoryWrite(t *testing.T) {
	item := MemoryItem{TenantID: "t", ActorID: "a", Kind: "preference", Subject: "platform", Confidence: .8}
	if ValidateMemoryWrite(item, false) != nil {
		t.Fatal("workspace memory should write without a per-item consent prompt")
	}
	if ValidateMemoryWrite(item, true) != nil {
		t.Fatal("valid memory rejected")
	}
	item.Sensitivity = "sensitive"
	if ValidateMemoryWrite(item, true) == nil {
		t.Fatal("sensitive memory accepted")
	}
}

func TestFuseMemoryResultsDeduplicatesAndRanksSharedItems(t *testing.T) {
	shared := MemoryItem{MemoryID: "shared", Kind: "preference", Subject: "platform", Confidence: .9}
	got := FuseMemoryResults([]MemoryItem{shared, {MemoryID: "relation", Kind: "stable_fact", Subject: "region", Confidence: .8}}, []MemoryItem{{MemoryID: "semantic", Kind: "feedback", Subject: "tone", Confidence: .8}, shared}, 3)
	if len(got) != 3 || got[0].MemoryID != "shared" {
		t.Fatalf("unexpected fusion: %#v", got)
	}
}
