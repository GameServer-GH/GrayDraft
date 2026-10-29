package multimodal

import "testing"

func TestComposePromptWithContext(t *testing.T) {
	got := ComposePromptWithContext(DefaultBasePrompt, "补充", "InternLM 教程")
	if got == DefaultBasePrompt {
		t.Fatal("expected stacked prompt")
	}
	if !containsSubstr(got, "工作区背景") || !containsSubstr(got, "InternLM") {
		t.Fatalf("missing context: %s", got)
	}
}

func containsSubstr(s, sub string) bool {
	return len(sub) == 0 || (len(s) >= len(sub) && indexOf(s, sub) >= 0)
}

func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}
