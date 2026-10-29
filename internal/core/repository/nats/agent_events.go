package nats

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/nats-io/nats.go"
)

const agentRunNotifyPrefix = "graydraft.agent.run.notify."

// RunEventNotifier publishes lightweight wakeups for SSE subscribers.
type RunEventNotifier struct {
	conn *nats.Conn
}

// NewRunEventNotifier creates a notifier when core NATS is connected.
func NewRunEventNotifier(p *Publisher) *RunEventNotifier {
	if p == nil || p.conn == nil {
		return nil
	}
	return &RunEventNotifier{conn: p.conn}
}

// NotifyRunEvent signals that a new event seq is available for run_id.
func (n *RunEventNotifier) NotifyRunEvent(_ context.Context, _, runID string, seq int64) error {
	if n == nil || n.conn == nil || runID == "" {
		return nil
	}
	data, err := json.Marshal(map[string]any{"run_id": runID, "seq": seq})
	if err != nil {
		return err
	}
	return n.conn.Publish(agentRunNotifyPrefix+runID, data)
}

// SubscribeRunEvents listens for new event notifications for one run.
func (n *RunEventNotifier) SubscribeRunEvents(runID string, onSeq func(int64)) (func(), error) {
	if n == nil || n.conn == nil {
		return nil, fmt.Errorf("nats notifier unavailable")
	}
	sub, err := n.conn.Subscribe(agentRunNotifyPrefix+runID, func(msg *nats.Msg) {
		var payload struct {
			Seq int64 `json:"seq"`
		}
		if json.Unmarshal(msg.Data, &payload) == nil && payload.Seq > 0 {
			onSeq(payload.Seq)
		}
	})
	if err != nil {
		return nil, err
	}
	return func() { _ = sub.Unsubscribe() }, nil
}

// Connected reports whether live SSE wakeups are available.
func (n *RunEventNotifier) Connected() bool {
	return n != nil && n.conn != nil && n.conn.IsConnected()
}
