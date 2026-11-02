package nats

import (
	"context"
	"errors"
	"fmt"
	"testing"
)

type completionFailure struct{}

func (completionFailure) Error() string            { return "completion interrupted" }
func (completionFailure) TerminalRunFailure() bool { return true }

func TestCompletionFailuresAreNotRedeliveredAsTheSameRun(t *testing.T) {
	for _, err := range []error{context.DeadlineExceeded, context.Canceled,
		fmt.Errorf("overview: %w", completionFailure{}), completionFailure{},
	} {
		if !finalAgentRunError(err) {
			t.Fatal(err)
		}
	}
	if finalAgentRunError(errors.New("temporary database unavailable")) {
		t.Fatal("infrastructure failures should retain existing retry policy")
	}
}
