package agent

import (
	"regexp"
	"strings"
	"unicode/utf8"
)

var (
	reKBTypeID   = regexp.MustCompile(`(?i)\bgd_[a-f0-9]{8,}\b`)
	reUUID       = regexp.MustCompile(`(?i)\b[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}\b`)
	reKBTypeKV   = regexp.MustCompile(`(?i)\bkb_type\s*[:=]\s*["']?[\w.-]+["']?`)
	reTopKKV     = regexp.MustCompile(`(?i)\btop_k\s*[:=]\s*\d+`)
	reJSONFence  = regexp.MustCompile("(?s)```(?:json)?\\s*.*?```")
	reProtoNoise = regexp.MustCompile(`(?i)(return json only|schema\s*:|tool_calls|decision\s*[:=]|thought_summary|call_id|arguments\s*:|confidence\s*[:=])`)
)

// SanitizeUserThought turns planner notes into short, user-safe progress text.
// It strips internal IDs, protocol jargon and long chain-of-thought dumps.
func SanitizeUserThought(raw string) string {
	s := strings.TrimSpace(raw)
	if s == "" {
		return ""
	}
	s = reJSONFence.ReplaceAllString(s, "")
	s = reKBTypeID.ReplaceAllString(s, "")
	s = reUUID.ReplaceAllString(s, "")
	s = reKBTypeKV.ReplaceAllString(s, "")
	s = reTopKKV.ReplaceAllString(s, "")
	s = reProtoNoise.ReplaceAllString(s, "")
	s = strings.ReplaceAll(s, "\r\n", "\n")
	// Keep first meaningful sentence/paragraph only.
	if i := strings.IndexAny(s, "\n。！？"); i >= 0 {
		cut := i
		if r, _ := utf8.DecodeRuneInString(s[i:]); r == '。' || r == '！' || r == '？' {
			cut = i + utf8.RuneLen(r)
		}
		s = s[:cut]
	}
	s = strings.Join(strings.Fields(s), " ")
	s = strings.Trim(s, " :：,，;；\"'`")
	const maxRunes = 80
	runes := []rune(s)
	if len(runes) > maxRunes {
		s = string(runes[:maxRunes]) + "…"
	}
	return strings.TrimSpace(s)
}
