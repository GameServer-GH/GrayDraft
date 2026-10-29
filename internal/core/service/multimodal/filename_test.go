package multimodal

import "testing"

func TestLooksLikeAttachmentFilename(t *testing.T) {
	name := "S2「接口说明」版本发布时间整理及抓取建议_2_少冰半糖幽兰_来自小红书网页版.jpg"
	if !LooksLikeAttachmentFilename(name) {
		t.Fatalf("expected attachment filename: %q", name)
	}
	if LooksLikeAttachmentFilename("InternVL 接口发布时间表") {
		t.Fatal("guide title should not match")
	}
}

func TestPerImageSectionTitle_skipsFilename(t *testing.T) {
	got := PerImageSectionTitle("", 2, "foo_2_bar.jpg")
	if got != "图 2" {
		t.Fatalf("got %q", got)
	}
}
