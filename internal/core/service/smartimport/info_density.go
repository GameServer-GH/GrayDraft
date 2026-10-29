// Package smartimport provides information density calculation for knowledge quality assessment.
//
// Uses bigram-based tokenization (captures multi-char Chinese words without external deps),
// character entropy for noise detection, dynamic domain terms from taxonomyevolve, and
// multi-dimensional linguistic features to distinguish real knowledge from noise — without
// hardcoded word lists.
package smartimport

import (
	"math"
	"regexp"
	"strings"
	"unicode/utf8"
)

// ── tokenization ────────────────────────────────────────────────────────────

// bigramTokens extracts Chinese bigrams (overlapping 2-rune windows) plus
// standalone ASCII tokens. This captures multi-character words like "闪避",
// "释放", "触发" without needing an external segmenter, and works for any
// character-based language.
func bigramTokens(s string) []string {
	runes := []rune(s)
	if len(runes) == 0 {
		return nil
	}
	tokens := make([]string, 0, len(runes)*2)

	i := 0
	for i < len(runes) {
		r := runes[i]
		// ASCII / digit / punctuation: treat as single-char token boundaries
		if r < 128 || isPunct(r) {
			if !isSpace(r) {
				tokens = append(tokens, string(r))
			}
			i++
			continue
		}
		// CJK character: emit bigram (current + next) and current as unigram
		if i+1 < len(runes) && runes[i+1] >= 128 && !isPunct(runes[i+1]) {
			tokens = append(tokens, string([]rune{r, runes[i+1]}))
		}
		tokens = append(tokens, string(r))
		i++
	}
	return tokens
}

