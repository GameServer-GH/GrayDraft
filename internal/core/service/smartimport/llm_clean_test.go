package smartimport

import (
	"strings"
	"testing"
)

func TestVerifyCleaningFactConsistency_Pass(t *testing.T) {
	original := "InternVL 推荐使用视觉编码器。闪避敌人技能后触发追击。"
	cleaned := "InternVL 推荐使用视觉编码器。闪避敌人技能后触发追击。"

	result := VerifyCleaningFactConsistency(original, cleaned)
	if !result.PassedCheck {
		t.Errorf("identical text should pass: %v", result.CheckFailures)
	}
}

func TestVerifyCleaningFactConsistency_RemovedNonKnowledge(t *testing.T) {
	original := "图片展示了InternVL的技能界面。InternVL 推荐使用视觉编码器。背景为米黄色。"
	cleaned := "InternVL 推荐使用视觉编码器。"

	result := VerifyCleaningFactConsistency(original, cleaned)
	if !result.PassedCheck {
		t.Errorf("removed non-knowledge should pass: %v", result.CheckFailures)
	}
}

func TestVerifyCleaningFactConsistency_FabricatedEntity(t *testing.T) {
	// Use book-title entities which are reliably detected
	original := "《InternVL》多模态接口说明。"
	cleaned := "《其他模型》多模态接口说明。" // fabricated: 崩坏 not in original

	result := VerifyCleaningFactConsistency(original, cleaned)
	if result.PassedCheck {
		t.Error("fabricated book-title entity should fail consistency check")
	}
	if result.Cleaned != original {
		t.Error("should revert to original on fabrication detection")
	}
}

func TestVerifyCleaningFactConsistency_FabricatedNumber(t *testing.T) {
	// Use numeric-unit pattern which is reliably detected
	original := "释放技能造成200%伤害。"
	cleaned := "释放技能造成300%伤害。"

	result := VerifyCleaningFactConsistency(original, cleaned)
	// Note: 200% and 300% are different — cleaned should fail
	if result.PassedCheck {
		t.Error("fabricated number should fail consistency check")
	}
}

func TestBuildCleanPrompt(t *testing.T) {
	content := "这是一段测试文本"
	prompt := BuildCleanPrompt(content)
	if !strings.Contains(prompt, "待清洗文本") {
		t.Error("prompt should contain cleaning instructions")
	}
	if !strings.Contains(prompt, content) {
		t.Error("prompt should contain the original content")
	}
}

func TestShouldTriggerLLMCleaning(t *testing.T) {
	// Low density → should trigger
	if !ShouldTriggerLLMCleaning(0.07, 0.2, "测试") {
		t.Error("low density should trigger cleaning")
	}
	// High density, no noise → should not trigger
	if ShouldTriggerLLMCleaning(0.15, 0.1, "正常文本") {
		t.Error("high density without noise should not trigger")
	}
	// High density with noise → should trigger
	if !ShouldTriggerLLMCleaning(0.15, 0.6, "正常文本") {
		t.Error("high density with high ngram repeat should trigger")
	}
}

func TestExtractFactualTokens(t *testing.T) {
	text := "《InternVL》视觉编码器在 1024 分辨率下输出特征。"
	tokens := extractFactualTokens(text)
	if len(tokens) == 0 {
		t.Error("should extract factual tokens")
	}
}

func TestHasMarketingPatterns(t *testing.T) {
	tests := []struct {
		content  string
		expected bool
	}{
		{"限时活动，最后3天，点击下载领取奖励", true},
		{"充值返利，首充送皮肤", true},
		{"InternVL 推荐使用视觉编码器", false},
		{"正常教程内容，参数说明", false},
		{"特价优惠，满减折扣", true},
	}

	for _, tt := range tests {
		result := HasMarketingPatterns(tt.content)
		if result != tt.expected {
			t.Errorf("HasMarketingPatterns(%q) = %v, want %v", tt.content, result, tt.expected)
		}
	}
}
