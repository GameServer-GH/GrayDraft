package agent

import (
	"context"
	"errors"
	"strings"
	"testing"
)

type stubStreamer struct {
	deltas []string
	err    error
}

func TestAnswerDoesNotCommitPartialOrEmptyGeneration(t *testing.T) {
	for _, tc := range []struct {
		name   string
		deltas []string
		err    error
	}{
		{"partial_timeout", []string{"半截正文"}, context.DeadlineExceeded},
		{"partial_upstream_error", []string{"半截正文"}, errors.New("upstream failed")},
		{"empty", nil, nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			emitted := false
			out, err := StreamFinalAnswer(context.Background(), stubStreamer{deltas: tc.deltas, err: tc.err}, []byte("q"), nil, "旧草稿", func(string) error { emitted = true; return nil })
			if err == nil || out != "" || emitted {
				t.Fatalf("out=%q err=%v emitted=%v", out, err, emitted)
			}
		})
	}
}

func (s stubStreamer) StreamTokens(_ context.Context, _, _ string, onDelta func(string) error) error {
	for _, d := range s.deltas {
		if err := onDelta(d); err != nil {
			return err
		}
	}
	return s.err
}

func TestStreamFinalAnswerEmitsDeltas(t *testing.T) {
	var got []string
	out, err := StreamFinalAnswer(context.Background(), stubStreamer{deltas: []string{"你", "好"}}, []byte("q"), nil, "", func(d string) error {
		got = append(got, d)
		return nil
	})
	if err != nil || out != "你好" || strings.Join(got, "") != "你好" {
		t.Fatalf("out=%q got=%v err=%v", out, got, err)
	}
}

func TestStreamFinalAnswerFallsBackToDraft(t *testing.T) {
	out, err := StreamFinalAnswer(context.Background(), nil, []byte("q"), nil, "草稿答案", nil)
	if err != nil || out != "草稿答案" {
		t.Fatalf("out=%q err=%v", out, err)
	}
}
