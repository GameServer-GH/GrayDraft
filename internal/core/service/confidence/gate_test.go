package confidence

import (
	"testing"

	"github.com/graydraft/core/model"
)

func TestTitleHit_inTitle(t *testing.T) {
	if !TitleHit("巴黎旅游注意事项", "浪漫之都避坑指南", map[string]any{"search_anchors": []any{"巴黎", "法国"}}) {
		t.Fatal("expected anchor match")
	}
}

func TestCalculateFinalConfidence_titleBonus(t *testing.T) {
	chunks := []model.Chunk{{
		RerankScore:  0.2,
		VectorScore:  0.05,
		TitleMatched: true,
	}}
	got := CalculateFinalConfidence("书生多模态技能", chunks)
	if got < 0.25 {
		t.Fatalf("expected pass with title bonus, got %v", got)
	}
}

func TestPassesRetrievalGate_factThreshold(t *testing.T) {
	chunks := []model.Chunk{{
		RerankScore:  0.35,
		VectorScore:  0.12,
		QualityScore: 0.78,
		Content:      "| 主题 | 时间/区间 |\n| 小夜 | 06.19 - 06.21 | 版本发布：小夜 |",
	}}
	got := CalculateFinalConfidenceWithIntent("小夜何时出没", "fact", chunks)
	if got < 0.20 {
		t.Fatalf("expected fact table boost, got %v", got)
	}
	if !PassesRetrievalGateWithIntent("小夜何时出没", "fact", chunks) {
		t.Fatal("expected pass under fact intent threshold")
	}
}

func TestPassesRetrievalGate(t *testing.T) {
	if PassesRetrievalGate("q", nil) {
		t.Fatal("empty should fail")
	}
	if !PassesRetrievalGate("q", []model.Chunk{{RerankScore: 0.5, VectorScore: 0.2}}) {
		t.Fatal("expected pass")
	}
}

func TestPassesRetrievalGate_directEvidenceInContent(t *testing.T) {
	chunks := []model.Chunk{{
		Score:        0.08,
		QualityScore: 0.71,
		DocTitle:     "食谱分享|一周减脂餐",
		Content:      "周一\n- 早餐：水煮蛋2个、小番茄100g\n周二\n- 晚餐：番茄金针菇汤（番茄100g）",
		Extensions:   map[string]any{"search_anchors": []any{"一周减脂餐"}},
	}}
	if !PassesRetrievalGateWithIntent("番茄周几吃，一次吃多少", "fact", chunks) {
		t.Fatal("expected direct content evidence to pass kb_only gate")
	}
}

func TestRetrievalGateThreshold(t *testing.T) {
	if RetrievalGateThreshold("fact") != gateThresholdFact {
		t.Fatal("fact threshold")
	}
	if RetrievalGateThreshold("concept") != gateThresholdConcept {
		t.Fatal("concept threshold")
	}
}

func TestRetrievalGateThresholdDynamic_qualityRelax(t *testing.T) {
	chunks := []model.Chunk{{QualityScore: 0.82, TitleMatched: true}}
	th := RetrievalGateThresholdDynamic("concept", chunks)
	if th >= gateThresholdConcept {
		t.Fatalf("expected relaxed threshold, got %v", th)
	}
}
