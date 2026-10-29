package model

import "testing"

func TestSanitizeQueryEntitiesLLM_motefi(t *testing.T) {
	in := []string{"介绍一下莫特斐", "莫特斐"}
	got := SanitizeQueryEntitiesLLM(in, "能不能介绍一下莫特斐啊")
	if len(got) != 1 || got[0] != "莫特斐" {
		t.Fatalf("got %v", got)
	}
}

func TestSanitizeQueryEntitiesLLM_spuriousLong(t *testing.T) {
	in := []string{"请告诉我这个模型怎么调用，好吗"}
	got := SanitizeQueryEntitiesLLM(in, "请告诉我这个模型怎么调用")
	for _, e := range got {
		if isSpuriousLongEntity(e) || LooksLikeConversationalFragment(e) {
			t.Fatalf("should drop spurious: %v", got)
		}
	}
}
