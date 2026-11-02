package agent

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"
)

func TestOverviewHedgeFastCompletionAndEarlyErrorNeverDuplicate(t *testing.T) {
	for _, failure := range []error{nil, errors.New("upstream failure")} {
		var calls atomic.Int32
		text, err := runOverviewWithHedge(context.Background(), time.Hour, &overviewHedgeBudget{}, func(context.Context) (string, error) { calls.Add(1); return "complete", failure })
		if calls.Load() != 1 || err != failure || text != "complete" {
			t.Fatalf("%q %v calls=%d", text, err, calls.Load())
		}
	}
}

func TestOverviewHedgeSlowProgressStillStartsBackupAndCancelsLoser(t *testing.T) {
	var calls atomic.Int32
	cancelled := make(chan struct{})
	text, err := runOverviewWithHedge(context.Background(), time.Millisecond, &overviewHedgeBudget{}, func(ctx context.Context) (string, error) {
		if calls.Add(1) == 1 {
			<-ctx.Done()
			close(cancelled)
			return "partial", ctx.Err()
		}
		return "validated backup", nil
	})
	if err != nil || text != "validated backup" || calls.Load() != 2 {
		t.Fatalf("%q %v calls=%d", text, err, calls.Load())
	}
	select {
	case <-cancelled:
	case <-time.After(time.Second):
		t.Fatal("loser not cancelled")
	}
}

func TestOverviewHedgeInvalidCandidateCannotWin(t *testing.T) {
	var calls atomic.Int32
	backupStarted := make(chan struct{})
	text, err := runOverviewWithHedge(context.Background(), time.Millisecond, &overviewHedgeBudget{}, func(ctx context.Context) (string, error) {
		if calls.Add(1) == 1 {
			<-backupStarted
			return "incomplete names", errors.New("validation failure")
		}
		close(backupStarted)
		return "validated complete summary", nil
	})
	if err != nil || text != "validated complete summary" {
		t.Fatalf("%q %v", text, err)
	}
}

func TestOverviewHedgeBudgetAllowsOnlyOneExtraPerOverview(t *testing.T) {
	budget := &overviewHedgeBudget{}
	var calls atomic.Int32
	cancelled := make(chan struct{})
	_, err := runOverviewWithHedge(context.Background(), time.Millisecond, budget, func(ctx context.Context) (string, error) {
		if calls.Add(1) == 1 {
			<-ctx.Done()
			close(cancelled)
			return "", ctx.Err()
		}
		return "complete", nil
	})
	if err != nil {
		t.Fatal(err)
	}
	<-cancelled
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		defer close(done)
		runOverviewWithHedge(ctx, time.Millisecond, budget, func(ctx context.Context) (string, error) { calls.Add(1); <-ctx.Done(); return "", ctx.Err() })
	}()
	// A used budget cannot launch a second extra, even after the first releases.
	time.AfterFunc(20*time.Millisecond, cancel)
	<-done
	if calls.Load() != 3 {
		t.Fatalf("calls=%d", calls.Load())
	}
}

func TestOverviewHedgeSaturationSkipsBackupAndParentCancellationStopsWait(t *testing.T) {
	// Wait for any cancelled backup from earlier tests to release its slot.
	deadline := time.Now().Add(time.Second)
	for len(overviewExtraSlots) > 0 && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	for i := 0; i < cap(overviewExtraSlots); i++ {
		overviewExtraSlots <- struct{}{}
	}
	defer func() {
		for i := 0; i < cap(overviewExtraSlots); i++ {
			<-overviewExtraSlots
		}
	}()
	ctx, cancel := context.WithCancel(context.Background())
	time.AfterFunc(20*time.Millisecond, cancel)
	var calls atomic.Int32
	_, err := runOverviewWithHedge(ctx, time.Millisecond, &overviewHedgeBudget{}, func(ctx context.Context) (string, error) { calls.Add(1); <-ctx.Done(); return "", ctx.Err() })
	if !errors.Is(err, context.Canceled) || calls.Load() != 1 {
		t.Fatalf("%v calls=%d", err, calls.Load())
	}
}
