package model

import "testing"

func TestLooksLikeConversationalFragment(t *testing.T) {
	if !LooksLikeConversationalFragment("我现在要去吃午饭") {
		t.Fatal("expected utterance")
	}
	if LooksLikeConversationalFragment("千岛湖") {
		t.Fatal("place name should pass")
	}
}
