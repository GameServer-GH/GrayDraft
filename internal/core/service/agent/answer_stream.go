package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

// TokenStreamer streams assistant tokens from an LLM. Implemented by llmclient.Client.
type TokenStreamer interface {
	StreamTokens(ctx context.Context, system, user string, onDelta func(string) error) error
}

// ReasoningTokenStreamer optionally forwards model reasoning_content alongside answer tokens.
type ReasoningTokenStreamer interface {
	StreamTokensWithReasoning(ctx context.Context, system, user string, onContent, onReasoning func(string) error) error
}

// StreamFinalAnswer generates the user-facing answer via token streaming, emitting
// answer.delta events. Falls back to draft when the streamer is unavailable or fails.
// Model reasoning (if any) is forwarded via ReasoningSinkFrom(ctx).
func StreamFinalAnswer(
	ctx context.Context,
	streamer TokenStreamer,
	prompt []byte,
	obs []Observation,
	draft string,
	emit func(delta string) error,
) (string, error) {
	if streamer == nil {
		return strings.TrimSpace(draft), nil
	}
	onReasoning := ReasoningSinkFrom(ctx)
	system := `你是书生资料学习助手。根据证据如实作答，使用简体中文 Markdown。
不要编造证据中没有的数值、参数或版本。证据不足时明确说明资料里没有这部分内容。`
	if onReasoning != nil {
		system += "\n推理与思考过程也请全程使用简体中文。"
	} else {
		system += "\n请简洁直接作答，不要复述证据原文。"
	}
	user := buildAnswerStreamPrompt(prompt, obs, draft)
	var b strings.Builder
	onContent := func(delta string) error {
		if delta == "" {
			return nil
		}
		b.WriteString(delta)
		if emit != nil {
			return emit(delta)
		}
		return nil
	}

	var err error
	if rs, ok := streamer.(ReasoningTokenStreamer); ok {
		// Final answers stay on the complex model. A nil reasoning sink only hides the thought trace.
		err = rs.StreamTokensWithReasoning(ctx, system, user, onContent, onReasoning)
	} else {
		err = streamer.StreamTokens(ctx, system, user, onContent)
	}
	out := strings.TrimSpace(b.String())
	if err != nil {
		if out != "" {
			return out, nil
		}
		if d := strings.TrimSpace(draft); d != "" {
			return d, nil
		}
		return "", err
	}
	if out == "" {
		return strings.TrimSpace(draft), nil
	}
	return out, nil
}

func buildAnswerStreamPrompt(prompt []byte, obs []Observation, draft string) string {
	var b strings.Builder
	b.WriteString("用户问题：\n")
	b.WriteString(string(prompt))
	b.WriteString("\n\n证据：\n")
	if len(obs) == 0 {
		b.WriteString("（无工具观测）\n")
	}
	for i, o := range obs {
		if o.Err != nil || len(o.Output) == 0 {
			continue
		}
		fmt.Fprintf(&b, "[%d] tool=%s\n%s\n\n", i+1, o.Name, truncateRunes(string(o.Output), 2400))
	}
	if d := strings.TrimSpace(draft); d != "" {
		b.WriteString("规划草稿（可参考，请输出最终完整答案）：\n")
		b.WriteString(d)
		b.WriteString("\n")
	}
	b.WriteString("\n请直接输出最终答案，不要输出 JSON。")
	return b.String()
}

func truncateRunes(s string, max int) string {
	r := []rune(s)
	if len(r) <= max {
		return s
	}
	return string(r[:max]) + "…"
}

// CompactObservationsJSON is a test helper for prompt inspection.
func CompactObservationsJSON(obs []Observation) string {
	raw, _ := json.Marshal(obs)
	return string(raw)
}
