package agent

import (
	"context"
	"fmt"
	"strings"
)

type MemoryMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type ShortTermMemory interface {
	Append(context.Context, string, MemoryMessage) error
	Recent(context.Context, string, int) ([]MemoryMessage, error)
}

type SummaryProvider interface {
	LatestSummary(context.Context, string) (string, error)
}
type MemoryCompactor interface {
	Compact(context.Context, string, int) error
}

type ContextBuilder struct {
	Memory      ShortTermMemory
	MaxMessages int
	MaxChars    int
}

func (b ContextBuilder) Build(ctx context.Context, conversationID, prompt string) ([]byte, error) {
	if strings.TrimSpace(prompt) == "" {
		return nil, fmt.Errorf("agent: prompt is empty")
	}
	if b.Memory == nil || conversationID == "" {
		return []byte(prompt), nil
	}
	n := b.MaxMessages
	if n <= 0 {
		n = 20
	}
	maxChars := b.MaxChars
	if maxChars <= 0 {
		maxChars = 24000
	}
	msgs, err := b.Memory.Recent(ctx, conversationID, n)
	if err != nil {
		return nil, err
	}
	var sb strings.Builder
	if summaries, ok := b.Memory.(SummaryProvider); ok {
		if summary, err := summaries.LatestSummary(ctx, conversationID); err == nil && summary != "" {
			sb.WriteString("conversation_summary: ")
			sb.WriteString(summary)
			sb.WriteString("\n")
		}
	}
	used := 0
	for _, m := range msgs {
		if len(m.Content) > 4000 {
			m.Content = m.Content[:4000]
		}
		if used+len(m.Content) > maxChars {
			break
		}
		sb.WriteString(m.Role)
		sb.WriteString(": ")
		sb.WriteString(m.Content)
		sb.WriteString("\n")
		used += len(m.Content)
	}
	sb.WriteString("user: ")
	sb.WriteString(prompt)
	return []byte(sb.String()), nil
}
