package workspacecontext

import (
	"testing"

	"github.com/graydraft/core/model"
)

func TestFormatForMultimodal(t *testing.T) {
	p := model.ContextProfile{
		Description: "InternLM 教程",
		Entities:    []string{"模型", "发布时间"},
		Rules:       []string{"忽略视觉描述"},
	}
	out := FormatForMultimodal(p)
	if out == "" || !contains(out, "核心实体") || !contains(out, "InternLM") {
		t.Fatalf("unexpected: %q", out)
	}
}

func TestFormatEmbeddingPrefix(t *testing.T) {
	p := model.ContextProfile{
		Description: "InternLM",
		Entities:    []string{"模型"},
	}
	prefix := FormatEmbeddingPrefix("ws_abc", p)
	if prefix == "" || !contains(prefix, "模型") {
		t.Fatalf("prefix: %q", prefix)
	}
}

func TestEntityContextBonus(t *testing.T) {
	p := model.ContextProfile{Entities: []string{"小夜"}}
	bonus := EntityContextBonus("小夜何时发布", p, []model.Chunk{
		{Content: "多模态模型小夜发布时间", DocTitle: "发布表"},
	})
	if bonus <= 0 {
		t.Fatal("expected bonus")
	}
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || len(sub) == 0 ||
		(len(s) > 0 && len(sub) > 0 && stringIndex(s, sub)))
}

func stringIndex(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
