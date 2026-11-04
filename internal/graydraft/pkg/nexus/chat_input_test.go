package nexus

import "testing"

func TestAgentQuestionDoesNotIncludeTransportConfig(t *testing.T) {
	q := "第一行问题\n第二行问题"
	got := buildAgentRunPrompt(ChatStreamRequest{Query: q, TopK: 9, ChatMode: "hybrid", WorkspaceContext: "workspace background"})
	if got != q {
		t.Fatalf("question rewritten: %q", got)
	}
}
