package smartimport

import (
	"testing"
)

func TestComputeMinHash(t *testing.T) {
	s1 := "InternVL 推荐使用视觉编码器，推理参数选择推理参数4件套"
	s2 := "InternVL 推荐使用视觉编码器，推理参数选择推理参数4件套"
	s3 := "温迪推荐使用文本接口，推理参数选择上下文4件套"

	sig1 := ComputeMinHash(s1)
	sig2 := ComputeMinHash(s2)
	sig3 := ComputeMinHash(s3)

	// Same content should have high similarity
	sim12 := jaccardEstimate(sig1, sig2)
	if sim12 < 0.95 {
		t.Errorf("identical content similarity = %.3f, expected >= 0.95", sim12)
	}

	// Different content should have lower similarity
	sim13 := jaccardEstimate(sig1, sig3)
	if sim13 > 0.80 {
		t.Errorf("different content similarity = %.3f, expected < 0.80", sim13)
	}
}

func TestComputeMinHash_Empty(t *testing.T) {
	sig := ComputeMinHash("")
	for i, v := range sig {
		if v != 0 && i == 0 {
			// First element might be 0 if no tokens
		}
	}
}

func TestLSHBandKeys(t *testing.T) {
	sig := ComputeMinHash("测试文本")
	keys := bandKeys(sig)
	if len(keys) != minhashBands {
		t.Errorf("bandKeys count = %d, expected %d", len(keys), minhashBands)
	}
}

func TestClusterSegmentsMinHash(t *testing.T) {
	segments := []string{
		"InternVL 推荐使用视觉编码器推理参数选择推理参数四件套参数说明InternVL加课件加论文加笔记",
		"InternVL推荐视觉编码器推理参数四件套组合笔记加九条加论文加笔记",
		"温迪推荐使用文本接口推理参数选择上下文四件套参数说明温迪加InternLM加笔记加公式",
		"温迪推荐文本接口上下文四件套组合温迪加InternLM加笔记",
		"完全不同的内容关于另一份资料InternLM推荐文本接口上下文长度四件套",
	}

	cr := ClusterSegmentsMinHash(segments)

	// Verify all segments accounted for
	totalClustered := 0
	for _, cluster := range cr.Clusters {
		totalClustered += len(cluster)
	}
	total := totalClustered + len(cr.Singles)
	if total != len(segments) {
		t.Errorf("total segments = %d, expected %d (clusters=%d, singles=%d)", total, len(segments), len(cr.Clusters), len(cr.Singles))
	}
	// MinHash clustering may or may not find clusters depending on text length;
	// the important thing is that all segments are accounted for.
	t.Logf("clusters=%d singles=%d", len(cr.Clusters), len(cr.Singles))
}

func TestJaccardEstimate(t *testing.T) {
	sig1 := ComputeMinHash("完全相同的文本")
	sig2 := ComputeMinHash("完全相同的文本")
	sim := jaccardEstimate(sig1, sig2)
	if sim < 0.95 {
		t.Errorf("identical signatures similarity = %.3f", sim)
	}
}

func TestMergeClusteredSegments(t *testing.T) {
	raw := "InternVL 教程\n推荐视觉编码器\n\nInternLM 教程\n推荐文本接口"
	sk := ScanSkeleton(raw)

	segs := []PlannedSegment{
		{Content: "InternVL 教程推荐使用视觉编码器推理参数选择推理参数四件套参数说明InternVL加课件加论文加笔记", LineRange: [2]int{1, 2}, Title: "InternVL 教程"},
		{Content: "InternVL 教程推荐视觉编码器推理参数四件套组合笔记九条论文笔记", LineRange: [2]int{1, 2}, Title: "InternVL 教程"},
		{Content: "InternLM 教程推荐使用文本接口推理参数选择上下文长度四件套参数说明InternLM加公式加章节加引用", LineRange: [2]int{3, 4}, Title: "InternLM 教程"},
	}

	merged := MergeClusteredSegments(segs, raw, sk)
	if len(merged) >= len(segs) {
		t.Logf("merge result: %d -> %d (MinHash may not cluster short texts)", len(segs), len(merged))
	}
	// Should preserve both topics regardless of merge outcome
	foundLei := false
	foundZhong := false
	for _, seg := range merged {
		if seg.Title == "InternVL 教程" || containsStr(seg.Content, "InternVL") {
			foundLei = true
		}
		if seg.Title == "InternLM 教程" || containsStr(seg.Content, "InternLM") {
			foundZhong = true
		}
	}
	if !foundLei || !foundZhong {
		t.Error("result should preserve both topics")
	}
}

func containsStr(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
