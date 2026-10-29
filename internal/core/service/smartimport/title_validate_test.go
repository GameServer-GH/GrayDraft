package smartimport

import "testing"

func TestIsInvalidTitle(t *testing.T) {
	for _, bad := range []string{
		"json", "JSON", "```json", "{", "## 附件: x.jpg",
		"多图要点汇总", "要点汇总",
		"S2「接口说明」版本发布时间整理及抓取建议_2_少冰半糖幽兰_来自小红书网页版.jpg",
	} {
		if !isInvalidTitle(bad) {
			t.Fatalf("expected invalid: %q", bad)
		}
	}
	if isInvalidTitle("InternVL 接口大量发布时间表") {
		t.Fatal("expected valid game guide title")
	}
}
