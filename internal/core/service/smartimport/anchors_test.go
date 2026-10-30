package smartimport

import "testing"

func TestDocGroupIDStable(t *testing.T) {
	a := DocGroupID("InternVL", "", 1, 3)
	b := DocGroupID("InternVL", "other", 2, 3)
	if a == "" || a != b {
		t.Fatalf("group=%q %q", a, b)
	}
	if DocGroupID("", "", 1, 1) != "" {
		t.Fatal("single untitled segment should not invent a group")
	}
}
