package embedder

import "testing"

func TestPseudoEmbed_deterministic(t *testing.T) {
	a := PseudoEmbed("hello", 128)
	b := PseudoEmbed("hello", 128)
	if len(a) != 128 {
		t.Fatalf("expected dim 128")
	}
	for i := range a {
		if a[i] != b[i] {
			t.Fatalf("non-deterministic at %d", i)
		}
	}
}
