// Package cache implements fine-grained cache invalidation (V1-16).
package cache

import (
	"context"
	"encoding/json"
	"log/slog"

	"github.com/graydraft/core/model"
	natspub "github.com/graydraft/core/repository/nats"
)

// InvalidationListener publishes and subscribes to fine-grained cache invalidation events.
type InvalidationListener struct {
	nats    *natspub.Publisher
	mgrs    []*Manager
	subject string
}

// NewInvalidationListener wires invalidation NATS publication.
func NewInvalidationListener(
	nats *natspub.Publisher,
	mgrs []*Manager,
	subject string,
) *InvalidationListener {
	return &InvalidationListener{
		nats:    nats,
		mgrs:    mgrs,
		subject: subject,
	}
}

// Start begins listening for invalidation events (best-effort).
// Currently relies on the UGC service's purgeCaches on import; this is a forward-looking hook.
func (l *InvalidationListener) Start(ctx context.Context) {
	if l.nats == nil || l.subject == "" {
		return
	}
	if !l.nats.JetStreamEnabled() {
		slog.Info("cache invalidation: JetStream not available, using full purge on import")
		return
	}

	slog.Info("cache invalidation listener starting", "subject", l.subject)
	go l.listenLoop(ctx)
}

// PublishInvalidation sends a fine-grained invalidation event to JetStream.
func (l *InvalidationListener) PublishInvalidation(ctx context.Context, ev model.CacheInvalidationEvent) error {
	if l.nats == nil || l.subject == "" || !l.nats.JetStreamEnabled() {
		return nil
	}
	data, err := json.Marshal(ev)
	if err != nil {
		return err
	}
	return l.nats.PublishStageMessage(ctx, l.subject, ev.TaskID, ev.TraceID, data)
}

func (l *InvalidationListener) listenLoop(ctx context.Context) {
	slog.Info("cache invalidation listener: events published via JetStream, full purge on import")
	<-ctx.Done()
}
