package chat

import (
	"testing"

	"github.com/graydraft/core/model"
	"github.com/graydraft/core/service/confidence"
)

func TestKBAnswer_emptyUsesNotRecorded(t *testing.T) {
	if got := KBAnswer(nil); got != KBNotRecordedAnswer() {
		t.Fatalf("empty chunks: got %q", got)
	}
}

func TestKBNotRecordedAnswer(t *testing.T) {
	if !contains(KBNotRecordedAnswer(), "未入录") {
		t.Fatal("expected not-recorded wording")
	}
}

func TestBuildUnifiedMessages_kbOnly_includesChunks(t *testing.T) {
	msgs := BuildUnifiedMessages("InternVL 接口", "tutorial", []model.Chunk{
		{Content: "Q 技能说明", DocTitle: "技能速查"},
	}, confidence.GradeA, nil, ChatModeKBOnly, nil)
	if len(msgs) != 2 || msgs[0].Role != "system" {
		t.Fatalf("unexpected messages: %+v", msgs)
	}
	for _, sub := range []string{"仅知识库", "技能速查", "高可信"} {
		if !contains(msgs[0].Content, sub) {
			t.Fatalf("kb_only system prompt missing %q: %s", sub, msgs[0].Content)
		}
	}
}

func TestBuildUnifiedMessages_hybrid_includesChunks(t *testing.T) {
	msgs := BuildUnifiedMessages("什么是 RAG", "faq", []model.Chunk{
		{Content: "GrayDraft 支持混合检索", DocTitle: "doc1"},
	}, confidence.GradeA, nil, ChatModeHybrid, nil)
	if len(msgs) != 2 {
		t.Fatalf("expected 2 messages")
	}
	if msgs[0].Role != "system" || !contains(msgs[0].Content, "参考素材") {
		t.Fatalf("system prompt missing context: %s", msgs[0].Content)
	}
	for _, sub := range []string{"高可信", "doc1", "GrayDraft"} {
		if !contains(msgs[0].Content, sub) {
			t.Fatalf("hybrid RAG system prompt missing %q", sub)
		}
	}
}

func TestBuildUnifiedMessages_GradeD_NoChunks(t *testing.T) {
	msgs := BuildUnifiedMessages("什么是 RAG", "faq", nil, confidence.GradeD, nil, ChatModeHybrid, nil)
	if len(msgs) != 2 {
		t.Fatalf("expected 2 messages")
	}
	if !contains(msgs[0].Content, "未检索到") {
		t.Fatalf("GradeD prompt missing no-hits signal: %s", msgs[0].Content)
	}
	if !contains(msgs[0].Content, "通用知识") {
		t.Fatalf("GradeD prompt missing generic knowledge instruction: %s", msgs[0].Content)
	}
}

func TestKBCoverageBoundary(t *testing.T) {
	if !contains(KBCoverageBoundary(nil), "未收录") {
		t.Fatal("expected not-recorded wording")
	}
	if got := KBCoverageBoundary([]string{"实体A", "实体B"}); !contains(got, "实体A") {
		t.Fatalf("expected entity list: %s", got)
	}
}

func TestBuildGeneralMessages(t *testing.T) {
	msgs := BuildGeneralMessages("test")
	if len(msgs) != 2 {
		t.Fatalf("expected 2 messages")
	}
	if !contains(msgs[0].Content, "通用知识") {
		t.Fatalf("general prompt missing: %s", msgs[0].Content)
	}
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || len(sub) == 0 || stringIndex(s, sub) >= 0)
}

func stringIndex(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}
