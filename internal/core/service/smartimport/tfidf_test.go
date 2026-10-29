package smartimport

import (
	"testing"
)

func TestNewDocumentCorpus(t *testing.T) {
	docs := []string{
		"InternVL 推荐使用视觉编码器，推理参数选择推理参数",
		"温迪推荐使用文本接口，推理参数选择上下文",
		"InternLM推荐使用文本接口，推理参数选择上下文长度",
	}
	dc := NewDocumentCorpus(docs)
	if dc.totalDocs != 3 {
		t.Errorf("totalDocs = %d, want 3", dc.totalDocs)
	}
}

func TestTFIDF(t *testing.T) {
	docs := []string{
		"InternVL 推荐使用视觉编码器推理参数选择推理参数四件套",
		"InternVL 教程推荐视觉编码器推理参数参数说明",
		"温迪推荐使用文本接口推理参数选择上下文四件套",
		"InternLM推荐使用文本接口推理参数选择上下文长度四件套",
	}
	dc := NewDocumentCorpus(docs)
	scores := dc.TFIDF(10)

	if len(scores) == 0 {
		t.Error("TF-IDF should produce scores for a non-trivial corpus")
	}
	// Verify scores are sorted by IDF descending
	for i := 1; i < len(scores); i++ {
		if scores[i].IDF > scores[i-1].IDF {
			t.Errorf("scores not sorted by IDF: [%d]=%.2f > [%d]=%.2f", i, scores[i].IDF, i-1, scores[i-1].IDF)
		}
	}
}

func TestTFIDF_Empty(t *testing.T) {
	dc := NewDocumentCorpus(nil)
	scores := dc.TFIDF(10)
	if len(scores) != 0 {
		t.Error("empty corpus should produce no scores")
	}
}

func TestTFIDFDomainTermProvider(t *testing.T) {
	docs := []string{
		"InternVL 推荐使用视觉编码器推理参数",
		"温迪推荐文本接口上下文",
	}
	provider := NewTFIDFProvider(docs, 5, defaultTermProvider{})
	kw := provider.DomainKeywords()
	if len(kw) == 0 {
		t.Error("TF-IDF provider should return keywords")
	}
}

func TestFilterMeaningfulTerms(t *testing.T) {
	terms := []TermScore{
		{Term: "视觉", Weight: 0.8},
		{Term: "ab", Weight: 0.5},
		{Term: "测试", Weight: 0.6},
		{Term: "12", Weight: 0.4},
	}
	filtered := FilterMeaningfulTerms(terms)
	for _, ft := range filtered {
		if ft.Term == "ab" || ft.Term == "12" {
			t.Errorf("non-CJK term %q should be filtered out", ft.Term)
		}
	}
}

func TestExtractDomainTermsFromDocs(t *testing.T) {
	docs := []string{
		"InternVL参数说明课件论文笔记",
		"InternVL 视觉编码器与推理参数",
		"温迪组合InternLM笔记公式",
	}
	terms := ExtractDomainTermsFromDocs(docs, 5)
	if len(terms) == 0 {
		t.Error("should extract domain terms")
	}
}
