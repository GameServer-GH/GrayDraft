package smartimport

import "testing"

func TestContentLooksMixed_fromSkeletonDocType(t *testing.T) {
	raw := "第一段\n---\n第二段\n---\n第三段"
	sk := ScanSkeleton(raw + "\n## A\nx\n## B\ny\n## C\nz")
	if !contentLooksMixed(sk) {
		t.Fatalf("expected mixed for doc_type=%q", sk.Analysis.DocType)
	}
}

func TestContentLooksMixed_singleGuide(t *testing.T) {
	raw := `InternVL是《书生文档》中的英雄，技能包括风墙与斩击。`
	sk := ScanSkeleton(raw)
	if contentLooksMixed(sk) {
		t.Fatalf("single guide should not be mixed, doc_type=%q", sk.Analysis.DocType)
	}
}
