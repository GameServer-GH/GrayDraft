package postgres

import "testing"

func TestJoinChunkParts(t *testing.T) {
	if joinChunkParts(nil) != "" {
		t.Fatal("empty")
	}
	if joinChunkParts([]string{"a"}) != "a" {
		t.Fatal("single")
	}
	got := joinChunkParts([]string{"a", "b"})
	if got != "a\n\nb" {
		t.Fatalf("got %q", got)
	}
}
