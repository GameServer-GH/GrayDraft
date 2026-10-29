package agent

import (
	"context"
	"testing"
)

func TestReasoningSinkRoundTrip(t *testing.T) {
	var got []string
	ctx := WithReasoningSink(context.Background(), func(d string) error {
		got = append(got, d)
		return nil
	})
	sink := ReasoningSinkFrom(ctx)
	if sink == nil {
		t.Fatal("expected sink")
	}
	_ = sink("a")
	_ = sink("b")
	if len(got) != 2 || got[0] != "a" || got[1] != "b" {
		t.Fatalf("got=%v", got)
	}
	if ReasoningSinkFrom(context.Background()) != nil {
		t.Fatal("empty ctx should have nil sink")
	}
}
