package agent

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
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

// StreamFinalAnswer generates the user-facing answer via token streaming.
// Each content delta is forwarded to emit optimistically as it arrives so the
// user watches the answer type out. A failed or interrupted stream still
// returns an error (never a committed answer); already-emitted partial text is
// superseded by the authoritative run.completed message, which may also carry
// the answer gate's safe replacement. Drafts are used only when no streamer is
// configured. Model reasoning (if any) is forwarded via ReasoningSinkFrom(ctx).
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
不要编造证据中没有的数值、参数或版本。证据不足时说明当前证据未覆盖这部分内容。
truncated或omitted_hits表示上下文经过截取，不能据此断言整个工作区没有这部分资料。
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
		// 乐观逐 token 转发；失败/被门禁替换时由 run.completed 的权威答案覆盖。
		if emit != nil {
			if err := emit(delta); err != nil {
				return err
			}
		}
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
	return out, nil
}

func buildAnswerStreamPrompt(prompt []byte, obs []Observation, draft string) string {
	var b strings.Builder
	b.WriteString("用户问题：\n")
	b.WriteString(CurrentUserQuery(string(prompt)))
	b.WriteString("\n\n证据：\n")
	if len(obs) == 0 {
		b.WriteString("（无工具观测）\n")
	}
	for i, o := range obs {
		if o.Err != nil || len(o.Output) == 0 {
			continue
		}
		limit := 2400
		if strings.HasPrefix(o.Name, "knowledge.") {
			limit = 12000
		}
		if !strings.HasPrefix(o.Name, "knowledge.") {
			b.WriteString("辅助上下文（不是用户问题或可引用事实证据）：\n")
		}
		fmt.Fprintf(&b, "[%d] tool=%s\n%s\n\n", i+1, o.Name, answerObservation(o, limit))
	}
	if d := strings.TrimSpace(draft); d != "" {
		b.WriteString("可选提纲（不要照抄，请依据证据重写完整答案）：\n")
		b.WriteString(d)
		b.WriteString("\n")
	}
	b.WriteString("\n请直接输出最终答案，不要输出 JSON。证据不足时明确说明本工作区资料没有。")

	return b.String()
}

// Budget evidence contents individually instead of cutting serialized JSON in
// the middle of a hit. Retain source diversity before additional same-doc hits.
func answerObservation(o Observation, limit int) string {
	if o.AnswerPrepared {
		return string(o.Output)
	}
	var payload knowledgeSearchPayload
	if !strings.HasPrefix(o.Name, "knowledge.") || json.Unmarshal(o.Output, &payload) != nil || len(payload.Hits) == 0 {
		return truncateRunes(string(o.Output), limit)
	}

	ordered := make([]EvidenceHit, 0, len(payload.Hits))
	var rest []EvidenceHit
	seen := map[string]bool{}
	for _, h := range payload.Hits {
		key := h.DocID
		if key == "" {
			key = h.Title
		}
		if seen[key] {
			rest = append(rest, h)
			continue
		}
		seen[key] = true
		ordered = append(ordered, h)
	}
	ordered = append(ordered, rest...)
	if len(ordered) > 12 {
		ordered = ordered[:12]
	}
	budget := limit / len(ordered)
	for {
		hits := make([]EvidenceHit, 0, len(ordered))
		for _, h := range ordered {
			original := h.Content
			sum := sha256.Sum256([]byte(original))
			h.OriginalContentSHA256 = hex.EncodeToString(sum[:])
			h.ContentStartRune = 0
			h.ContentEndRune = len([]rune(original))
			if h.ContentEndRune > budget {
				h.ContentEndRune = budget
				h.Truncated = true
			}
			h.Content = truncateRunes(original, budget)
			h.Title = truncateRunes(h.Title, 200)
			h.Source = truncateRunes(h.Source, 200)
			h.MediaURL = truncateRunes(h.MediaURL, 512)
			hits = append(hits, h)
		}
		copyPayload := payload
		copyPayload.Hits = hits
		body, _ := json.Marshal(struct {
			knowledgeSearchPayload
			Omitted int `json:"omitted_hits,omitempty"`
		}{copyPayload, len(payload.Hits) - len(hits)})
		over := len([]rune(string(body))) - limit
		if over <= 0 {
			return string(body)
		}
		if budget == 0 && len(ordered) > 1 {
			ordered = ordered[:len(ordered)-1]
			budget = limit / len(ordered)
			continue
		}
		if budget == 0 {
			return string(body)
		}
		budget -= (over + len(hits) - 1) / len(hits)
		if budget < 0 {
			budget = 0
		}
	}
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
