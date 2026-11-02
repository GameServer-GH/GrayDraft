package agent

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

type overviewStreamer struct {
	calls     []string
	systems   []string
	responses []string
	err       error
}

type tieredOverviewStreamer struct {
	fast, complex atomic.Int32
	streamErr     error
}

func (s *tieredOverviewStreamer) StreamTokens(context.Context, string, string, func(string) error) error {
	return errors.New("unexpected unbounded generation")
}
func (s *tieredOverviewStreamer) StreamOverviewBounded(_ context.Context, _, user string, _ int, emit func(string) error) error {
	s.fast.Add(1)
	if strings.Contains(user, "Vision-22B") {
		return errors.New("identity evidence incorrectly routed to fast")
	}
	if s.streamErr != nil {
		return s.streamErr
	}
	return emit("该研究介绍语言模型的预训练与对齐方法。")
}
func (s *tieredOverviewStreamer) StreamAnswerBounded(_ context.Context, _, user string, _ int, emit func(string) error) error {
	s.complex.Add(1)
	return emit("该研究提出视觉模型 Vision-6B，可作为 Vision-22B 的替代方案。")
}
func TestOverviewIdentityEvidenceKeepsComplexModel(t *testing.T) {
	s := &tieredOverviewStreamer{}
	var published []string
	out, err := StreamFinalAnswer(context.Background(), s, []byte("两篇各自讲什么"), overviewObservations(t), "", func(delta string) error { published = append(published, delta); return nil })
	if err != nil || s.fast.Load() != 1 || s.complex.Load() != 1 || !strings.Contains(out, "Vision-22B") || len(published) != 1 || published[0] != out {
		t.Fatalf("out=%q err=%v fast=%d complex=%d published=%v", out, err, s.fast.Load(), s.complex.Load(), published)
	}
}
func TestOverviewDoesNotRetryTransportFailure(t *testing.T) {
	s := &tieredOverviewStreamer{streamErr: errors.New("upstream failure")}
	_, err := StreamFinalAnswer(context.Background(), s, []byte("两篇各自讲什么"), overviewObservations(t), "", nil)
	if err == nil || s.fast.Load() != 1 || s.complex.Load() > 1 {
		t.Fatalf("err=%v fast=%d complex=%d", err, s.fast.Load(), s.complex.Load())
	}
}

func TestOverviewEvidencePrioritizesAnchorsWithoutDroppingContext(t *testing.T) {
	hits := []EvidenceHit{
		{ChunkID: "abstract", Content: "The vision model has six billion parameters and several benchmarks."},
		{ChunkID: "caption", Content: "The vision foundation model (i.e., Vision-6B) aligns with language models."},
		{ChunkID: "comparison", Content: "It can be an alternative to Vision-22B. Full comparison sentence."},
	}
	selected := prioritizeOverviewEvidence(hits, []string{"Vision-6B", "Vision-22B"})
	if len(selected) != 3 || selected[0] != hits[1] || selected[1] != hits[2] || selected[2] != hits[0] || len(hits) != 3 {
		t.Fatalf("%+v", selected)
	}
	if len(prioritizeOverviewEvidence(hits, nil)) != 3 || len(prioritizeOverviewEvidence(hits, []string{"missing"})) != 3 {
		t.Fatal("incomplete constraints dropped source text")
	}
}

type parallelOverviewStreamer struct {
	started chan string
	release chan struct{}
}

func (s *parallelOverviewStreamer) StreamTokens(context.Context, string, string, func(string) error) error {
	return errors.New("must use bounded answer method")
}

func (s *parallelOverviewStreamer) StreamAnswerBounded(ctx context.Context, _, user string, maxTokens int, emit func(string) error) error {
	if maxTokens != 600 {
		return errors.New("missing output budget")
	}
	s.started <- user
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-s.release:
	}
	if strings.Contains(user, "Vision-22B") {
		return emit("该研究提出视觉模型 Vision-6B，可作为 Vision-22B 的替代方案。")
	}
	return emit("该研究介绍语言模型的预训练与对齐方法。")
}

func TestBoundedOverviewRunsIndependentlyAndPreservesWorkOrder(t *testing.T) {
	s := &parallelOverviewStreamer{started: make(chan string, 2), release: make(chan struct{})}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	obs := overviewObservations(t)
	type result struct {
		text string
		err  error
	}
	done := make(chan result, 1)
	go func() {
		text, err := StreamFinalAnswer(ctx, s, []byte("各自讲什么"), obs, "", nil)
		done <- result{text, err}
	}()
	for i := 0; i < 2; i++ {
		select {
		case <-s.started:
		case <-ctx.Done():
			t.Fatal("independent calls did not start concurrently")
		}
	}
	close(s.release)
	r := <-done
	if r.err != nil || strings.Index(r.text, "**视觉研究**") > strings.Index(r.text, "**语言研究**") {
		t.Fatalf("%q %v", r.text, r.err)
	}
}

func (s *overviewStreamer) StreamTokens(_ context.Context, system, user string, emit func(string) error) error {
	s.calls = append(s.calls, user)
	s.systems = append(s.systems, system)
	i := len(s.calls) - 1
	if i < len(s.responses) {
		if err := emit(s.responses[i]); err != nil {
			return err
		}
	}
	return s.err
}

