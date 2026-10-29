package agent

import (
	"strings"
	"testing"
)

func TestSanitizeUserThought(t *testing.T) {
	raw := `我们被要求根据用户请求："怎么获得跑车"。kb_type必须是gd_60303ae22b998861bce3b28f33eec1be。所以我们发出一个tool_calls。thought_summary应该用中文。`
	got := SanitizeUserThought(raw)
	if got == "" {
		t.Fatal("empty")
	}
	for _, leak := range []string{"gd_", "kb_type", "tool_calls", "thought_summary"} {
		if strings.Contains(got, leak) {
			t.Fatalf("leaked %q in %q", leak, got)
		}
	}
	short := SanitizeUserThought("证据不足，补充检索跑车获取方式。")
	if short != "证据不足，补充检索跑车获取方式。" {
		t.Fatalf("short=%q", short)
	}
}
