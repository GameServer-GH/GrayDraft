package agent

import (
	"context"
	"strings"
	"testing"
)

type captureStreamer struct{ system string }

func (c *captureStreamer) StreamTokens(_ context.Context, system, _ string, emit func(string) error) error {
	c.system = system
	return emit("资料回答。")
}

func TestAnswerPromptKeepsProperNamesFromEvidence(t *testing.T) {
	s := &captureStreamer{}
	_, err := StreamFinalAnswer(context.Background(), s, []byte("各自讲什么"), nil, "", nil)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(s.system, "按证据原文保留") || !strings.Contains(s.system, "不要换成同一段里出现的另一个名称") {
		t.Fatal(s.system)
	}
}