func overviewObservations(t *testing.T) []Observation {
	t.Helper()
	data, err := json.Marshal(map[string]any{"hits": []EvidenceHit{
		{ChunkID: "one", Title: "视觉研究（1/3）", Content: "The model Vision-6B is an alternative to Vision-22B."},
		{ChunkID: "two", Title: "语言研究（1/2）", Content: "The language model uses pretraining and alignment."},
	}})
	if err != nil {
		t.Fatal(err)
	}
	return []Observation{{Name: "knowledge.read", Output: data}}
}

func TestOverviewIsolatesWorksAndEmitsOnlyValidatedSummaries(t *testing.T) {
	s := &overviewStreamer{responses: []string{"该研究提出视觉模型 Vision-6B，可作为 Vision-22B 的替代方案。", "该研究介绍语言模型的预训练与对齐方法。"}}
	var emitted string
	out, err := StreamFinalAnswer(context.Background(), s, []byte("两篇各自讲什么"), overviewObservations(t), "", func(d string) error { emitted += d; return nil })
	if err != nil {
		t.Fatal(err)
	}
	if emitted != out || !strings.Contains(out, "**视觉研究**") || !strings.Contains(out, "**语言研究**") {
		t.Fatalf("%q / %q", out, emitted)
	}
	if len(s.calls) != 2 || strings.Contains(s.calls[0], "language model") || strings.Contains(s.calls[1], "Vision-22B") {
		t.Fatalf("cross-work evidence: %v", s.calls)
	}
	if !strings.Contains(s.systems[0], "完整型号：Vision-6B、Vision-22B") || !strings.Contains(s.systems[0], "不能仅用参数量") || strings.Contains(s.systems[1], "Vision-22B") {
		t.Fatalf("misplaced identity constraints: %v", s.systems)
	}
}

func TestOverviewMissingFactDoesNotPassJustBecauseTitleExists(t *testing.T) {
	s := &overviewStreamer{responses: []string{"视觉研究提出视觉模型 Vision-6B，用于多模态理解。"}}
	emitted := false
	out, err := StreamFinalAnswer(context.Background(), s, []byte("各自讲什么"), overviewObservations(t), "", func(string) error { emitted = true; return nil })
	if err == nil || !strings.Contains(err.Error(), "Vision-22B") || emitted || out != "" || len(s.calls) != 1 {
		t.Fatalf("out=%q err=%v emitted=%v calls=%d", out, err, emitted, len(s.calls))
	}
}

func TestOverviewInterruptedStreamDoesNotEmitOrContinue(t *testing.T) {
	s := &overviewStreamer{responses: []string{"该研究提出视觉模型 Vision-6B，可作为 Vision-22B 的替代方案。"}, err: context.DeadlineExceeded}
	emitted := false
	out, err := StreamFinalAnswer(context.Background(), s, []byte("各自讲什么"), overviewObservations(t), "", func(string) error { emitted = true; return nil })
	if !errors.Is(err, context.DeadlineExceeded) || emitted || out != "" || len(s.calls) != 1 {
		t.Fatalf("out=%q err=%v emitted=%v", out, err, emitted)
	}
}

func TestOverviewNamesOnlyComeFromContent(t *testing.T) {
	names := overviewModelNames([]EvidenceHit{{ChunkID: "source-22", Title: "title-33", Content: "state-of-the-art Vision-6B replaces Vision-22B; Vision-6B. GPT-4 appears in background."}})
	if strings.Join(names, ",") != "Vision-6B,Vision-22B" {
		t.Fatal(names)
	}
	for _, text := range []string{"", "The original English introduction.", "中文摘要在这里突然结束", "这里是一段中文概述。\n## Introduction"} {
		if validateOverviewSummary(text, nil) == nil {
			t.Fatalf("accepted %q", text)
		}
	}
}

func TestOverviewDetectsCatalogWorkWithoutReadEvidence(t *testing.T) {
	obs := overviewObservations(t)
	obs = append(obs, Observation{Name: "knowledge.catalog", Output: json.RawMessage(`{"documents":[{"title":"视觉研究（1/3）"},{"title":"语言研究（1/2）"},{"title":"第三份材料"}]}`)})
	s := &overviewStreamer{responses: []string{"该研究提出视觉模型 Vision-6B，可作为 Vision-22B 的替代方案。", "该研究介绍语言模型的预训练与对齐方法。"}}
	emitted := false
	answer, err := StreamFinalAnswer(context.Background(), s, []byte("三篇各自讲什么"), obs, "", func(string) error { emitted = true; return nil })
	if err == nil || !strings.Contains(err.Error(), "no evidence") || answer != "" || emitted {
		t.Fatalf("%q %v emitted=%v", answer, err, emitted)
	}
}

func TestOverviewParagraphNormalizationOnlyChangesWhitespace(t *testing.T) {
	prose := "研究将 Vision-6B 与语言模型对齐。\n\n可以作为 Vision-22B 的替代方案。"
	normalized := normalizeOverviewParagraph(prose)
	if normalized != "研究将 Vision-6B 与语言模型对齐。 可以作为 Vision-22B 的替代方案。" {
		t.Fatal(normalized)
	}
	if err := validateOverviewSummary(normalized, []string{"Vision-6B", "Vision-22B"}); err != nil {
		t.Fatal(err)
	}
	outline := "- 研究将 Vision-6B 与语言模型对齐。\n- 可以作为 Vision-22B 的替代方案。"
	if normalizeOverviewParagraph(outline) != outline {
		t.Fatal("outline was silently accepted")
	}
	if err := validateOverviewSummary(normalizeOverviewParagraph(outline), nil); err == nil {
		t.Fatal("outline accepted")
	}
}
