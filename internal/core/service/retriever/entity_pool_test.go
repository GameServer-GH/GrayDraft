package retriever

import "testing"

func TestAugmentEntitiesFromGlobalPool(t *testing.T) {
	pool := []string{"莫特斐", "漂泊者"}
	got := augmentEntitiesFromGlobalPool("能不能介绍一下 InternVL", []string{"莫特斐"}, pool)
	if len(got) != 1 {
		t.Fatalf("want 1 entity, got %v", got)
	}
	got2 := augmentEntitiesFromGlobalPool("莫特斐和漂泊者", nil, pool)
	if len(got2) != 2 {
		t.Fatalf("want 2 entities, got %v", got2)
	}
}

func TestAugmentEntitiesFromGlobalPool_dropsConversationalSpan(t *testing.T) {
	pool := []string{"请告诉我这个模型怎么调用，好吗", "莫特斐"}
	got := augmentEntitiesFromGlobalPool("莫特斐怎么组合", nil, pool)
	if len(got) != 1 || got[0] != "莫特斐" {
		t.Fatalf("got %v", got)
	}
}
