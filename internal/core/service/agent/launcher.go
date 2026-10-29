package agent

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"
)

type ToolExecutorFactory func(RunRecord) ToolExecutor

// Launcher isolates HTTP lifetime from Run lifetime and bounds concurrent Runs
// per tenant. Execution is dispatched through JetStream; ExecuteRun is the
// worker boundary shared by the queue consumer and tests.
type Launcher struct {
	Store        RunStore
	Planner      Planner
	Tools        ToolExecutorFactory
	Memory       func(RunRecord) ShortTermMemory
	LongMemory   func(RunRecord) LongTermMemory
	VectorMemory func(RunRecord) VectorLongTermMemory
	Embedder     func(RunRecord) Embedder
	Streamer     TokenStreamer
	Validator    AnswerValidator
	OnComplete   func(RunRecord, Result, error)
	Budget       GlobalBudgetGuard
	PerTenant    int
	mu           sync.Mutex
	semaphores   map[string]chan struct{}
}

func (l *Launcher) semaphore(tenant string) chan struct{} {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.semaphores == nil {
		l.semaphores = map[string]chan struct{}{}
	}
	if ch := l.semaphores[tenant]; ch != nil {
		return ch
	}
	n := l.PerTenant
	if n <= 0 {
		n = 4
	}
	ch := make(chan struct{}, n)
	l.semaphores[tenant] = ch
	return ch
}

func (l *Launcher) Launch(run RunRecord, limits Limits) error {
	go func() {
		_, _ = l.Run(context.Background(), run, limits)
	}()
	return nil
}

func (l *Launcher) Run(parent context.Context, run RunRecord, limits Limits) (Result, error) {
	if err := l.Budget.Check(parent, run.TenantID, false); err != nil {
		return Result{}, err
	}
	sem := l.semaphore(run.TenantID)
	sem <- struct{}{}
	defer func() { <-sem }()
	deadline := run.Deadline
	if deadline.IsZero() {
		deadline = time.Now().Add(time.Minute)
	}
	ctx, cancel := context.WithDeadline(parent, deadline)
	defer cancel()
	var tools ToolExecutor
	if l.Tools != nil {
		tools = l.Tools(run)
	}
	var memory ShortTermMemory
	if l.Memory != nil && run.ConversationID != "" {
		userPrompt := string(run.Prompt)
		memory = l.Memory(run)
		maxMsgs, maxChars := 20, 24000
		if prompt, err := (ContextBuilder{Memory: memory, MaxMessages: maxMsgs, MaxChars: maxChars}).Build(ctx, run.ConversationID, userPrompt); err == nil {
			run.Prompt = prompt
		}
		if memory != nil {
			_ = memory.Append(ctx, run.ConversationID, MemoryMessage{Role: "user", Content: userPrompt})
		}
	}
	var relational, semantic []MemoryItem
	if l.LongMemory != nil {
		if memories, err := l.LongMemory(run).Search(ctx, run.TenantID, run.ActorID, "", 8); err == nil {
			relational = memories
		}
	}
	if l.VectorMemory != nil && l.Embedder != nil {
		if vec, err := l.Embedder(run).EmbedQuery(ctx, string(run.Prompt)); err == nil {
			if memories, err := l.VectorMemory(run).SearchSimilar(ctx, run.TenantID, run.ActorID, vec, 8); err == nil {
				semantic = memories
			}
		}
	}
	if memories := FuseMemoryResults(relational, semantic, 8); len(memories) > 0 {
		var b strings.Builder
		for _, m := range memories {
			b.WriteString(m.Subject)
			b.WriteString(": ")
			b.WriteString(fmt.Sprint(m.Value))
			b.WriteString("\n")
		}
		run.Prompt = []byte("long_term_memory:\n" + b.String() + "\n" + string(run.Prompt))
	}
	result, err := ExecuteRunWith(ctx, l.Store, run, l.Planner, tools, limits, l.Streamer, l.Validator)
	if l.OnComplete != nil {
		l.OnComplete(run, result, err)
	}
	if memory != nil && run.ConversationID != "" && result.Answer != "" {
		_ = memory.Append(ctx, run.ConversationID, MemoryMessage{Role: "assistant", Content: result.Answer})
		if compactor, ok := memory.(MemoryCompactor); ok {
			_ = compactor.Compact(ctx, run.ConversationID, 20)
		}
	}
	return result, err
}
