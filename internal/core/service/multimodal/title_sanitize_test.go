package multimodal

import "testing"

func TestSanitizeDocumentTitle_rejectsJSONFence(t *testing.T) {
	for _, bad := range []string{"json", "```json", "{", "foo.jpg"} {
		if SanitizeDocumentTitle(bad) != "" {
			t.Fatalf("expected empty for %q", bad)
		}
	}
	if SanitizeDocumentTitle("S2模型发布指南") == "" {
		t.Fatal("expected valid title")
	}
}
