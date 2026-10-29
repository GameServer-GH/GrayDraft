package multimodal

import "testing"

func TestHammingDistance_identical(t *testing.T) {
	h := "a1b2c3d4e5f60718"
	if d := hammingDistance(h, h); d != 0 {
		t.Fatalf("expected 0, got %d", d)
	}
}

func TestComputePHash_emptyForText(t *testing.T) {
	if got := computePHash([]byte("hello"), "text/plain"); got != "" {
		t.Fatalf("expected empty phash for text, got %q", got)
	}
}
