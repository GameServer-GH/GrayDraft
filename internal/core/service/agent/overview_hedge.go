package agent

import (
	"context"
	"log/slog"
	"sync/atomic"
	"time"
)

// At most one extra request per complete overview and two extras per Core
// process. Saturation never queues a backup or delays the original request.
type overviewHedgeBudget struct{ used atomic.Bool }

var overviewExtraSlots = make(chan struct{}, 2)

type overviewCandidate struct {
	text string
	err  error
}

// The candidate function must buffer and validate its complete output, obey
// cancellation and be safe to call concurrently. Never splice candidates.
func runOverviewWithHedge(ctx context.Context, delay time.Duration, budget *overviewHedgeBudget, generate func(context.Context) (string, error)) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	if delay <= 0 {
		return generate(ctx)
	}
	workCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	results := make(chan overviewCandidate, 2)
	start := func() { text, err := generate(workCtx); results <- overviewCandidate{text, err} }
	go start()
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case result := <-results:
		return result.text, result.err
	case <-ctx.Done():
		return "", ctx.Err()
	case <-timer.C:
	}
	// Prefer an original completion that arrived as the timer fired.
	select {
	case result := <-results:
		return result.text, result.err
	default:
	}
	acquired := false
	select {
	case overviewExtraSlots <- struct{}{}:
		if budget.used.CompareAndSwap(false, true) {
			acquired = true
		} else {
			<-overviewExtraSlots
		}
	default:
	}
	if !acquired {
		select {
		case result := <-results:
			return result.text, result.err
		case <-ctx.Done():
			return "", ctx.Err()
		}
	}
	slog.InfoContext(ctx, "overview hedge started", "after_ms", delay.Milliseconds())
	go func() { defer func() { <-overviewExtraSlots }(); start() }()
	var firstErr error
	for remaining := 2; remaining > 0; remaining-- {
		select {
		case result := <-results:
			if result.err == nil {
				return result.text, nil
			}
			if firstErr == nil {
				firstErr = result.err
			}
		case <-ctx.Done():
			return "", ctx.Err()
		}
	}
	return "", firstErr
}
