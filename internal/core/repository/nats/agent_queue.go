package nats

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/graydraft/core/service/agent"
	natsgo "github.com/nats-io/nats.go"
	"time"
)

const agentRunSubject = "graydraft.agent.run"

type AgentRunLauncher interface {
	Launch(agent.RunRecord, agent.Limits) error
}
type QueueLauncher struct {
	Publisher *Publisher
}

func (q QueueLauncher) Launch(run agent.RunRecord, _ agent.Limits) error {
	if q.Publisher == nil {
		return fmt.Errorf("agent jetstream publisher unavailable")
	}
	return q.Publisher.PublishAgentRun(context.Background(), run)
}
func (p *Publisher) PublishAgentRun(ctx context.Context, run agent.RunRecord) error {
	if p == nil || p.js == nil {
		return fmt.Errorf("agent jetstream unavailable")
	}
	b, err := json.Marshal(run)
	if err != nil {
		return err
	}
	_, err = p.js.Publish(agentRunSubject, b, natsgo.MsgId(run.RunID))
	return err
}
func (p *Publisher) StartAgentConsumer(ctx context.Context, handle func(context.Context, agent.RunRecord) error) (func() error, error) {
	if p == nil || p.js == nil {
		return nil, fmt.Errorf("agent jetstream unavailable")
	}
	sub, err := p.js.QueueSubscribe(agentRunSubject, "graydraft-agent-workers", func(msg *natsgo.Msg) {
		var run agent.RunRecord
		if json.Unmarshal(msg.Data, &run) != nil {
			_ = msg.Term()
			return
		}
		workCtx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
		defer cancel()
		if err := handle(workCtx, run); err != nil {
			// A deadline already burned the run budget. Redelivering it repeats the hang
			// until the chat client times out.
			if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
				_ = msg.Ack()
				return
			}
			if md, e := msg.Metadata(); e == nil && md.NumDelivered >= 5 {
				h := natsgo.Header{}
				h.Set("X-DLQ-Reason", err.Error())
				_, _ = p.js.PublishMsg(&natsgo.Msg{Subject: "graydraft.agent.dlq", Data: msg.Data, Header: h})
				_ = msg.Ack()
				return
			}
			_ = msg.NakWithDelay(2 * time.Second)
			return
		}
		_ = msg.Ack()
	}, natsgo.Durable("graydraft-agent-runs"), natsgo.ManualAck(), natsgo.AckWait(100*time.Second), natsgo.MaxDeliver(5))
	if err != nil {
		return nil, err
	}
	go func() { <-ctx.Done(); _ = sub.Drain() }()
	return sub.Drain, nil
}
