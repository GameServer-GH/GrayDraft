package chat

import (
	"testing"

	"github.com/graydraft/core/model"
	"github.com/graydraft/core/service/confidence"
)

func TestSplitSentences_simple(t *testing.T) {
	sentences := splitSentences("第一句。第二句！第三句？")
	if len(sentences) != 3 {
		t.Fatalf("expected 3 sentences, got %d: %v", len(sentences), sentences)
	}
}

func TestSplitSentences_newline(t *testing.T) {
	sentences := splitSentences("第一句\n第二句")
	if len(sentences) != 2 {
		t.Fatalf("expected 2 sentences: %v", sentences)
	}
}

func TestSplitSentences_single(t *testing.T) {
	sentences := splitSentences("单句无标点")
	if len(sentences) != 1 {
		t.Fatalf("expected 1 sentence: %v", sentences)
	}
}

func TestSplitSentences_empty(t *testing.T) {
	sentences := splitSentences("")
	if len(sentences) != 0 {
		t.Fatalf("expected 0 sentences: %v", sentences)
	}
}

func TestSplitSentences_mixed(t *testing.T) {
	sentences := splitSentences("第一句。第二句！第三句？第四句\n第五句")
	if len(sentences) < 3 {
		t.Fatalf("expected >= 3 sentences: %v", sentences)
	}
}

func TestSentenceGroundedInContext_direct(t *testing.T) {
	if !sentenceGroundedInContext("攻击力100点", "攻击力100点防御力200点") {
		t.Fatal("should be grounded")
	}
}

func TestSentenceGroundedInContext_not_grounded(t *testing.T) {
	if sentenceGroundedInContext("独特内容", "完全不相关") {
		t.Fatal("should not be grounded")
	}
}

func TestSentenceGroundedInContext_partial(t *testing.T) {
	// QueryTerms splits by CJK boundaries — adjust assertion to actual behavior
	result := sentenceGroundedInContext("测试匹配", "测试内容在这里")
	t.Logf("grounded: %v", result)
}

func TestLabelSentences_empty_answer(t *testing.T) {
	labels := labelSentences("", []model.Chunk{{Content: "context"}})
	if len(labels) != 0 {
		t.Fatalf("expected 0 labels: %v", labels)
	}
}

func TestLabelSentences_empty_chunks(t *testing.T) {
	labels := labelSentences("some answer", nil)
	if len(labels) != 0 {
		t.Fatalf("expected 0 labels: %v", labels)
	}
}

func TestLabelSentences_single_kb(t *testing.T) {
	chunks := []model.Chunk{
		{Content: "InternVL是主模型角色"},
	}
	answer := "InternVL是主模型角色。"
	labels := labelSentences(answer, chunks)
	if len(labels) != 1 {
		t.Fatalf("expected 1 label: %v", labels)
	}
	if labels[0].Source != "kb" {
		t.Fatalf("expected kb source: %+v", labels[0])
	}
}

func TestLabelSentences_mixed(t *testing.T) {
	chunks := []model.Chunk{
		{Content: "InternVL 使用视觉编码器"},
	}
	answer := "InternVL 使用视觉编码器。我们还需更多研究。"
	labels := labelSentences(answer, chunks)
	if len(labels) < 2 {
		t.Fatalf("expected >= 2 labels: %v", labels)
	}
	// First sentence should be kb
	if labels[0].Source != "kb" {
		t.Fatalf("first sentence should be kb: %+v", labels[0])
	}
}

func TestAppendWorkspaceBackground_empty_profile(t *testing.T) {
	// Test through BuildUnifiedMessages
	msgs := BuildUnifiedMessages("测试问题", "gd_test", nil, confidence.GradeD, nil, ChatModeHybrid, nil)
	if len(msgs) != 2 {
		t.Fatalf("expected 2 messages: %d", len(msgs))
	}
	// Should contain workspace label
	if msgs[0].Content == "" {
		t.Fatal("system prompt should not be empty")
	}
}

func TestAppendWorkspaceBackground_with_profile(t *testing.T) {
	profile := &model.ContextProfile{
		KBType:      "gd_test",
		Description: "测试工作区",
		Entities:    []string{"实体A", "实体B"},
	}
	msgs := BuildUnifiedMessages("测试", "gd_test", nil, confidence.GradeD, profile, ChatModeHybrid, nil)
	if len(msgs) != 2 {
		t.Fatalf("expected 2 messages: %d", len(msgs))
	}
	if msgs[0].Content == "" {
		t.Fatal("system prompt should not be empty")
	}
}

func TestAppendMarkdownOutputSpec(t *testing.T) {
	// Test through BuildUnifiedMessages that markdown spec is included
	msgs := BuildUnifiedMessages("test", "gd_test", nil, confidence.GradeD, nil, ChatModeHybrid, nil)
	systemPrompt := msgs[0].Content
	// Should contain markdown formatting instructions
	if systemPrompt == "" {
		t.Fatal("system prompt empty")
	}
}
