package smartimport

import (
	"testing"
)

func TestEvaluateConfidence_Pass(t *testing.T) {
	sk := DocumentSkeleton{
		Lines: []SkeletonLine{
			{LineNo: 1, Kind: "h1", Text: "InternVL 教程"},
			{LineNo: 2, Kind: "text"},
		},
	}
	content := "InternVL 教程：推荐使用视觉编码器，推理参数选择推理参数4件套。参数说明：InternVL+课件+论文+笔记。"
	title := "InternVL 教程"

	result := EvaluateConfidence(sk, content, title, 0.8, 0.1)
	if result.Label == LabelReject {
		t.Errorf("expected not Reject, got %s: %s (fails: %v)", result.Label, result.Explain, result.FailedChecks)
	}
	if !result.StructureOk {
		t.Error("structure should be ok with h1 heading")
	}
	if !result.TitleOk {
		t.Error("title should appear in content as substring")
	}
}

func TestEvaluateConfidence_Reject_NoEntity(t *testing.T) {
	sk := DocumentSkeleton{
		Lines: []SkeletonLine{
			{LineNo: 1, Kind: "text"},
		},
	}
	// Truly content-free noise: only repeat chars + punctuation
	content := "。。。。。。。。。，，，，，，，，，"
	title := ""

	result := EvaluateConfidence(sk, content, title, 0.0, 0.1)
	if result.Label == LabelPass {
		t.Errorf("expected not Pass, got %s: %s", result.Label, result.Explain)
	}
}

func TestEvaluateConfidence_ShortExemption(t *testing.T) {
	sk := DocumentSkeleton{
		Lines: []SkeletonLine{
			{LineNo: 1, Kind: "h1", Text: "InternVL"},
		},
	}
	// "InternVL" is 4 runes < 80, and reNamedEnt should match it
	content := "InternVL"
	title := "InternVL"

	result := EvaluateConfidence(sk, content, title, 0.5, 0.0)
	// Short exemption should trigger if runeLen < 80, entityCount >=1, TitleOk, EntityTraceOk
	if result.ShortExempt {
		if result.Label != LabelWarning {
			t.Errorf("short exempt should be Warning, got %s", result.Label)
		}
	}
	// Even without short exemption, this should not be Reject
	if result.Label == LabelReject {
		t.Errorf("should not Reject short content with entity: %s", result.Explain)
	}
}

func TestEvaluateConfidence_Warning(t *testing.T) {
	sk := DocumentSkeleton{
		Lines: []SkeletonLine{
			{LineNo: 1, Kind: "h1", Text: "教程"},
			{LineNo: 2, Kind: "text"},
		},
	}
	content := "这是一个简单的教程说明，推荐核对参数。"
	title := "教程"

	result := EvaluateConfidence(sk, content, title, 0.5, 0.1)
	if result.Label != LabelWarning && result.Label != LabelPass {
		t.Errorf("expected Warning or Pass, got %s: %s", result.Label, result.Explain)
	}
}

func TestEvaluateConfidence_Reject_HighNoise(t *testing.T) {
	sk := DocumentSkeleton{
		Lines: []SkeletonLine{
			{LineNo: 1, Kind: "h1", Text: "测试"},
		},
	}
	content := "测试测试测试测试测试测试测试"
	title := "测试"

	result := EvaluateConfidence(sk, content, title, 0.8, 0.8) // high ngram repeat
	if result.Label == LabelPass {
		t.Errorf("high noise should not Pass, got %s", result.Label)
	}
}

func TestLevenshteinDistance(t *testing.T) {
	tests := []struct {
		a, b string
		dist int
	}{
		{"InternVL", "InternVL", 0},
		{"InternVL", "InternLM", 2},
		{"参数说明", "教程参数说明", 2},
		{"", "测试", 2},
		{"测试", "", 2},
	}
	for _, tt := range tests {
		d := levenshteinDistance([]rune(tt.a), []rune(tt.b))
		if d != tt.dist {
			t.Errorf("levenshtein(%q, %q) = %d, want %d", tt.a, tt.b, d, tt.dist)
		}
	}
}

func TestLCSSimilarity(t *testing.T) {
	// LCS should recognize "参数说明" within "教程参数说明"
	a := []rune("参数说明")
	b := []rune("教程参数说明")
	lcs := lcsLength(a, b)
	ratio := float64(lcs) / float64(len(a))
	if ratio < 0.70 {
		t.Errorf("LCS ratio = %.2f, expected >= 0.70", ratio)
	}
}

func TestNgramRepeatRate(t *testing.T) {
	// Normal text
	normal := NgramRepeatRate("InternVL 推荐使用视觉编码器")
	if normal > 0.3 {
		t.Errorf("normal text ngram repeat = %.3f, expected < 0.3", normal)
	}
	// Repetitive text
	repetitive := NgramRepeatRate("测试测试测试测试测试测试测试测试")
	if repetitive < 0.5 {
		t.Errorf("repetitive text ngram repeat = %.3f, expected >= 0.5", repetitive)
	}
}

func TestTitleEditDistanceMatch(t *testing.T) {
	// Edit distance checks whole-line similarity — "参数说明" should match a line like "参数说明"
	content := "参数说明\nInternVL组合"
	if !titleEditDistanceMatch("参数说明", content) {
		t.Error("'参数说明' should match identical line via edit distance")
	}
	// "参数说明" vs long line "教程参数说明：温迪+InternLM+笔记" has large edit distance
	// so edit distance won't match — that's expected, LCS handles substring cases
	if titleEditDistanceMatch("完全不存在的标题xyz", content) {
		t.Error("non-existent title should not match")
	}
}

func TestTitleLCSMatch(t *testing.T) {
	content := "教程参数说明\nInternVL组合"
	if !titleLCSMatch("参数说明", content) {
		t.Error("'参数说明' should match via LCS")
	}
}
