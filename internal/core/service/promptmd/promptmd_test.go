package promptmd

import "testing"

func TestStandardBlock_containsFourDimensions(t *testing.T) {
	for _, sub := range []string{"角色", "格式规范", "内容约束", "负向约束", "Skip the preamble"} {
		if !contains(StandardBlock, sub) {
			t.Fatalf("StandardBlock missing %q", sub)
		}
	}
}

func TestLineBreakRules_containsExample(t *testing.T) {
	if !contains(LineBreakRules, "优先推主线") {
		t.Fatal("expected numbered-list example")
	}
}

func contains(s, sub string) bool {
	return len(sub) == 0 || (len(s) >= len(sub) && index(s, sub) >= 0)
}

func index(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}
