package observability

import (
	"context"
	"log/slog"
	"time"

	natspub "github.com/graydraft/core/repository/nats"
)

// RunDLQCollector periodically updates graydraft_dlq_messages from JetStream.
func RunDLQCollector(ctx context.Context, pub *natspub.Publisher, interval time.Duration) {
	if !enabled || pub == nil {
		return
	}
	if interval <= 0 {
		interval = 30 * time.Second
	}
	tick := func() {
		stats, err := pub.DLQStats(ctx)
		if err != nil {
			slog.Debug("dlq metrics poll failed", "error", err)
			return
		}
		SetDLQMessages(float64(stats.Messages))
	}
	tick()
	t := time.NewTicker(interval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			tick()
		}
	}
}
