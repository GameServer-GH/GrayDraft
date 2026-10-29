package agentlog

import (
	"encoding/json"
)

// SanitizeJSON redacts large base64 attachment payloads in smart-import bodies.
func SanitizeJSON(raw json.RawMessage) json.RawMessage {
	if len(raw) == 0 {
		return raw
	}
	var v any
	if json.Unmarshal(raw, &v) != nil {
		return raw
	}
	redactValue(v)
	b, err := json.Marshal(v)
	if err != nil {
		return raw
	}
	return b
}

func redactValue(v any) {
	switch t := v.(type) {
	case map[string]any:
		redactMap(t)
	case []any:
		for _, item := range t {
			redactValue(item)
		}
	}
}

func redactMap(m map[string]any) {
	// Redact base64 attachment payloads — keep data_url for debugging.
	if att, ok := m["attachments"].([]any); ok {
		for i, item := range att {
			am, ok := item.(map[string]any)
			if !ok {
				continue
			}
			if b64, ok := am["data_base64"].(string); ok && len(b64) > 0 {
				am["data_base64"] = formatRedacted(len(b64))
			}
			att[i] = am
		}
		m["attachments"] = att
	}
	// Redact top-level media base64 fields.
	for _, k := range []string{"media_image_base64", "media_video_base64"} {
		if v, ok := m[k].(string); ok && len(v) > 0 {
			m[k] = formatRedacted(len(v))
		}
	}
	if raw, ok := m["raw_text"].(string); ok && len(raw) > 32<<10 {
		m["raw_text"] = raw[:32<<10] + fmtTrunc(len(raw))
	}
	for k, val := range m {
		if k == "api_key" || k == "APIKey" {
			m[k] = "[redacted]"
			continue
		}
		redactValue(val)
	}
}

func formatRedacted(n int) string {
	return "[base64 " + itoa(n) + " bytes redacted]"
}

func fmtTrunc(n int) string {
	return "…(truncated, " + itoa(n) + " bytes total)"
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var d [20]byte
	i := len(d)
	for n > 0 {
		i--
		d[i] = byte('0' + n%10)
		n /= 10
	}
	return string(d[i:])
}
