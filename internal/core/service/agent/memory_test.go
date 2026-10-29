package agent

import (
	"context"
	"testing"
)

type memoryStub struct{ msgs []MemoryMessage }

func (s memoryStub) Append(context.Context, string, MemoryMessage) error          { return nil }
func (s memoryStub) Recent(context.Context, string, int) ([]MemoryMessage, error) { return s.msgs, nil }
func TestContextBuilder(t *testing.T) {
	b := ContextBuilder{Memory: memoryStub{msgs: []MemoryMessage{{Role: "user", Content: "上一个问题"}}}}
	out, err := b.Build(context.Background(), "c1", "现在呢")
	if err != nil || string(out) != "user: 上一个问题\nuser: 现在呢" {
		t.Fatalf("%q %v", out, err)
	}
}
