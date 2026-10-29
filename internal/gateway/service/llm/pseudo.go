package llm

import (
	"context"
	"fmt"
	"strings"
	"unicode"
)

// PseudoStream generates a deterministic dev answer from context.
func PseudoStream(ctx context.Context, messages []Message, emit func(StreamDelta) error) error {
	answer := synthesizePseudoAnswer(messages)
	parts := splitForStream(answer)
	for i, p := range parts {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		if err := emit(StreamDelta{Content: p}); err != nil {
			return err
		}
		if i < len(parts)-1 {
			// small pacing without sleep for tests; optional sleep in prod
		}
	}
	return emit(StreamDelta{Done: true})
}

func synthesizePseudoAnswer(messages []Message) string {
	var userQ, contextBlock string
	for _, m := range messages {
		switch m.Role {
		case "user":
			userQ = messageContentString(m.Content)
		case "system":
			contextBlock = messageContentString(m.Content)
		}
	}
	if userQ == "" {
		userQ = "你的问题"
	}
	if contextBlock != "" {
		return "根据知识库内容，针对「" + trimRunes(userQ, 80) + "」：\n\n" +
			trimRunes(extractContextSnippet(contextBlock), 600) +
			"\n\n（开发环境 pseudo 模型生成，配置 OPENAI_API_KEY 后可切换真实 LLM。）"
	}
	return "（pseudo）关于「" + trimRunes(userQ, 80) + "」，当前未提供知识库上下文。"
}

func extractContextSnippet(system string) string {
	if idx := strings.Index(system, "参考资料"); idx >= 0 {
		return strings.TrimSpace(system[idx:])
	}
	return system
}

func splitForStream(text string) []string {
	var parts []string
	var buf strings.Builder
	for _, r := range text {
		buf.WriteRune(r)
		if unicode.IsPunct(r) || r == '\n' || r == ' ' {
			if buf.Len() > 0 {
				parts = append(parts, buf.String())
				buf.Reset()
			}
		}
	}
	if buf.Len() > 0 {
		parts = append(parts, buf.String())
	}
	if len(parts) == 0 {
		return []string{text}
	}
	return parts
}

// messageContentString extracts a string representation from a Message Content field.
func messageContentString(v any) string {
	switch c := v.(type) {
	case string:
		return c
	case []any:
		var parts []string
		for _, part := range c {
			if m, ok := part.(map[string]any); ok {
				if t, _ := m["type"].(string); t == "text" {
					if txt, _ := m["text"].(string); txt != "" {
						parts = append(parts, txt)
					}
				}
			}
		}
		return strings.Join(parts, " ")
	default:
		return fmt.Sprintf("%v", c)
	}
}

func trimRunes(s string, max int) string {
	r := []rune(s)
	if len(r) <= max {
		return s
	}
	return string(r[:max]) + "…"
}
