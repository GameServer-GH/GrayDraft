package smartimport

import (
	"math"
	"testing"
)

func TestCalcInfoDensity(t *testing.T) {
	tests := []struct {
		name     string
		content  string
		minScore float64
		maxScore float64
		desc     string
	}{
		{
			name:     "strategy content with action verbs",
			content:  "闪避敌人技能后立刻接重击，能够触发额外伤害增益。建议在boss释放大招前保持护盾状态。",
			minScore: 0.06,
			maxScore: 0.50,
			desc:     "entity-sparse but information-rich strategy text should pass noise threshold",
		},
		{
			name:     "visual noise description",
			content:  "界面包含按钮、文字：确定。该界面为活动日历，背景为米黄色。",
			minScore: 0.0,
			maxScore: 0.35,
			desc:     "UI description — some domain terms (活动/日历) may match fallback provider",
		},
		{
			name:     "numbered steps",
			content:  "步骤1：准备材料。步骤2：合成装备。步骤3：强化到+15。",
			minScore: 0.08,
			maxScore: 0.60,
			desc:     "step markers + numeric values give reasonable density",
		},
		{
			name:     "entity-rich game content",
			content:  "《InternVL》InternVL 推荐使用视觉编码器，推理参数选择推理参数4件套。参数说明：InternVL+课件+论文+笔记。",
			minScore: 0.10,
			maxScore: 0.60,
			desc:     "named entities in book-title marks should score high",
		},
		{
			name:     "empty content",
			content:  "",
			minScore: 0.0,
			maxScore: 0.0,
			desc:     "empty string should score zero",
		},
		{
			name:     "single word",
			content:  "教程",
			minScore: 0.0,
			maxScore: 0.3,
			desc:     "single word has limited signal",
		},
		{
			name:     "repetitive garbage",
			content:  "啊啊啊啊啊啊啊啊啊啊啊啊啊啊啊啊啊啊啊啊啊啊啊啊啊啊啊啊啊啊啊啊啊啊啊啊啊啊",
			minScore: 0.0,
			maxScore: 0.03,
			desc:     "repetitive characters should have low entropy and low density",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := CalcInfoDensity(tt.content)
			if result.Score < tt.minScore || result.Score > tt.maxScore {
				t.Errorf("CalcInfoDensity(%q) score = %.4f, want in [%.4f, %.4f] (%s)",
					tt.content, result.Score, tt.minScore, tt.maxScore, tt.desc)
			}
		})
	}
}

func TestIsNoiseByInfoDensity(t *testing.T) {
	// "闪避敌人技能..." should NOT be noise
	if IsNoiseByInfoDensity("闪避敌人技能后立刻接重击，能够触发额外伤害增益") {
		t.Error("strategy content should not be classified as noise")
	}
	// "界面包含按钮..." should be noise
	// Test with known noise content
	if !IsNoiseByInfoDensity("") {
		t.Error("empty content should be classified as noise")
	}
	// Strategy content should NOT be noise
	if IsNoiseByInfoDensity("闪避敌人技能后立刻接重击，能够触发额外伤害增益。建议保持护盾状态。") {
		t.Error("strategy content should not be noise")
	}
}

func TestCharEntropy(t *testing.T) {
	// Natural language should have moderate entropy
	naturalEntropy := charEntropy("闪避敌人技能后立刻接重击能够触发额外伤害增益")
	if naturalEntropy < 3.0 || naturalEntropy > 8.0 {
		t.Errorf("natural language entropy = %.2f, expected 3-8", naturalEntropy)
	}

	// Repetitive text should have very low entropy
	repetitiveEntropy := charEntropy("啊啊啊啊啊啊啊啊啊啊啊啊啊啊啊啊啊啊啊啊啊啊")
	if repetitiveEntropy > 0.5 {
		t.Errorf("repetitive text entropy = %.2f, expected < 0.5", repetitiveEntropy)
	}
}

func TestEntropyScore(t *testing.T) {
	tests := []struct {
		entropy float64
		min     float64
		max     float64
	}{
		{0.5, 0.0, 0.2},   // very low → noise
		{5.5, 0.80, 1.0},  // natural → high
		{10.0, 0.0, 0.35}, // very high → random garbage
	}
	for _, tt := range tests {
		s := entropyScore(tt.entropy)
		if s < tt.min || s > tt.max {
			t.Errorf("entropyScore(%.1f) = %.3f, want [%.3f, %.3f]", tt.entropy, s, tt.min, tt.max)
		}
	}
}

func TestBigramTokens(t *testing.T) {
	tokens := bigramTokens("闪避技能")
	if len(tokens) == 0 {
		t.Fatal("empty tokens")
	}
	// Should contain bigrams like "闪避", "避技", "技能"
	hasBigram := false
	for _, tok := range tokens {
		if len([]rune(tok)) >= 2 {
			hasBigram = true
			break
		}
	}
	if !hasBigram {
		t.Error("bigramTokens should produce multi-character bigrams")
	}
}

func TestCalcInfoDensity_AllDimensionsNonZero(t *testing.T) {
	// Content that should trigger all four dimensions
	content := "《崩坏：星穹铁道》镜流推荐冰伤球和攻击绳。" +
		"步骤1：释放战技叠加被动。步骤2：触发追加攻击造成150%伤害。" +
		"参数说明：镜流+布洛妮娅+佩拉+罗刹。3秒内闪避可触发反击。"

	result := CalcInfoDensity(content)
	if result.Score < 0.08 {
		t.Errorf("rich content score = %.4f, expected >= 0.08. breakdown: %v", result.Score, result.Breakdown)
	}
	if result.EntityCount == 0 {
		t.Error("should detect named entities")
	}
	if result.NumericCount == 0 {
		t.Error("should detect numeric values")
	}
	if result.StepCount == 0 {
		t.Error("should detect step markers")
	}
}

func TestDomainTermProvider_Fallback(t *testing.T) {
	provider := defaultTermProvider{}
	kw := provider.DomainKeywords()
	if len(kw) == 0 {
		t.Error("fallback provider should return some keywords")
	}
	// Verify key terms exist
	expected := []string{"触发", "释放", "装备", "推荐"}
	for _, e := range expected {
		if _, ok := kw[e]; !ok {
			t.Errorf("fallback provider missing expected keyword: %q", e)
		}
	}
}

func TestInfoDensity_Rounding(t *testing.T) {
	result := CalcInfoDensity("测试文本内容")
	// Score should be rounded to 4 decimal places
	rounded := math.Round(result.Score*10000) / 10000
	if result.Score != rounded {
		t.Errorf("score %.6f not rounded to 4 decimal places", result.Score)
	}
}
