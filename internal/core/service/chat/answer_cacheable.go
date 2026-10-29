package chat

import "strings"

// noInfoAnswerMarkers are substrings indicating the model had nothing useful to say.
// Such answers must not be written to answer cache.
var noInfoAnswerMarkers = []string{
	"没有找到相关信息",
	"未能检索到",
	"无法回答您的问题",
	"无法回答你的问题",
	"无法回答该问题",
	"无法回答这个问题",
	"无检索结果",
	"未找到相关知识",
	"未入录该知识点",
	"没有相关信息",
	"没有找",
	"资料不足",
	"无法根据",
	"不能回答",
}

// IsAnswerCacheable reports whether a chat answer should be stored in answer cache.
// Now accepts grade for kb_only caching decisions.
func IsAnswerCacheable(answer string, chunksUsed int) bool {
	if chunksUsed <= 0 {
		return false
	}
	a := strings.TrimSpace(answer)
	if a == "" {
		return false
	}
	lower := strings.ToLower(a)
	for _, m := range noInfoAnswerMarkers {
		if strings.Contains(lower, strings.ToLower(m)) {
			return false
		}
	}
	return true
}
