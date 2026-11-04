package smartimport

import (
	"strings"
	"testing"

	"github.com/graydraft/core/model"
)

func TestInjectVirtualOutline(t *testing.T) {
	raw := "第一段内容\n\n第二段关于InternVL\n\n第三段"
	outline := []VirtualOutlineSection{
		{Level: 2, Title: "发布时间表", StartLine: 1},
		{Level: 3, Title: "InternVL", StartLine: 3},
	}
	got := injectVirtualOutline(raw, outline)
	if !strings.Contains(got, "## 发布时间表") || !strings.Contains(got, "### InternVL") {
		t.Fatalf("got %q", got)
	}
}

func TestApplySemanticPivot_structuredMarkdown(t *testing.T) {
	req := &model.SmartImportRequest{RawText: "old", Extensions: model.Extensions{}}
	pivot := SemanticPivotResult{
		Tags4D:             SemanticTags4D{Domain: "教程资料", Scene: "版本发布", Style: "时间表"},
		GlobalSummary:      "摘要",
		StructuredMarkdown: "# InternVL 接口发布时间表\n\n## 要点汇总\n\n| 主题 | 时间 |\n| InternVL | 05.22-05.24 |",
	}
	ApplySemanticPivot(req, pivot)
	if req.RawText != "old" {
		t.Fatalf("raw=%q", req.RawText)
	}
	if req.Extensions["global_summary"] != "摘要" {
		t.Fatalf("extensions=%v", req.Extensions)
	}
}

func TestPivotPreservesScientificSource(t *testing.T) {
	original := "# 数据\n\n观测信号32.4，时间差1.7秒；适用条件必须保留。"
	req := &model.SmartImportRequest{RawText: original}
	ApplySemanticPivot(req, SemanticPivotResult{StructuredMarkdown: "# 摘要\n\n只有概括，没有任何原始观测数据。", Outline: []VirtualOutlineSection{{Level: 2, Title: "新标题", StartLine: 1}}})
	if req.RawText != original {
		t.Fatal("source changed")
	}
	applyUnifiedTitleToRaw(req, "不同标题")
	if req.RawText != original {
		t.Fatal("title rewrote source")
	}
}
