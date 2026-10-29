package multimodal

import (
	"strings"
	"testing"
)

func TestComposePrompt(t *testing.T) {
	got := ComposePrompt(DefaultBasePrompt, "提取装备数值")
	if got == "" {
		t.Fatal("empty")
	}
	if got == DefaultBasePrompt {
		t.Fatal("expected workspace suffix")
	}
}

func TestMergeExtractionsIntoRawText(t *testing.T) {
	out := MergeExtractionsIntoRawText("user note", []ExtractionResult{
		{Filename: "a.png", Text: "screenshot text"},
	}, "测试标题")
	if out == "" || !strings.Contains(out, "screenshot") || !strings.Contains(out, "user note") {
		t.Fatalf("unexpected merge: %q", out)
	}
}
