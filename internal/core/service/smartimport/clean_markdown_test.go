package smartimport

import (
	"strings"
	"testing"
)

func TestCleanMarkdownForIngest_Headings(t *testing.T) {
	input := "### 参数说明\n\n推荐使用InternVL。"
	output := CleanMarkdownForIngest(input)
	if strings.Contains(output, "###") {
		t.Errorf("output should not contain '###', got: %q", output)
	}
	if !strings.Contains(output, "参数说明") {
		t.Errorf("output should preserve heading text, got: %q", output)
	}
}

func TestCleanMarkdownForIngest_TableConversion(t *testing.T) {
	input := "| 主题 | 时间 | 要点 |\n| --- | --- | --- |\n| 模型A | 3.1-3.15 | 版本发布 |"
	output := CleanMarkdownForIngest(input)
	if strings.Contains(output, "|") {
		t.Errorf("output should not contain pipe chars, got: %q", output)
	}
	// Should have KV format: 主题：模型A，时间：3.1-3.15，要点：版本发布
	if !strings.Contains(output, "主题") && !strings.Contains(output, "模型A") {
		t.Errorf("output should contain table data in NL format, got: %q", output)
	}
}

func TestCleanMarkdownForIngest_BoldAndItalic(t *testing.T) {
	input := "**重要**提示：_请注意_装备选择。"
	output := CleanMarkdownForIngest(input)
	if strings.Contains(output, "**") {
		t.Errorf("output should not contain '**', got: %q", output)
	}
	if !strings.Contains(output, "重要") {
		t.Errorf("output should preserve bold text, got: %q", output)
	}
}

func TestCleanMarkdownForIngest_Links(t *testing.T) {
	input := "参考[官方教程](https://example.com)获取详情。"
	output := CleanMarkdownForIngest(input)
	if strings.Contains(output, "https://") || strings.Contains(output, "](") {
		t.Errorf("output should not contain link syntax, got: %q", output)
	}
	if !strings.Contains(output, "官方教程") {
		t.Errorf("output should preserve link text, got: %q", output)
	}
}

func TestCleanMarkdownForIngest_CodeBlocks(t *testing.T) {
	input := "配置如下：\n```\nconst MAX = 100\n```\n以上为示例。"
	output := CleanMarkdownForIngest(input)
	if strings.Contains(output, "```") {
		t.Errorf("output should not contain code fences, got: %q", output)
	}
}

func TestCleanMarkdownForIngest_HorizontalRule(t *testing.T) {
	input := "第一段\n\n---\n\n第二段"
	output := CleanMarkdownForIngest(input)
	if strings.Contains(output, "---") {
		t.Errorf("output should not contain horizontal rule, got: %q", output)
	}
}

func TestCleanMarkdownForIngest_Blockquote(t *testing.T) {
	input := "> 这是一段引用文字\n> 第二行引用"
	output := CleanMarkdownForIngest(input)
	if strings.Contains(output, ">") {
		t.Errorf("output should not contain '>', got: %q", output)
	}
	if !strings.Contains(output, "这是一段引用文字") {
		t.Errorf("output should preserve quote text, got: %q", output)
	}
}

func TestCleanMarkdownForIngest_ListItems(t *testing.T) {
	input := "- 第一项\n- 第二项\n- 第三项"
	output := CleanMarkdownForIngest(input)
	if strings.Contains(output, "- ") {
		t.Errorf("output should not contain list markers, got: %q", output)
	}
}

func TestCleanMarkdownForIngest_OrderedList(t *testing.T) {
	input := "1. 第一步\n2. 第二步\n3. 第三步"
	output := CleanMarkdownForIngest(input)
	if strings.Contains(output, "1. ") || strings.Contains(output, "2. ") {
		t.Errorf("output should not contain ordered markers, got: %q", output)
	}
}

func TestCleanMarkdownForIngest_Empty(t *testing.T) {
	if CleanMarkdownForIngest("") != "" {
		t.Error("empty input should return empty string")
	}
}

func TestCleanMarkdownForIngest_PreservesContent(t *testing.T) {
	input := "# InternVL 教程\n\n推荐使用**视觉编码器**。详见[链接](url)。"
	output := CleanMarkdownForIngest(input)
	// All key content should be preserved
	for _, keyword := range []string{"InternVL 教程", "视觉编码器", "推荐使用", "链接"} {
		if !strings.Contains(output, keyword) {
			t.Errorf("output missing keyword %q: %q", keyword, output)
		}
	}
	// No markdown syntax should remain
	for _, syntax := range []string{"# ", "**", "](", "```", "---"} {
		if strings.Contains(output, syntax) {
			t.Errorf("output contains markdown syntax %q: %q", syntax, output)
		}
	}
}

func TestSplitTableCells(t *testing.T) {
	cells := splitTableCells("| 主题 | 时间 | 要点 |")
	if len(cells) != 3 {
		t.Fatalf("expected 3 cells, got %d: %v", len(cells), cells)
	}
	if cells[0] != "主题" {
		t.Errorf("first cell = %q, want '主题'", cells[0])
	}
}

func TestIsTableSeparator(t *testing.T) {
	if !isTableSeparator("| --- | --- | --- |") {
		t.Error("should detect table separator")
	}
	if !isTableSeparator("| :--- | ---: | :---: |") {
		t.Error("should detect aligned table separator")
	}
	if isTableSeparator("| data | data |") {
		t.Error("should not misidentify data row as separator")
	}
}
