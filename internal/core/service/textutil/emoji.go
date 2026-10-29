package textutil

import (
	"regexp"
	"strings"
)

// emojiPattern matches common pictographs, symbols, flags, and joiners used in Emoji sequences.
var emojiPattern = regexp.MustCompile(
	`(?:[\x{1F1E0}-\x{1F1FF}]{2})|` +
		`[\x{1F600}-\x{1F64F}]|` +
		`[\x{1F300}-\x{1F5FF}]|` +
		`[\x{1F680}-\x{1F6FF}]|` +
		`[\x{1F700}-\x{1F77F}]|` +
		`[\x{1F780}-\x{1F7FF}]|` +
		`[\x{1F800}-\x{1F8FF}]|` +
		`[\x{1F900}-\x{1F9FF}]|` +
		`[\x{1FA00}-\x{1FA6F}]|` +
		`[\x{1FA70}-\x{1FAFF}]|` +
		`[\x{2600}-\x{26FF}]|` +
		`[\x{2700}-\x{27BF}]|` +
		`[\x{2300}-\x{23FF}]|` +
		`[\x{2B50}-\x{2B55}]|` +
		`[\x{FE00}-\x{FE0F}]|` +
		`[\x{200D}]|` +
		`[\x{20E3}]|` +
		`[\x{E0020}-\x{E007F}]`,
)

// StripEmojis removes Emoji / pictograph characters from text.
func StripEmojis(s string) string {
	if s == "" {
		return s
	}
	out := emojiPattern.ReplaceAllString(s, "")
	out = strings.ReplaceAll(out, "\u200b", "")
	return strings.TrimSpace(out)
}
