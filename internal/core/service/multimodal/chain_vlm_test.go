package multimodal

import (
	"strings"
	"testing"

	"github.com/graydraft/core/model"
)

func TestShouldChainVLM(t *testing.T) {
	atts := []model.SmartImportAttachment{
		{Filename: "a.png", MimeType: "image/png"},
		{Filename: "b.jpg", MimeType: "image/jpeg"},
	}
	if !shouldChainVLM(atts) {
		t.Fatal("expected chain for two images")
	}
	atts = []model.SmartImportAttachment{
		{Filename: "a.png", MimeType: "image/png"},
		{Filename: "b.txt", MimeType: "text/plain"},
	}
	if shouldChainVLM(atts) {
		t.Fatal("expected no chain when only one visual")
	}
}

func TestAppendChainVLMPrompt(t *testing.T) {
	base := "提取知识"
	got := appendChainVLMPrompt(base, &chainContextParams{
		Index:        2,
		Total:        3,
		PriorContext: "图1：InternVL 参数列表",
	})
	if !strings.Contains(got, "图1：InternVL 参数列表") || !strings.Contains(got, "2/3") {
		t.Fatalf("prompt missing chain context: %q", got)
	}
	got2 := appendChainVLMPrompt(base, nil)
	if got2 != base {
		t.Fatal("nil chain should not modify prompt")
	}
}

func TestExtractChainSummary_fromJSON(t *testing.T) {
	raw := `{"summary":"InternVL 接口发布","text_content":"样例图在周二出现"}`
	parsed := parseModelOutput(raw)
	sum := extractChainSummary(raw, parsed)
	if sum != "InternVL 接口发布" {
		t.Fatalf("summary=%q", sum)
	}
}

func TestBuildChainContext(t *testing.T) {
	ctx := buildChainContext("gear.png", ExtractionResult{
		ChainSummary: "装备与材料对照",
	}, 1, 2)
	if !strings.Contains(ctx, "装备与材料") || !strings.Contains(ctx, "1/2") {
		t.Fatalf("context=%q", ctx)
	}
}
