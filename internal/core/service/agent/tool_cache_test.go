package agent

import (
	"context"
	"testing"
	"time"
)

type toolStoreStub struct{ calls int }

func (s *toolStoreStub) ActiveForTenant(context.Context, string) ([]ToolSpec, error) {
	s.calls++
	return []ToolSpec{{Name: "knowledge.lexical", Version: "1"}}, nil
}
func TestTenantToolCache(t *testing.T) {
	s := &toolStoreStub{}
	c := NewTenantToolCache(s, time.Minute)
	_, _ = c.ForTenant(context.Background(), "t")
	_, _ = c.ForTenant(context.Background(), "t")
	if s.calls != 1 {
		t.Fatalf("calls=%d", s.calls)
	}
	c.Invalidate("t")
	_, _ = c.ForTenant(context.Background(), "t")
	if s.calls != 2 {
		t.Fatalf("calls=%d", s.calls)
	}
}
