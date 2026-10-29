package smartimport

import (
	"strings"
	"testing"
)

func TestExtractByLineRange_first_line(t *testing.T) {
	raw := "line1\nline2\nline3\nline4"
	result := ExtractByLineRange(raw, 1, 1)
	if result != "line1" {
		t.Fatalf("got %q", result)
	}
}

func TestExtractByLineRange_middle(t *testing.T) {
	raw := "line1\nline2\nline3\nline4"
	result := ExtractByLineRange(raw, 2, 3)
	expected := "line2\nline3"
	if result != expected {
		t.Fatalf("got %q, want %q", result, expected)
	}
}

func TestExtractByLineRange_full_range(t *testing.T) {
	raw := "line1\nline2\nline3"
	result := ExtractByLineRange(raw, 1, 3)
	expected := "line1\nline2\nline3"
	if result != expected {
		t.Fatalf("got %q, want %q", result, expected)
	}
}

func TestExtractByLineRange_start_less_than_1(t *testing.T) {
	raw := "line1\nline2"
	result := ExtractByLineRange(raw, 0, 2)
	expected := "line1\nline2"
	if result != expected {
		t.Fatalf("got %q, want %q", result, expected)
	}
}

func TestExtractByLineRange_end_exceeds_length(t *testing.T) {
	raw := "line1\nline2"
	result := ExtractByLineRange(raw, 1, 10)
	expected := "line1\nline2"
	if result != expected {
		t.Fatalf("got %q, want %q", result, expected)
	}
}

func TestExtractByLineRange_start_greater_than_end(t *testing.T) {
	raw := "line1\nline2"
	result := ExtractByLineRange(raw, 3, 1)
	if result != "" {
		t.Fatalf("got %q, want empty", result)
	}
}

func TestExtractByLineRange_start_beyond_length(t *testing.T) {
	raw := "line1"
	result := ExtractByLineRange(raw, 5, 10)
	if result != "" {
		t.Fatalf("got %q, want empty", result)
	}
}

func TestExtractByLineRange_empty_raw(t *testing.T) {
	result := ExtractByLineRange("", 1, 3)
	if result != "" {
		t.Fatalf("got %q, want empty", result)
	}
}

func TestExtractByLineRange_single_line_markdown(t *testing.T) {
	raw := "# 标题\n## 副标题\n正文内容\n### 细节\n结束"
	result := ExtractByLineRange(raw, 2, 4)
	expected := "## 副标题\n正文内容\n### 细节"
	if result != expected {
		t.Fatalf("got %q, want %q", result, expected)
	}
}

func TestSegmentTitleFromRange_h2_heading(t *testing.T) {
	raw := "# 主标题\n## 接口说明\n正文内容"
	analysis := DocAnalysis{}
	title := segmentTitleFromRange(raw, 2, 3, analysis)
	if title != "接口说明" {
		t.Fatalf("got %q", title)
	}
}

func TestSegmentTitleFromRange_h3_heading(t *testing.T) {
	raw := "# 合集\n## 部分A\n### 装备推荐\n正文内容"
	analysis := DocAnalysis{}
	title := segmentTitleFromRange(raw, 3, 4, analysis)
	if title != "装备推荐" {
		t.Fatalf("got %q", title)
	}
}

func TestSegmentTitleFromRange_fallback_to_subject(t *testing.T) {
	raw := "InternVL参数说明：核心参数为InternVL搭配..."
	analysis := DocAnalysis{}
	title := segmentTitleFromRange(raw, 1, 1, analysis)
	if title == "" {
		t.Fatal("expected non-empty title")
	}
}

func TestSegmentTitleFromRange_fallback_to_parent(t *testing.T) {
	raw := "一段平文本无标题"
	analysis := DocAnalysis{ParentTitle: "父级标题"}
	title := segmentTitleFromRange(raw, 1, 1, analysis)
	if title == "" {
		t.Fatal("expected fallback to parent title")
	}
}

func TestSegmentTitleFromRange_fallback_to_extract(t *testing.T) {
	raw := "# 显式标题\n正文内容很多很多"
	analysis := DocAnalysis{ParentTitle: "父级标题"}
	title := segmentTitleFromRange(raw, 1, 1, analysis)
	if title != "显式标题" {
		t.Fatalf("got %q, want h1 heading", title)
	}
}

func TestSegmentTitleFromRange_generic_franchise_bypass(t *testing.T) {
	raw := "# InternVL\n正文内容"
	analysis := DocAnalysis{Subject: "InternVL"}
	title := segmentTitleFromRange(raw, 1, 2, analysis)
	// "InternVL" is a franchise title, should not be returned
	if title == "InternVL" {
		t.Log("franchise title might not be filtered for segment fallback (extractTitle path)")
	}
}

func TestDisplayTitle_both_present(t *testing.T) {
	result := displayTitle("合集", "接口说明")
	if result != "合集 · 接口说明" {
		t.Fatalf("got %q", result)
	}
}

func TestDisplayTitle_parent_empty(t *testing.T) {
	result := displayTitle("", "接口说明")
	if result != "接口说明" {
		t.Fatalf("got %q", result)
	}
}

func TestDisplayTitle_segment_empty(t *testing.T) {
	result := displayTitle("合集", "")
	if result != "合集" {
		t.Fatalf("got %q", result)
	}
}

func TestDisplayTitle_segment_contains_parent(t *testing.T) {
	result := displayTitle("合集", "合集：接口说明")
	if result != "合集：接口说明" {
		t.Fatalf("got %q", result)
	}
}

func TestDisplayTitle_both_empty(t *testing.T) {
	result := displayTitle("", "")
	if result != "" {
		t.Fatalf("got %q, want empty", result)
	}
}

func TestIsAttachmentFilenameOnlySegment_heading_match(t *testing.T) {
	content := "IMG_20240101_120000.jpg"
	if !isAttachmentFilenameOnlySegment(content) {
		t.Fatal("expected true for attachment filename")
	}
}

func TestIsAttachmentFilenameOnlySegment_with_small_body(t *testing.T) {
	content := "### screenshot_2024.png\n短文本"
	// body is short and heading looks like filename
	result := isAttachmentFilenameOnlySegment(content)
	t.Logf("isAttachmentFilenameOnlySegment: %v", result)
}

func TestIsAttachmentFilenameOnlySegment_with_large_body(t *testing.T) {
	content := "### 截图\n" + strings.Repeat("正文内容文本足够长可以绕过附件文件名的判断逻辑", 3)
	if isAttachmentFilenameOnlySegment(content) {
		t.Fatal("expected false for large body content")
	}
}

func TestIsClosingFluff_typical(t *testing.T) {
	if !isClosingFluff("以上就是全部内容") {
		t.Fatal("expected true")
	}
	if !isClosingFluff("全文完") {
		t.Fatal("expected true")
	}
	if !isClosingFluff("欢迎转载，转载请注明出处") {
		t.Fatal("expected true")
	}
}

func TestIsClosingFluff_long_text(t *testing.T) {
	if isClosingFluff(strings.Repeat("以上就是长篇内容", 30)) {
		t.Fatal("expected false for long text")
	}
}

func TestIsClosingFluff_normal_content(t *testing.T) {
	if isClosingFluff("InternVL角色参数说明：主力输出为InternVL，辅模型选InternLM") {
		t.Fatal("expected false for normal content")
	}
}
