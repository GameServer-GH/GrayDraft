package agentlog

import (
	"encoding/json"
	"strconv"
)

// SanitizeRequest copies smart-import request with redacted attachments.
func SanitizeRequest(req any) any {
	b, err := json.Marshal(req)
	if err != nil {
		return req
	}
	var v any
	if json.Unmarshal(b, &v) != nil {
		return req
	}
	redact(v)
	return v
}

func redact(v any) {
	switch t := v.(type) {
	case map[string]any:
		if att, ok := t["attachments"].([]any); ok {
			for i, item := range att {
				am, ok := item.(map[string]any)
				if !ok {
					continue
				}
				if c, ok := am["content"].(string); ok && len(c) > 128 {
					am["content"] = "[base64 " + strconv.Itoa(len(c)) + " bytes redacted]"
				}
				att[i] = am
			}
		}
		if raw, ok := t["raw_text"].(string); ok && len(raw) > 32<<10 {
			t["raw_text"] = raw[:32<<10] + "…(truncated)"
		}
		for k, val := range t {
			if k == "api_key" {
				t[k] = "[redacted]"
				continue
			}
			redact(val)
		}
	case []any:
		for _, item := range t {
			redact(item)
		}
	}
}
