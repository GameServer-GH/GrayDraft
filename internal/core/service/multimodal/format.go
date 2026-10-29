package multimodal

import (
	"encoding/json"
	"regexp"
	"strings"
)

var (
	reCodeFence     = regexp.MustCompile("(?is)^```(?:json)?\\s*\\n?")
	reTrailingFence = regexp.MustCompile("(?is)\\n?```\\s*$")
	reBareJSONLine  = regexp.MustCompile(`(?m)^\s*json\s*$`)
)

// FormatExtractionText normalizes model output for knowledge ingest (strip fences, drop visual noise).
func FormatExtractionText(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	raw = stripCodeFences(raw)
	parsed := parseModelOutput(raw)
	if strings.TrimSpace(parsed.Text) != "" {
		return filterVisualNoiseLines(strings.TrimSpace(parsed.Text))
	}
	return filterVisualNoiseLines(strings.TrimSpace(raw))
}

func filterVisualNoiseLines(text string) string {
	if text == "" {
		return ""
	}
	var kept []string
	for _, line := range strings.Split(text, "\n") {
		trim := strings.TrimSpace(line)
		if trim == "" {
			kept = append(kept, "")
			continue
		}
		if looksLikeVisualNoise(trim) && !strings.Contains(trim, "|") {
			continue
		}
		kept = append(kept, line)
	}
	return strings.TrimSpace(strings.Join(kept, "\n"))
}

func stripCodeFences(s string) string {
	s = strings.TrimSpace(s)
	for i := 0; i < 3; i++ {
		prev := s
		s = reCodeFence.ReplaceAllString(s, "")
		s = reTrailingFence.ReplaceAllString(s, "")
		s = strings.TrimSpace(s)
		if s == prev {
			break
		}
	}
	s = reBareJSONLine.ReplaceAllString(s, "")
	return strings.TrimSpace(s)
}

// parseModelOutputJSONFields extracts knowledge text from VLM JSON without visual_elements noise.
func parseModelOutputJSONFields(obj map[string]any) string {
	if tc, ok := obj["text_content"].(string); ok && strings.TrimSpace(tc) != "" {
		return strings.TrimSpace(tc)
	}
	var parts []string
	if v, ok := obj["summary"].(string); ok && strings.TrimSpace(v) != "" {
		s := strings.TrimSpace(v)
		if !looksLikeVisualNoise(s) {
			parts = append(parts, s)
		}
	}
	if v, ok := obj["semantic_intent"].(string); ok && strings.TrimSpace(v) != "" {
		parts = append(parts, strings.TrimSpace(v))
	}
	if rows := formatSpawnRecordsField(obj["spawn_records"]); rows != "" {
		parts = append(parts, rows)
	}
	kv := flattenStructuredField(obj["structured_kv"])
	if kv == "" {
		kv = flattenStructuredField(obj["kv_pairs"])
	}
	if kv != "" {
		parts = append(parts, kv)
	}
	return strings.TrimSpace(strings.Join(parts, "\n\n"))
}

func formatSpawnRecordsField(v any) string {
	arr, ok := v.([]any)
	if !ok || len(arr) == 0 {
		return ""
	}
	var lines []string
	for _, item := range arr {
		m, ok := item.(map[string]any)
		if !ok {
			continue
		}
		subject := toString(m["subject"])
		if subject == "" {
			subject = toString(m["entity"])
		}
		if subject == "" {
			subject = toString(m["spirit"])
		}
		if subject == "" {
			subject = toString(m["name"])
		}
		if subject == "" {
			continue
		}
		var block []string
		block = append(block, "### "+subject)
		w := toString(m["window"])
		if w == "" {
			w = toString(m["time"])
		}
		if w != "" {
			block = append(block, "- 时间/区间："+w)
		}
		rec := toString(m["recommendation"])
		if rec == "" {
			rec = toString(m["tool"])
		}
		if rec == "" {
			rec = toString(m["ball"])
		}
		if rec != "" {
			block = append(block, "- 建议/推荐："+rec)
		}
		if a := toString(m["attributes"]); a != "" {
			block = append(block, "- 属性/类型："+a)
		}
		if mat := toString(m["materials"]); mat != "" {
			block = append(block, "- 材料/依赖："+mat)
		}
		if note := toString(m["notes"]); note != "" {
			block = append(block, "- 备注："+note)
		}
		lines = append(lines, strings.Join(block, "\n"))
	}
	return strings.Join(lines, "\n\n")
}

func looksLikeVisualNoise(s string) bool {
	lower := strings.ToLower(s)
	noise := []string{"复古", "米黄色", "背景为", "视觉层次", "字体为", "装饰", "水印", "图鉴风格", "纸张质感"}
	for _, k := range noise {
		if strings.Contains(lower, k) {
			return true
		}
	}
	return false
}

func tryParseJSONObject(s string) (map[string]any, bool) {
	s = strings.TrimSpace(s)
	if s == "" || s[0] != '{' {
		return nil, false
	}
	var obj map[string]any
	if err := json.Unmarshal([]byte(s), &obj); err != nil {
		return nil, false
	}
	return obj, true
}
