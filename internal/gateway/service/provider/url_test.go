package provider

import "testing"

func TestDashScopeRerankURL(t *testing.T) {
	got := DashScopeRerankURL("https://dashscope.aliyuncs.com/compatible-mode")
	want := "https://dashscope.aliyuncs.com/api/v1/services/rerank/text-rerank/text-rerank"
	if got != want {
		t.Fatal(got)
	}
}

func TestOpenAIURL(t *testing.T) {
	if got := openAIURL("https://chat.intern-ai.org.cn/api/v1", "chat/completions"); got != "https://chat.intern-ai.org.cn/api/v1/chat/completions" {
		t.Fatal(got)
	}
	if got := openAIURL("https://api.deepseek.com", "embeddings"); got != "https://api.deepseek.com/v1/embeddings" {
		t.Fatal(got)
	}
	if got := openAIURL("https://dashscope.aliyuncs.com/compatible-mode/", "embeddings"); got != "https://dashscope.aliyuncs.com/compatible-mode/v1/embeddings" {
		t.Fatal(got)
	}
}
