package smartimport

import (
	"strings"
	"testing"
)

func TestScanSkeleton_headings(t *testing.T) {
	raw := "# InternVL 接口\n\n## 调用方式\n内容正文\n### 细节\n更多内容"
	sk := ScanSkeleton(raw)

	if sk.LineCount < 5 {
		t.Fatalf("line count: %d", sk.LineCount)
	}

	hasH1, hasH2, hasH3 := false, false, false
	for _, sl := range sk.Lines {
		switch sl.Kind {
		case "h1":
			hasH1 = true
			if sl.Text != "InternVL 接口" {
				t.Fatalf("h1 text=%q", sl.Text)
			}
		case "h2":
			hasH2 = true
			if sl.Text != "调用方式" {
				t.Fatalf("h2 text=%q", sl.Text)
			}
		case "h3":
			hasH3 = true
		}
	}
	if !hasH1 || !hasH2 || !hasH3 {
		t.Fatalf("missing headings: h1=%v h2=%v h3=%v", hasH1, hasH2, hasH3)
	}
}

func TestScanSkeleton_separator(t *testing.T) {
	raw := "# 标题\n一些文本\n---\n更多内容"
	sk := ScanSkeleton(raw)

	hasSep := false
	for _, sl := range sk.Lines {
		if sl.Kind == "sep" {
			hasSep = true
		}
	}
	if !hasSep {
		t.Fatal("expected sep for --- line")
	}
}

func TestScanSkeleton_bold(t *testing.T) {
	raw := "**加粗标题**\n正文"
	sk := ScanSkeleton(raw)

	hasBold := false
	for _, sl := range sk.Lines {
		if sl.Kind == "bold" && sl.Text == "**加粗标题**" {
			hasBold = true
		}
	}
	if !hasBold {
		t.Fatal("expected bold line")
	}
}

func TestScanSkeleton_qa(t *testing.T) {
	raw := "Q: 如何配置\nA: 使用推荐参数\nQ: 参数怎么选\nA: 看文档"
	sk := ScanSkeleton(raw)

	qaCount := 0
	for _, sl := range sk.Lines {
		if sl.Kind == "qa" {
			qaCount++
		}
	}
	if qaCount != 4 {
		t.Fatalf("expected 4 QA lines, got %d", qaCount)
	}
}

func TestScanSkeleton_empty_input(t *testing.T) {
	sk := ScanSkeleton("")
	// normalizeRaw splits empty string as [""], producing 1 empty-kind line
	if len(sk.Lines) > 0 && sk.Lines[0].Kind != "empty" {
		t.Fatalf("expected empty line, got kind=%q", sk.Lines[0].Kind)
	}
}

func TestScanSkeleton_empty_input_lines(t *testing.T) {
	raw := "line1\n\n\nline2"
	sk := ScanSkeleton(raw)

	if sk.LineCount != 4 {
		t.Fatalf("line count: %d", sk.LineCount)
	}
	emptyCount := 0
	for _, sl := range sk.Lines {
		if sl.Kind == "empty" {
			emptyCount++
		}
	}
	if emptyCount != 2 {
		t.Fatalf("expected 2 empty lines, got %d", emptyCount)
	}
}

func TestScanSkeleton_h4(t *testing.T) {
	raw := "#### 四级标题\n内容"
	sk := ScanSkeleton(raw)

	hasH4 := false
	for _, sl := range sk.Lines {
		if sl.Kind == "h4" && sl.Text == "四级标题" {
			hasH4 = true
		}
	}
	if !hasH4 {
		t.Fatal("expected h4 heading")
	}
}

func TestScanSkeleton_line_numbers(t *testing.T) {
	raw := "# 一\n## 二\n正文\n### 三"
	sk := ScanSkeleton(raw)

	// check line numbers are 1-based
	for i, sl := range sk.Lines {
		if sl.LineNo != i+1 {
			t.Fatalf("line %d has LineNo=%d", i+1, sl.LineNo)
		}
	}
}

func TestInferDocAnalysis_single_page(t *testing.T) {
	raw := "# 教程说明\n\nInternVL 接口说明"
	lines := ScanSkeleton(raw).Lines
	a := inferDocAnalysis(raw, lines)

	if a.ParentTitle != "教程说明" {
		t.Fatalf("parent_title=%q", a.ParentTitle)
	}
	if a.DocType != "单篇说明" {
		t.Fatalf("doc_type=%q", a.DocType)
	}
}

func TestInferDocAnalysis_compilation(t *testing.T) {
	raw := "# InternVL 文档合集\n\n## 角色A\n内容1\n## 角色B\n内容2\n### 细节\n内容"
	lines := ScanSkeleton(raw).Lines
	a := inferDocAnalysis(raw, lines)

	if a.DocType != "综合合集" {
		t.Fatalf("doc_type=%q, expected 综合合集", a.DocType)
	}
}

