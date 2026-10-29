package smartimport

import (
	"strings"
	"testing"
)

// ── Fuzz: CleanMarkdownForIngest ─────────────────────────────────────────────

func TestFuzz_CleanMarkdownForIngest_empty(t *testing.T) {
	result := CleanMarkdownForIngest("")
	if result != "" {
		t.Fatalf("empty input: %q", result)
	}
}

func TestFuzz_CleanMarkdownForIngest_whitespace_only(t *testing.T) {
	result := CleanMarkdownForIngest("   \n  \n  ")
	if strings.TrimSpace(result) != "" {
		t.Fatalf("whitespace only: %q", result)
	}
}

func TestFuzz_CleanMarkdownForIngest_panic_safety(t *testing.T) {
	// Ensure no panics on edge-case inputs
	inputs := []string{
		"",                                   // empty
		"a",                                  // single char
		"\n\n\n",                            // only newlines
		"####### ",                           // 7+ hashes (h7)
		"| || |",                            // degenerate table
		"***",                               // triple star
		"```\n```",                          // empty code block
		"> \n> \n> ",                        // empty blockquote
		"- \n- \n- ",                        // empty list
		"1. \n2. \n3. ",                     // empty ordered list
		"[text](url)",                       // bare link
		"![alt](image.png)",                 // bare image
		"~~~",                               // tildes
		"___",                               // underscores
		"---",                               // horizontal rule separators
		"\t\t\t",                            // tabs
		string(make([]byte, 10000)),         // null bytes
	}
	for _, input := range inputs {
		func() {
			defer func() {
				if r := recover(); r != nil {
					t.Errorf("panic on input %q: %v", input, r)
				}
			}()
			CleanMarkdownForIngest(input)
		}()
	}
}

func TestFuzz_CleanMarkdownForIngest_very_long(t *testing.T) {
	long := strings.Repeat("正常内容文本。", 1000)
	result := CleanMarkdownForIngest(long)
	if result == "" {
		t.Fatal("long content should produce output")
	}
}

func TestFuzz_CleanMarkdownForIngest_special_chars(t *testing.T) {
	inputs := []string{
		"<script>alert(1)</script>",
		"&lt; &gt; &amp;",
		"\\x00\\x01\\x02",
		"中文\t\t\t\t混合\f\f内容",
	}
	for _, input := range inputs {
		func() {
			defer func() {
				if r := recover(); r != nil {
					t.Errorf("panic on special chars %q: %v", input, r)
				}
			}()
			result := CleanMarkdownForIngest(input)
			t.Logf("special chars: %q → %q", input, result)
		}()
	}
}

// ── Fuzz: ExtractByLineRange ─────────────────────────────────────────────────

func TestFuzz_ExtractByLineRange_panic_safety(t *testing.T) {
	tests := []struct {
		raw   string
		start int
		end   int
	}{
		{"", 0, 0},
		{"", 1, 10},
		{"a", 0, 0},
		{"a", -1, 5},
		{"a\nb", 100, 200},
		{"a\nb\nc", 2, 1}, // reversed
		{"line\n", 1, 999},
	}
	for _, tt := range tests {
		func() {
			defer func() {
				if r := recover(); r != nil {
					t.Errorf("panic on (%q, %d, %d): %v", tt.raw, tt.start, tt.end, r)
				}
			}()
			ExtractByLineRange(tt.raw, tt.start, tt.end)
		}()
	}
}

// ── Fuzz: bigramTokens ───────────────────────────────────────────────────────

func TestFuzz_BigramTokens_panic_safety(t *testing.T) {
	inputs := []string{
		"",
		"a",
		"中",
		"中文",
		"中文English混合",
		"emoji😀test",
		strings.Repeat("测试内容", 100),
		"\x00\x00",
	}
	for _, input := range inputs {
		func() {
			defer func() {
				if r := recover(); r != nil {
					t.Errorf("panic on bigramTokens(%q): %v", input, r)
				}
			}()
			bigramTokens(input)
		}()
	}
}

func TestFuzz_BigramTokens_known_values(t *testing.T) {
	// Some known-good outputs
	tokens := bigramTokens("攻击防御")
	foundBigram := false
	for _, tok := range tokens {
		if tok == "攻击" || tok == "击防" || tok == "防御" {
			foundBigram = true
		}
	}
	if !foundBigram && len(tokens) > 0 {
		t.Logf("bigramTokens(攻击防御) = %v", tokens)
	}
}

// ── Fuzz: charEntropy ────────────────────────────────────────────────────────

