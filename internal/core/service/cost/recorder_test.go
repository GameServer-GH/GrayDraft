package cost

import (
	"context"
	"testing"
)

func TestEstimateTokens(t *testing.T) {
	if got := EstimateTokens(""); got != 0 {
		t.Fatalf("empty: got %d", got)
	}
	if got := EstimateTokens("abcd"); got != 1 {
		t.Fatalf("abcd: got %d", got)
	}
	if got := EstimateTokens("abcdefghi"); got != 3 {
		t.Fatalf("9 runes: got %d", got)
	}
}

func TestFromContextScope(t *testing.T) {
	ctx := WithScope(context.Background(), Scope{TenantID: "tn_a", AgentID: "ag_1"})
	s := FromContext(ctx)
	if s.TenantID != "tn_a" || s.AgentID != "ag_1" {
		t.Fatalf("unexpected scope: %+v", s)
	}
}
