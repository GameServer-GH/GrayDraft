package model

import "testing"

func TestQuerySemantic4D_CacheFingerprint_stable(t *testing.T) {
	a := QuerySemantic4D{
		Domain: "教程资料", Scene: "版本发布", Style: "时间表",
		Entities: []string{"模型", "S2"},
		Method:   "rules",
	}
	b := QuerySemantic4D{
		Domain: "教程资料", Scene: "版本发布", Style: "时间表",
		Entities: []string{"S2", "模型"},
		Method:   "llm",
	}
	if a.CacheFingerprint() != b.CacheFingerprint() {
		t.Fatalf("expected stable fingerprint, got %q vs %q", a.CacheFingerprint(), b.CacheFingerprint())
	}
}

func TestQuerySemantic4DCacheFingerprintWhenInactive(t *testing.T) {
	var q QuerySemantic4D
	if q.CacheFingerprint() != "" {
		t.Fatal("expected empty fingerprint")
	}
}
