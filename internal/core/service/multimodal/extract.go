package multimodal

import (
	"encoding/json"
	"strings"
)

// parsedExtraction holds normalized multimodal output.
type parsedExtraction struct {
	Text         string
	StructuredKV string
	Timestamps   []string
}

func parseModelOutput(s string) parsedExtraction {
	s = stripCodeFences(strings.TrimSpace(s))
	out := parsedExtraction{Text: s}
	var obj map[string]any
	if json.Unmarshal([]byte(s), &obj) != nil {
		if obj2, ok := tryParseJSONObject(s); ok {
			obj = obj2
		} else {
			return out
		}
	}
	if formatted := parseModelOutputJSONFields(obj); formatted != "" {
		out.Text = formatted
	}
	out.StructuredKV = flattenStructuredField(obj["structured_kv"])
	if out.StructuredKV == "" {
		out.StructuredKV = flattenStructuredField(obj["kv_pairs"])
	}
	if ts, ok := obj["timestamps"].([]any); ok {
		for _, t := range ts {
			if str, ok := t.(string); ok && str != "" {
				out.Timestamps = append(out.Timestamps, str)
			}
		}
	}
	return out
}

func flattenStructuredField(v any) string {
	switch t := v.(type) {
	case string:
		return strings.TrimSpace(t)
	case map[string]any:
		var lines []string
		for k, val := range t {
			lines = append(lines, k+": "+toString(val))
		}
		return strings.Join(lines, "\n")
	case []any:
		var lines []string
		for _, item := range t {
			switch row := item.(type) {
			case map[string]any:
				k := toString(row["key"])
				val := toString(row["value"])
				if k != "" {
					lines = append(lines, k+": "+val)
				}
			case string:
				if row != "" {
					lines = append(lines, row)
				}
			}
		}
		return strings.Join(lines, "\n")
	default:
		return ""
	}
}

func toString(v any) string {
	if v == nil {
		return ""
	}
	switch t := v.(type) {
	case string:
		return strings.TrimSpace(t)
	default:
		b, _ := json.Marshal(t)
		return strings.TrimSpace(string(b))
	}
}

func mergeTextAndStructured(text, structured string) string {
	text = strings.TrimSpace(text)
	structured = strings.TrimSpace(structured)
	switch {
	case text != "" && structured != "":
		return text + "\n\n--- structured ---\n\n" + structured
	case structured != "":
		return structured
	default:
		return text
	}
}
