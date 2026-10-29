package model

import "testing"

func TestSanitizeEntityToken_trimsQuotesOnly(t *testing.T) {
	if got := SanitizeEntityToken("《莫特斐》"); got != "莫特斐" {
		t.Fatalf("got %q", got)
	}
}

func TestFinalSanitize_dedupeLong(t *testing.T) {
	q := "介绍一下莫特斐"
	got := FinalSanitize([]string{"介绍一下莫特斐", "莫特斐"}, q)
	if len(got) != 1 || got[0] != "莫特斐" {
		t.Fatalf("got %v", got)
	}
}

func TestSlimEntities_shortest(t *testing.T) {
	got := SlimEntities([]string{"莫特斐", "漂泊者"})
	if len(got) != 1 || got[0] != "莫特斐" {
		t.Fatalf("got %v", got)
	}
}

func TestFinalSanitize_groundedInQuery(t *testing.T) {
	got := FinalSanitize([]string{"无关专名"}, "介绍一下莫特斐")
	if len(got) != 0 {
		t.Fatalf("hallucinated entity should drop: %v", got)
	}
}

func TestAtomicLongShortDedup(t *testing.T) {
	in := []string{"莫特斐", "介绍一下莫特斐"}
	got := atomicLongShortDedup(in)
	if len(got) != 1 || got[0] != "莫特斐" {
		t.Fatalf("got %v", got)
	}
}
