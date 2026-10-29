package nexus

import "testing"

func TestKbTypeFromWorkspaceKeyStable(t *testing.T) {
	a := KbTypeFromWorkspaceKey("my-secret-key", "gd_")
	b := KbTypeFromWorkspaceKey("my-secret-key", "gd_")
	if a != b {
		t.Fatalf("unstable: %s vs %s", a, b)
	}
	if len(a) != len("gd_")+32 {
		t.Fatalf("length: %d %q", len(a), a)
	}
	if KbTypeFromWorkspaceKey("other", "gd_") == a {
		t.Fatal("different keys should differ")
	}
}
