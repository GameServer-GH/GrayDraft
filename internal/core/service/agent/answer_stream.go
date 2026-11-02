package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"time"
)

var reHyphenName = regexp.MustCompile(`[A-Za-z][A-Za-z0-9]*(?:-[A-Za-z0-9]+)+`)

// TokenStreamer streams assistant tokens from an LLM. Implemented by llmclient.Client.
type TokenStreamer interface {
	StreamTokens(ctx context.Context, system, user string, onDelta func(string) error) error
}

// ReasoningTokenStreamer optionally forwards model reasoning_content alongside answer tokens.
type ReasoningTokenStreamer interface {
	StreamTokensWithReasoning(ctx context.Context, system, user string, onContent, onReasoning func(string) error) error
}

// AnswerGenerationError marks a failed finalization attempt. Retrying the same
// queued run would follow a terminal event with a second event sequence.
type AnswerGenerationError struct{ Cause error }

func (e *AnswerGenerationError) Error() string            { return e.Cause.Error() }
func (e *AnswerGenerationError) Unwrap() error            { return e.Cause }
func (e *AnswerGenerationError) TerminalRunFailure() bool { return true }

// StreamFinalAnswer generates the user-facing answer via token streaming, emitting
// answer.delta events after successful completion. A failed or interrupted stream
// never commits its partial text. Drafts are used only when no streamer is configured.
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
	if isContentOverview(CurrentUserQuery(string(prompt))) && len(overviewGroups(obs)) > 1 {
		return streamGroupedOverview(ctx, streamer, prompt, obs, emit)
	}
	onReasoning := ReasoningSinkFrom(ctx)
	system := `你是书生资料学习助手。根据证据如实作答，使用简体中文 Markdown。
不要编造证据中没有的数值、参数或版本。证据不足时明确说明资料里没有这部分内容。
型号、版本和参数名按证据原文保留，不要换成同一段里出现的另一个名称。`
	if onReasoning != nil {
		system += "\n推理与思考过程也请全程使用简体中文。"
	} else {
		system += "\n请简洁直接作答，不要大段照抄证据。"
	}
	user := buildAnswerStreamPrompt(prompt, obs, draft)
	var b strings.Builder
	onContent := func(delta string) error {
		if delta == "" {
			return nil
		}
		b.WriteString(delta)
		return nil
	}

	started := time.Now()
	var err error
	if rs, ok := streamer.(ReasoningTokenStreamer); ok {
		// Final answers stay on the complex model. A nil reasoning sink only hides the thought trace.
		err = rs.StreamTokensWithReasoning(ctx, system, user, onContent, onReasoning)
	} else {
		err = streamer.StreamTokens(ctx, system, user, onContent)
	}
	out := strings.TrimSpace(b.String())
	if err == nil {
		err = ctx.Err()
	}
	logAnswerCompletion(ctx, "answer", "", started, len(out), err)
	if err != nil {
		return "", err
	}
	if out == "" {
		return "", fmt.Errorf("answer generation returned no content")
	}
	if emit != nil {
		if err := emit(out); err != nil {
			return "", err
		}
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
		limit := 2400
		if o.Name == "knowledge.read" {
			limit = 12000
		}
		fmt.Fprintf(&b, "[%d] tool=%s\n%s\n\n", i+1, o.Name, truncateRunes(string(o.Output), limit))
	}
	if d := strings.TrimSpace(draft); d != "" {
		b.WriteString("可选提纲（不要照抄，请依据证据重写完整答案）：\n")
		b.WriteString(d)
		b.WriteString("\n")
	}
	b.WriteString("\n请直接输出最终答案，不要输出 JSON。证据不足时明确说明本工作区资料没有。")

	return b.String()
}

func groupTitlesFromRead(obs []Observation) []string {
	seen := map[string]bool{}
	var titles []string
	for _, hit := range readHits(obs) {
		name := catalogGroupTitle(hit.Title)
		if name == "" || seen[name] {
			continue
		}
		seen[name] = true
		titles = append(titles, name)
	}
	return titles
}

func readHits(obs []Observation) []EvidenceHit {
	var hits []EvidenceHit
	for _, o := range obs {
		if o.Name != "knowledge.read" || len(o.Output) == 0 || o.Err != nil {
			continue
		}
		var payload struct {
			Hits []EvidenceHit `json:"hits"`
		}
		if json.Unmarshal(o.Output, &payload) != nil {
			continue
		}
		hits = append(hits, payload.Hits...)
	}
	return hits
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
