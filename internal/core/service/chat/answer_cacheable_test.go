package chat

import "testing"

func TestIsAnswerCacheable(t *testing.T) {
	tests := []struct {
		answer     string
		chunksUsed int
		want       bool
	}{
		{"", 1, false},
		{"正常回答内容。", 2, true},
		{"抱歉，当前知识库中没有找到相关信息，无法回答您的问题。", 0, false},
		{"抱歉，当前知识库中没有找到相关信息，无法回答您的问题。", 3, false},
		{"很抱歉，根据您提供的参考资料，未能检索到关于 Bug 报告的相关内容。", 1, false},
		{"（检索模式，LLM 不可用）相关片段：\n1. foo", 2, true},
		{"未找到相关知识片段。", 0, false},
	}
	for _, tt := range tests {
		if got := IsAnswerCacheable(tt.answer, tt.chunksUsed); got != tt.want {
			t.Errorf("IsAnswerCacheable(%q, %d) = %v, want %v", tt.answer, tt.chunksUsed, got, tt.want)
		}
	}
}
