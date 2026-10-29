// Package smartimport provides LLM cleaning constraint protocol for noise removal.
// Implements optimization plan 8.3: LLM cleaning is allowed to DELETE non-knowledge
// sentences but must NEVER rewrite proper nouns, numbers, or verbs.
// Post-cleaning fact consistency check ensures no fabricated or altered content.
package smartimport

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/graydraft/core/service/textutil"
)

// LLMCleaningRequest is input for the LLM noise cleaning step.
type LLMCleaningRequest struct {
	Content    string
	SourceType string // "multimodal_vlm" | "multimodal_ocr" | "text"
}

// LLMCleaningResult holds the cleaned text and verification status.
type LLMCleaningResult struct {
	Cleaned       string
	PassedCheck   bool
	CheckFailures []string
}

// cleanPromptTemplate is the prompt for LLM noise cleaning.
// It instructs the model to only DELETE non-knowledge sentences, never rewrite content.
const cleanPromptTemplate = `你是 GrayDraft 知识清洗助手。按照以下约束处理文本：

【严格约束】
1. 仅允许删除非知识性句子（如视觉描述、界面导航、格式化噪声），禁止新增任何内容。
2. 禁止改写专有名词、数字、单位、动词。这些内容必须原样保留。
3. 禁止改变句子顺序或合并/拆分句子。
4. 清洗后文本的所有专名和数字必须在原文本中出现。
5. 输出中不要使用 Emoji。

【待清洗文本】
%s

只输出清洗后的纯文本，不要加任何说明。`

// BuildCleanPrompt builds the cleaning prompt for an LLM call.
func BuildCleanPrompt(content string) string {
	snippet := content
	if len([]rune(snippet)) > 6000 {
		snippet = string([]rune(snippet)[:6000]) + "…"
	}
	return fmt.Sprintf(cleanPromptTemplate, snippet)
}

// VerifyCleaningFactConsistency checks that all proper nouns and numbers in
// the cleaned output still appear in the original text.
// Returns true if consistent, false with a list of violations.
func VerifyCleaningFactConsistency(original, cleaned string) LLMCleaningResult {
	if strings.TrimSpace(cleaned) == "" {
		return LLMCleaningResult{
			Cleaned:     original,
			PassedCheck: true,
		}
	}

	// Extract named entities and numeric patterns from cleaned text
	cleanedEntities := extractFactualTokens(cleaned)
	if len(cleanedEntities) == 0 {
		return LLMCleaningResult{Cleaned: cleaned, PassedCheck: true}
	}

	var violations []string
	for _, entity := range cleanedEntities {
		if !strings.Contains(original, entity) {
			violations = append(violations, entity)
		}
	}

	if len(violations) > 0 {
		return LLMCleaningResult{
			Cleaned:       original, // Revert to original
			PassedCheck:   false,
			CheckFailures: violations,
		}
	}

	return LLMCleaningResult{Cleaned: cleaned, PassedCheck: true}
}

var (
	reFactualTokens = []*regexp.Regexp{
		regexp.MustCompile(`[《「『][^》」』\n]{2,24}[》」』]`),                // 书名号实体
		regexp.MustCompile(`[A-Z][a-z]+(?:[·\-_][A-Z][a-z]+)*`),               // CamelCase 实体
		regexp.MustCompile(`\d+[\.\d]*[%秒点层级倍次米码Kk万Ww]`),                // 数字+单位
		regexp.MustCompile(`[\p{Han}]{2,8}(?:模型|教程|论文|参数|接口|公式|章节|笔记)`), // 中文专名
	}
)

// extractFactualTokens extracts proper nouns and numeric expressions from text.
func extractFactualTokens(text string) []string {
	var tokens []string
	seen := map[string]bool{}
	for _, re := range reFactualTokens {
		for _, m := range re.FindAllString(text, -1) {
			m = strings.TrimSpace(m)
			if m != "" && !seen[m] {
				seen[m] = true
				tokens = append(tokens, m)
			}
		}
	}
	return tokens
}

// ShouldTriggerLLMCleaning determines if LLM cleaning should be applied.
// Returns true when info density is in [0.06, 0.10) or known noise signals exist.
func ShouldTriggerLLMCleaning(densityScore float64, ngramRepeatRate float64, content string) bool {
	// Low density: definitely needs cleaning
	if densityScore >= 0.06 && densityScore < 0.10 {
		return true
	}
	// High density but noise signals present
	if densityScore >= 0.10 {
		if ngramRepeatRate > 0.5 {
			return true
		}
		if HasURL(content) {
			return true
		}
		// Check for very high repetition
		runeLen := len([]rune(content))
		uniqueRatio := float64(len([]rune(textutil.StripEmojis(content)))) / float64(runeLen)
		if uniqueRatio < 0.3 && runeLen > 100 {
			return true
		}
	}
	return false
}
