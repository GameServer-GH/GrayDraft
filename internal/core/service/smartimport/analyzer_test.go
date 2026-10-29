package smartimport

import (
	"strings"
	"testing"
)

func TestAnalyze_workspaceRequired(t *testing.T) {
	_, err := Analyze("# 标题\n\n正文内容足够长。", "", "", "", "", false, nil, nil)
	if err == nil || !strings.Contains(err.Error(), "hint_kb_type") {
		t.Fatalf("expected workspace kb error, got %v", err)
	}
}

func TestAnalyze_withWorkspacePartition(t *testing.T) {
	hint := "gd_9f86d081884c7d659a2feaa0c55ad015"
	raw := `# 社区发帖规范

## 禁止的内容
- 广告、引流
`
	d, err := Analyze(raw, "rules.md", "", hint, "", false, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if d.KBType != hint {
		t.Fatalf("kb_type=%s", d.KBType)
	}
	if d.Title != "社区发帖规范" {
		t.Fatalf("title=%q", d.Title)
	}
}

func TestAnalyze_workspaceHintNotOverriddenByLLM(t *testing.T) {
	hint := "gd_9f86d081884c7d659a2feaa0c55ad015"
	raw := `# InternVL 教程

版本 S14 产品功能 api 配置 安装`
	llm := func(string) (string, error) {
		return `{"title":"InternVL技巧","confidence":0.95,"reason":"test"}`, nil
	}
	d, err := Analyze(raw, "", "", hint, "", true, llm, nil)
	if err != nil {
		t.Fatal(err)
	}
	if d.KBType != hint {
		t.Fatalf("kb_type=%s want workspace partition", d.KBType)
	}
}

func TestResolveKBType_rejectsLegacy(t *testing.T) {
	if _, _, ok := ResolveKBType("unknown_partition"); ok == "" {
		t.Fatal("unknown partition should not resolve")
	}
	if kb, _, _ := ResolveKBType("gd_abc123"); kb != "gd_abc123" {
		t.Fatal("workspace should resolve")
	}
}