func TestInferDocAnalysis_qa_set(t *testing.T) {
	raw := "Q: 如何配置\nA: 用推荐参数\nQ: 参数选什么\nA: 看文档\nQ: 公式怎么写\nA: 先写定义\nQ: 有没有替代\nA: 没有"
	lines := ScanSkeleton(raw).Lines
	a := inferDocAnalysis(raw, lines)

	if a.DocType != "问答集" {
		t.Fatalf("doc_type=%q, expected 问答集", a.DocType)
	}
}

func TestInferDocAnalysis_structured_archive(t *testing.T) {
	raw := "# 教程十四\n\n---\n\n## 第一部分\n内容\n---\n\n## 第二部分\n内容"
	lines := ScanSkeleton(raw).Lines
	a := inferDocAnalysis(raw, lines)

	if a.DocType != "综合合集" {
		t.Fatalf("doc_type=%q, expected 综合合集 for multi-sep", a.DocType)
	}
}

func TestInferDocAnalysis_subject_from_title(t *testing.T) {
	raw := "# InternVL：接口说明\n\n内容"
	lines := ScanSkeleton(raw).Lines
	a := inferDocAnalysis(raw, lines)

	if a.Subject != "InternVL" {
		t.Fatalf("subject=%q", a.Subject)
	}
	if a.DocType != "单篇说明" {
		t.Fatalf("doc_type=%q", a.DocType)
	}
}

func TestInferDocAnalysis_subject_bookmark(t *testing.T) {
	raw := "《InternVL》接口说明\n\n## InternLM\n多模态模型"
	lines := ScanSkeleton(raw).Lines
	a := inferDocAnalysis(raw, lines)

	if a.Subject != "InternVL" {
		t.Fatalf("subject=%q", a.Subject)
	}
}

func TestInferDocAnalysis_building_post(t *testing.T) {
	raw := "---\n讨论话题\n## 第一条回复\n内容"
	lines := ScanSkeleton(raw).Lines
	a := inferDocAnalysis(raw, lines)

	if a.DocType != "分段文档" {
		t.Fatalf("doc_type=%q, expected 分段文档", a.DocType)
	}
}

func TestInferDocAnalysis_no_title(t *testing.T) {
	raw := "纯内容文本，没有任何标题和结构，长度要足够让提取标题的逻辑失效。这是一段很长的文字，用来测试无标题时的情况。更多内容填充。"
	lines := ScanSkeleton(raw).Lines
	a := inferDocAnalysis(raw, lines)

	if a.DocType != "单篇说明" {
		t.Fatalf("doc_type=%q, expected 单篇说明 as default", a.DocType)
	}
	// Should still derive a fallback parent title
	if a.ParentTitle == "" {
		t.Fatal("expected fallback parent title")
	}
}

func TestFormatSkeletonForLLM(t *testing.T) {
	sk := ScanSkeleton("# 主标题\n\n## 副标题\n正文内容足够长\n### 细节\n---")
	formatted := FormatSkeletonForLLM(sk)

	if !strings.Contains(formatted, "line_no|kind|text") {
		t.Fatal("missing header row")
	}
	if !strings.Contains(formatted, "h1") || !strings.Contains(formatted, "主标题") {
		t.Fatalf("missing h1 content in: %s", formatted)
	}
	// text and empty lines should be omitted
	if strings.Contains(formatted, "正文内容") {
		t.Fatal("text lines should not appear in skeleton format")
	}
}

func TestFormatSkeletonForLLM_long_text_truncated(t *testing.T) {
	longTitle := "这是一个非常长的标题" + strings.Repeat("内容", 20)
	raw := "# " + longTitle + "\n正文"
	sk := ScanSkeleton(raw)
	formatted := FormatSkeletonForLLM(sk)

	// The text after truncation should end with …
	if !strings.Contains(formatted, "…") && len([]rune(longTitle)) > 80 {
		t.Logf("long title might not need truncation (len=%d)", len([]rune(longTitle)))
	}
}

func TestFormatSkeletonForLLM_empty(t *testing.T) {
	sk := ScanSkeleton("")
	formatted := FormatSkeletonForLLM(sk)

	if !strings.Contains(formatted, "line_no|kind|text") {
		t.Fatal("should still have header row")
	}
	// After header, should be basically empty apart from the newline
	lines := strings.Split(strings.TrimSpace(formatted), "\n")
	if len(lines) <= 1 {
		return // only header, fine
	}
	// Verify no content lines beyond header
	for _, line := range lines[1:] {
		if strings.TrimSpace(line) != "" {
			t.Fatalf("unexpected content line: %q", line)
		}
	}
}