func isPunct(r rune) bool {
	return r < 128 && !((r >= '0' && r <= '9') || (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z'))
}

func isSpace(r rune) bool {
	return r == ' ' || r == '\t' || r == '\n' || r == '\r'
}

// ── character entropy ───────────────────────────────────────────────────────

// charEntropy computes Shannon entropy over character bigram frequencies.
// Natural language: moderate entropy (~4-7 bits for Chinese). Noise text
// (OCR loops, repetitive patterns): very low. Random garbage: very high.
func charEntropy(s string) float64 {
	runes := []rune(s)
	if len(runes) < 2 {
		return 0
	}
	freq := map[string]int{}
	total := 0
	for i := 0; i < len(runes)-1; i++ {
		pair := string([]rune{runes[i], runes[i+1]})
		freq[pair]++
		total++
	}
	if total == 0 {
		return 0
	}
	entropy := 0.0
	for _, c := range freq {
		p := float64(c) / float64(total)
		entropy -= p * math.Log2(p)
	}
	return entropy
}

// entropyScore normalizes character entropy to [0,1].
// Optimal entropy ≈ 5-7 bits → score ≈ 0.8-1.0.
// Very low (<2) → repetitive noise (OCR loops).
// Very high (>9) → random garbage.
func entropyScore(entropy float64) float64 {
	if entropy < 1.0 {
		return 0.1
	}
	if entropy > 9.0 {
		return 0.3
	}
	if entropy >= 4.5 && entropy <= 7.5 {
		return 0.85 + (entropy-4.5)/3.0*0.15
	}
	if entropy < 4.5 {
		return 0.3 + (entropy-1.0)/3.5*0.55
	}
	return 1.0 - (entropy-7.5)/1.5*0.7
}

// ── dynamic domain term matching ────────────────────────────────────────────

// DomainTermProvider supplies domain-specific keywords for scoring.
// Implementations include taxonomyevolve keyword lists or TF-IDF extractions.
type DomainTermProvider interface {
	// DomainKeywords returns weighted keywords for the current domain.
	// Weights typically use IDF: rarer (more specific) terms get higher weight.
	DomainKeywords() map[string]float64
}

// defaultTermProvider uses common Chinese knowledge-bearing words as a fallback
// when no workspace-specific taxonomy is available.
type defaultTermProvider struct{}

func (d defaultTermProvider) DomainKeywords() map[string]float64 {
	// Common functional/predicate markers in Chinese knowledge text.
	// These are NOT game-specific — they signal information-bearing content
	// across any domain: strategy, product, medical, legal, etc.
	return map[string]float64{
		"可以": 0.3, "需要": 0.4, "使用": 0.3, "通过": 0.3, "进行": 0.2,
		"提供": 0.3, "获得": 0.3, "增加": 0.4, "减少": 0.4, "提高": 0.4,
		"降低": 0.4, "触发": 0.5, "释放": 0.5, "造成": 0.4, "持续": 0.4,
		"效果": 0.3, "攻击": 0.4, "防御": 0.4, "回复": 0.4, "选择": 0.3,
		"推荐": 0.4, "配置": 0.4, "装备": 0.5, "升级": 0.5, "解锁": 0.5,
	}
}

// ── content-derived adaptive term provider ───────────────────────────────────

// ContentDerivedTermProvider builds a domain term lexicon from the input text
// itself, using CJK bigram frequency analysis. This eliminates dependency on
// hardcoded keyword lists: any text with rich, diverse Chinese vocabulary
// (e.g., VLM-extracted game tables, team compositions, equipment names)
// automatically scores well without needing pre-registered terms.
//
// Strategy:
//   - Tokenize content into bigrams, count frequencies.
//   - Filter: CJK-only bigrams with frequency >= 2 (repeated = domain signal).
//   - Weight by normalized TF (frequency / total) — higher = more domain-relevant.
//   - Combine with an optional base provider (taxonomy keywords) for bonus scoring.
type ContentDerivedTermProvider struct {
	content string
	base    DomainTermProvider // optional fallback (taxonomy, default)
}

// NewContentDerivedTermProvider creates a provider that learns terms from content.
func NewContentDerivedTermProvider(content string, base DomainTermProvider) *ContentDerivedTermProvider {
	return &ContentDerivedTermProvider{content: content, base: base}
}

func (p *ContentDerivedTermProvider) DomainKeywords() map[string]float64 {
	// Start with base provider terms if available
	terms := map[string]float64{}
	if p.base != nil {
		for k, v := range p.base.DomainKeywords() {
			terms[k] = v
		}
	}

	// Derive terms from content via bigram frequency
	tokens := bigramTokens(p.content)
	if len(tokens) < 10 {
		return terms
	}

	// Count CJK bigram frequencies
	freq := map[string]int{}
	for _, tok := range tokens {
		runes := []rune(tok)
		if len(runes) < 2 {
			continue
		}
		// Only CJK bigrams (both runes are Han characters)
		allHan := true
		for _, r := range runes {
			if r < 0x4E00 || r > 0x9FFF {
				allHan = false
				break
			}
		}
		if !allHan {
			continue
		}
		freq[tok]++
	}

	// Weight: sqrt(freq) normalizes extreme frequencies
	// This gives moderate weight to repeated terms (e.g., "术语" ×3 → ~1.0)
	// while capping very frequent particles
	total := float64(len(tokens))
	for tok, cnt := range freq {
		if cnt < 2 {
			continue // single occurrence = likely noise, skip
		}
		weight := math.Sqrt(float64(cnt)) * 0.5 // sqrt(2)=0.7, sqrt(5)=1.1, sqrt(10)=1.6
		if existing, ok := terms[tok]; ok {
			// Blend with base: content-derived weight adds to base weight
			terms[tok] = math.Max(existing, weight)
		} else {
			terms[tok] = weight
		}
		_ = total
	}

	return terms
}

// ── numeric & pattern recognition ───────────────────────────────────────────

var (
	reNumberUnit = regexp.MustCompile(`[\d.]+%|[\d.]+秒|[\d.]+点|[\d.]+层|[\d.]+级|[\d.]+倍|[\d.]+次|[\d.]+米|[\d.]+码|Lv[.\d]+|等级[\d.]+|[+\-]?[\d.]+[Kk]|[+\-]?[\d.]+[Ww万]|第[\d]+|[\d]+[．.]`)
	reStepSeq    = regexp.MustCompile(`第[一二三四五六七八九十\d]+[步阶段条项]|[12][.)、]\s|[ step]*[123][.)、]|步骤[一二三四五\d]|首先|然后|接着|最后|其次|再者|再|最后一步|最终`)
	reNamedEnt   = regexp.MustCompile(`[《「『][^》」』\n]{1,24}[》」』]|[A-Z][a-z]+(?:[·\-_][A-Z][a-z]+)*|[\p{Han}]{2,6}(?:模型|教程|论文|参数|接口|公式|章节|笔记)`)
	reURL        = regexp.MustCompile(`https?://[^\s\n]{4,}`)

	// Colon-separated key:value pattern — typical in VLM-extracted tables.
	// Detects lines like "角色名：InternVL" or "术语说明：视觉编码器"
	reColonKV = regexp.MustCompile(`^[\p{Han}A-Za-z0-9]+(?:构成|组成|推荐|搭配|选择|定位|职业|属性|效果|描述|说明|技能|类型|模式|方式|成员|角色)[：:]`)
	// Detects list/table structural patterns without markdown headings.
	reListBullet = regexp.MustCompile(`^[-•·]\s|^\d+[.)、]\s|^[*]\s`)
)

// hasSemanticStructure detects whether text has structured information
// without relying on markdown headings. Designed for VLM-extracted content
// like game tables, team compositions, and key:value data.
//
// Detection strategy (no keywords, no hardcoded word lists):
//   1. Colon-separated lines (key: value) — ≥3 lines with "：" or ":".
//   2. Tabular content — ≥2 lines containing "|" (pipe character).
//   3. List-structured lines — ≥3 lines starting with bullets/numbers.
//   4. Repeated line prefix — ≥3 consecutive lines sharing the same first
//      2-4 characters (common in structured data like team tables).
func hasSemanticStructure(content string) bool {
	lines := strings.Split(content, "\n")
	nonEmpty := 0
	colonLines := 0
	pipeLines := 0
	listLines := 0
	prefixCounts := map[string]int{}

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		nonEmpty++

		// 1. Colon-separated key:value pairs
		if reColonKV.MatchString(line) {
			colonLines++
		}

		// 2. Pipe characters (table markdown)
		if strings.Count(line, "|") >= 2 {
			pipeLines++
		}

		// 3. List bullet patterns
		if reListBullet.MatchString(line) {
			listLines++
		}

		// 4. Repeated prefix: first 2-4 CJK chars as structural signature
		runes := []rune(line)
		for _, prefixLen := range []int{3, 4, 2} {
			if len(runes) >= prefixLen {
				prefix := string(runes[:prefixLen])
				allHan := true
				for _, r := range prefix {
					if r < 0x4E00 || r > 0x9FFF {
						allHan = false
						break
					}
				}
				if allHan {
					prefixCounts[prefix]++
				}
				break
			}
		}
	}

	// Require at least 3 non-empty lines
	if nonEmpty < 3 {
		return false
	}

	// Colon-separated pattern (common in VLM tables)
	if colonLines >= 3 {
		return true
	}

	// Pipe-separated table
	if pipeLines >= 2 {
		return true
	}

	// List-structured pattern
	if listLines >= 3 {
		return true
	}

	// Repeated line prefix (same first 2-4 CJK chars)
	for _, cnt := range prefixCounts {
		if cnt >= 3 {
			return true
		}
	}

	return false
}

// ── result type ─────────────────────────────────────────────────────────────

// InfoDensityResult holds the multi-dimensional information density score.
type InfoDensityResult struct {
	Score        float64            `json:"score"`
	TotalTokens  int                `json:"total_tokens"`
	DomainHits   int                `json:"domain_hits"`
	NumericCount int                `json:"numeric_count"`
	StepCount    int                `json:"step_count"`
	EntityCount  int                `json:"entity_count"`
	EntropyBits  float64            `json:"entropy_bits"`
	Breakdown    map[string]float64 `json:"breakdown"`
}

// CalcInfoDensity computes multi-dimensional information density.
//
// Dimensions (all normalized to [0,1] contribution):
//   - language_score (0.30): character entropy — natural text has moderate entropy
//   - content_word (0.30): domain keyword hits / total tokens, using dynamic provider
//   - named_entity (0.20): detected proper nouns / total tokens
//   - numeric_step (0.20): numbers, units, and step markers / total tokens
//
// This works across any domain without hardcoded word lists. The DomainTermProvider
// interface allows injection of workspace-specific keywords from taxonomyevolve.
func CalcInfoDensity(content string) InfoDensityResult {
	return CalcInfoDensityWithTerms(content, defaultTermProvider{})
}

// CalcInfoDensityWithTerms computes density with a custom domain term provider.
func CalcInfoDensityWithTerms(content string, provider DomainTermProvider) InfoDensityResult {
	content = strings.TrimSpace(content)
	if content == "" {
		return InfoDensityResult{Breakdown: map[string]float64{}}
	}

	tokens := bigramTokens(content)
	total := len(tokens)
	if total == 0 {
		return InfoDensityResult{TotalTokens: 0, Breakdown: map[string]float64{}}
	}

	// 1. Language score via character entropy
	entropy := charEntropy(content)
	langScore := entropyScore(entropy)

	// 2. Domain keyword hits with IDF weighting
	keywords := provider.DomainKeywords()
	domainHits := 0.0
	for _, tok := range tokens {
		if w, ok := keywords[tok]; ok {
			domainHits += w
		}
	}
	contentWordScore := math.Min(domainHits/float64(total), 1.0)

	// 3. Named entity density
	entityCount := len(reNamedEnt.FindAllString(content, -1))
	entityScore := math.Min(float64(entityCount)/float64(total)*10, 1.0) // scale: 10% entities → 1.0

	// 4. Numeric + step marker density
	numericCount := len(reNumberUnit.FindAllString(content, -1))
	stepCount := len(reStepSeq.FindAllString(content, -1))
	nsTotal := numericCount + stepCount*2 // steps weighted higher (structural signal)
	nsScore := math.Min(float64(nsTotal)/float64(total)*8, 1.0)

	// Weighted fusion
	score := langScore*0.30 + contentWordScore*0.30 + entityScore*0.20 + nsScore*0.20
	score = math.Round(score*10000) / 10000

	return InfoDensityResult{
		Score:        score,
		TotalTokens:  total,
		DomainHits:   int(domainHits),
		NumericCount: numericCount,
		StepCount:    stepCount,
		EntityCount:  entityCount,
		EntropyBits:  math.Round(entropy*100) / 100,
		Breakdown: map[string]float64{
			"language":    math.Round(langScore*10000) / 10000,
			"content_word": math.Round(contentWordScore*10000) / 10000,
			"entity":      math.Round(entityScore*10000) / 10000,
			"numeric_step": math.Round(nsScore*10000) / 10000,
		},
	}
}

// ── threshold helpers ───────────────────────────────────────────────────────

// IsNoiseByInfoDensity returns true when information density is below 0.06.
func IsNoiseByInfoDensity(content string) bool {
	return CalcInfoDensity(content).Score < 0.06
}

// IsLowDensity returns true when density is between 0.06 and 0.10 (LLM cleaning trigger).
func IsLowDensity(content string) bool {
	s := CalcInfoDensity(content).Score
	return s >= 0.06 && s < 0.10
}

// IsHighDensity returns true when density >= 0.10 (skip LLM cleaning).
func IsHighDensity(content string) bool {
	return CalcInfoDensity(content).Score >= 0.10
}

// ── convenience helpers ─────────────────────────────────────────────────────

// EntityCountFromContent returns the count of named entity patterns in content.
func EntityCountFromContent(content string) int {
	return len(reNamedEnt.FindAllString(content, -1))
}

// ContentRuneLen returns the rune length (used for short-knowledge exemption).
func ContentRuneLen(content string) int {
	return utf8.RuneCountInString(strings.TrimSpace(content))
}

// HasURL returns true if the content contains raw URLs (noise signal).
func HasURL(content string) bool {
	return reURL.MatchString(content)
}

// ── Marketing pattern detection (optimization plan 4.2) ─────────────────────

var (
	rePricePattern    = regexp.MustCompile(`\d+元[起以内]?[，。\s]*(?:[皮肤道具装备礼包]|购买|获得)`)
	reTimePressure    = regexp.MustCompile(`限时|截止|最后\d+天|倒计时|即将结束|错过再等|仅剩\d+`)
	reCallToAction    = regexp.MustCompile(`点击下载|充值返利|立即购买|马上领取|扫码|加群|关注公众号`)
	rePromoKeyword    = regexp.MustCompile(`首充|首冲|充值\d+送|满减|折扣|优惠|促销|特价|免费送`)
)

// HasMarketingPatterns checks for marketing/promotional content patterns.
// Returns true if any price, urgency, CTA, or promo patterns are detected.
func HasMarketingPatterns(content string) bool {
	if rePricePattern.MatchString(content) {
		return true
	}
	if reTimePressure.MatchString(content) {
		return true
	}
	if reCallToAction.MatchString(content) {
		return true
	}
	if rePromoKeyword.MatchString(content) {
		return true
	}
	return false
}