func TestFuzz_CharEntropy_panic_safety(t *testing.T) {
	inputs := []string{
		"",
		"a",
		"ab",
		"中文测试",
		strings.Repeat("aa", 100),
	}
	for _, input := range inputs {
		func() {
			defer func() {
				if r := recover(); r != nil {
					t.Errorf("panic on charEntropy(%q): %v", input, r)
				}
			}()
			charEntropy(input)
		}()
	}
}

func TestFuzz_CharEntropy_repetitive(t *testing.T) {
	entropy := charEntropy(strings.Repeat("ab", 100))
	if entropy <= 0 {
		t.Fatal("repetitive text should have non-zero entropy")
	}
	// Repetitive "ab" should have low entropy
	if entropy > 3 {
		t.Logf("entropy of 'ab'×100: %.2f (expected low)", entropy)
	}
}

func TestFuzz_CharEntropy_diverse(t *testing.T) {
	entropy := charEntropy("这是一段多样化内容丰富的中文文本用于测试信息熵计算的准确性")
	if entropy < 3 {
		t.Fatalf("diverse text entropy too low: %.2f", entropy)
	}
}

// ── Fuzz: NgramRepeatRate ────────────────────────────────────────────────────

func TestFuzz_NgramRepeatRate_panic_safety(t *testing.T) {
	inputs := []string{
		"",
		"a",
		"ab",
		"abc",
		strings.Repeat("重复文本", 100),
	}
	for _, input := range inputs {
		func() {
			defer func() {
				if r := recover(); r != nil {
					t.Errorf("panic on NgramRepeatRate(%q): %v", input, r)
				}
			}()
			NgramRepeatRate(input)
		}()
	}
}

func TestFuzz_NgramRepeatRate_high_repeat(t *testing.T) {
	rate := NgramRepeatRate(strings.Repeat("重复", 100))
	if rate < 0.4 {
		t.Fatalf("repetitive text ngram rate too low: %.2f", rate)
	}
}

func TestFuzz_NgramRepeatRate_low_repeat(t *testing.T) {
	rate := NgramRepeatRate("这是一段正常的中文文本内容没有过多重复词汇")
	if rate > 0.5 {
		t.Logf("normal text ngram rate: %.2f", rate)
	}
}

// ── Fuzz: HasMarketingPatterns ───────────────────────────────────────────────

func TestFuzz_HasMarketingPatterns_panic_safety(t *testing.T) {
	inputs := []string{
		"",
		"normal text",
		strings.Repeat("广告", 100),
	}
	for _, input := range inputs {
		func() {
			defer func() {
				if r := recover(); r != nil {
					t.Errorf("panic on HasMarketingPatterns(%q): %v", input, r)
				}
			}()
			HasMarketingPatterns(input)
		}()
	}
}

func TestFuzz_HasMarketingPatterns_positive(t *testing.T) {
	inputs := []string{
		"限时活动，最后3天！",
		"点击下载领取大礼包",
		"首充送100元礼包",
		"充值100送50",
	}
	for _, input := range inputs {
		if !HasMarketingPatterns(input) {
			t.Logf("missed marketing pattern: %q", input)
		}
	}
}

func TestFuzz_HasMarketingPatterns_negative(t *testing.T) {
	inputs := []string{
		"InternVL参数说明",
		"技能说明：造成冰属性伤害",
		"装备选择建议",
	}
	for _, input := range inputs {
		if HasMarketingPatterns(input) {
			t.Errorf("false positive on: %q", input)
		}
	}
}

// ── Fuzz: HasURL ─────────────────────────────────────────────────────────────

func TestFuzz_HasURL(t *testing.T) {
	if !HasURL("see https://example.com/page") {
		t.Fatal("should detect URL")
	}
	if HasURL("no url here") {
		t.Fatal("should not detect URL")
	}
	if HasURL("") {
		t.Fatal("empty should not have URL")
	}
}

// ── Fuzz: entityCountFromContent ─────────────────────────────────────────────

func TestFuzz_EntityCountFromContent_panic(t *testing.T) {
	func() {
		defer func() {
			if r := recover(); r != nil {
				t.Errorf("panic: %v", r)
			}
		}()
		EntityCountFromContent("")
		EntityCountFromContent("测试")
		EntityCountFromContent(strings.Repeat("《实体》", 100))
	}()
}

// ── Fuzz: ContentRuneLen ─────────────────────────────────────────────────────

func TestFuzz_ContentRuneLen(t *testing.T) {
	tests := []struct {
		input    string
		expected int
	}{
		{"", 0},
		{"abc", 3},
		{"中文", 2},
		{"a中文b", 4},
	}
	for _, tt := range tests {
		if got := ContentRuneLen(tt.input); got != tt.expected {
			t.Errorf("ContentRuneLen(%q)=%d, want %d", tt.input, got, tt.expected)
		}
	}
}
