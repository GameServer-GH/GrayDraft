package agent

import (
	"testing"

	"github.com/google/uuid"
)

func TestNormalizeConversationID(t *testing.T) {
	t.Parallel()
	if got := NormalizeConversationID(""); got != "" {
		t.Fatalf("empty -> %q", got)
	}
	valid := "550e8400-e29b-41d4-a716-446655440000"
	if got := NormalizeConversationID(valid); got != valid {
		t.Fatalf("uuid passthrough: got %q", got)
	}
	opaque := "1784256924551"
	got := NormalizeConversationID(opaque)
	if _, err := uuid.Parse(got); err != nil {
		t.Fatalf("opaque not uuid: %q err=%v", got, err)
	}
	if again := NormalizeConversationID(opaque); again != got {
		t.Fatalf("not deterministic: %q vs %q", got, again)
	}
	if got == opaque {
		t.Fatalf("opaque should be remapped, still %q", got)
	}
}
