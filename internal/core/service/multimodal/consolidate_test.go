package multimodal

import (
	"strings"
	"testing"
)

func TestConsolidateIngestMarkdown_fromLegacyJSON(t *testing.T) {
	results := []ExtractionResult{
		{Filename: "S2接口说明版本发布_1.jpg", Text: legacyJSON1},
		{Filename: "S2接口说明版本发布_2.jpg", Text: legacyJSON2},
	}
	userNote := "梳理附近中模型发布时间和抓取建议"
	docTitle := "InternVL 接口发布时间表"
	final, gotTitle, ok := FinalizeIngestText(userNote, results, docTitle)
	if !ok {
		t.Fatal("expected consolidation")
	}
	if gotTitle != docTitle {
		t.Fatalf("title=%q want %q", gotTitle, docTitle)
	}
	if strings.Contains(final, "# "+userNote) {
		t.Fatalf("user hint must not be H1: %s", final)
	}
	if !strings.Contains(final, "导入说明") || !strings.Contains(final, userNote) {
		t.Fatalf("user note missing as blockquote: %s", final)
	}
	if !strings.Contains(final, "InternVL") || !strings.Contains(final, "视觉编码") || !strings.Contains(final, "文本编码") {
		t.Fatalf("missing rows: %s", final)
	}
	if strings.Contains(final, "分图识别详情") {
		t.Fatal("per-image filename sections must not be included in consolidated ingest text")
	}
	if strings.Contains(final, ".jpg") {
		t.Fatal("upload filenames must not appear in consolidated ingest text")
	}
}

func TestExtractFactRowsPaired_twoPerImage(t *testing.T) {
	text := "版本发布：InternVL\n05.22 - 05.24\n批大小\n版本发布：视觉编码\n05.29 - 05.31\n分辨率"
	rows := extractFactRowsPaired(text)
	if len(rows) < 2 {
		t.Fatalf("want 2 rows, got %d: %+v", len(rows), rows)
	}
}

const legacyJSON1 = `json
{
  "text_content": "S2 活动日历拆解图鉴\n\n5月\n- 时间：05.22 - 05.24，版本发布：InternVL\n- 时间：05.29 - 05.31，版本发布：视觉编码"
}`

const legacyJSON2 = `{
  "text_content": "版本发布：文本编码 06.19 - 06.21\n光合球 易捕捉草系、光系\n合成材料：喵喵草、象牙花、凤眼莲"
}`

func TestFormatExtractionText_stripsJsonFence(t *testing.T) {
	raw := "```json\n{\"text_content\":\"版本发布：灵狐\\n07.03 - 07.05\"}\n```"
	got := FormatExtractionText(raw)
	if got == "" || strings.Contains(got, "visual_elements") {
		t.Fatalf("got %q", got)
	}
	if !strings.Contains(got, "灵狐") {
		t.Fatalf("want 灵狐 in %q", got)
	}
}

func TestIsImportIntentNote(t *testing.T) {
	if !IsImportIntentNote("梳理附近中模型发布时间和抓取建议") {
		t.Fatal("expected intent note")
	}
	if IsImportIntentNote("S2「接口说明」版本发布时间整理及抓取建议") {
		t.Fatal("filename-like title should not be intent")
	}
}
