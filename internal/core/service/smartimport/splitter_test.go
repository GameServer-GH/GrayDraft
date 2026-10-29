package smartimport

import (
	"strings"
	"testing"
)

func TestSplitRawText_separatorsStaySingleWithoutUnifiedPlan(t *testing.T) {
	raw := strings.Repeat("第一段社区规范说明内容。", 4) + "\n---\n" + strings.Repeat("第二段产品说明 index_ready 介绍。", 4)
	segs := SplitRawText(raw)
	if len(segs) != 1 {
		t.Fatalf("expected single segment, got %d: %#v", len(segs), segs)
	}
}

func TestSplitRawText_qaSingleSegment(t *testing.T) {
	raw := strings.Join([]string{
		"问：如何重置密码？",
		"答：在设置页点击重置。",
		"问：如何开启 2FA？",
		"答：安全设置里开启。",
	}, "\n")
	segs := SplitRawText(raw)
	if len(segs) != 1 {
		t.Fatalf("expected single segment for short QA block, got %d", len(segs))
	}
}

func TestSplitRawText_empty(t *testing.T) {
	if got := SplitRawText("   "); got != nil {
		t.Fatalf("expected nil, got %#v", got)
	}
}
