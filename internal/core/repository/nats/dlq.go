package nats

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/graydraft/core/model"
	"github.com/nats-io/nats.go"
)

// DLQStats returns JetStream DLQ stream statistics.
func (p *Publisher) DLQStats(ctx context.Context) (*model.DLQStreamStats, error) {
	if err := p.requireJS(); err != nil {
		return nil, err
	}
	stream := p.cfg.NATS.JetStream.DLQStream
	info, err := p.js.StreamInfo(stream)
	if err != nil {
		return nil, fmt.Errorf("dlq stream info: %w", err)
	}
	return &model.DLQStreamStats{
		Stream:   stream,
		Subject:  p.cfg.NATS.JetStream.DLQSubject,
		Messages: info.State.Msgs,
		Bytes:    info.State.Bytes,
		FirstSeq: info.State.FirstSeq,
		LastSeq:  info.State.LastSeq,
	}, nil
}

// ListDLQ returns recent DLQ messages (newest first).
func (p *Publisher) ListDLQ(ctx context.Context, limit int) ([]model.DLQMessage, *model.DLQStreamStats, error) {
	if err := p.requireJS(); err != nil {
		return nil, nil, err
	}
	if limit <= 0 || limit > 100 {
		limit = 50
	}

	stats, err := p.DLQStats(ctx)
	if err != nil {
		return nil, nil, err
	}
	if stats.Messages == 0 || stats.LastSeq == 0 {
		return []model.DLQMessage{}, stats, nil
	}

	stream := p.cfg.NATS.JetStream.DLQStream
	var out []model.DLQMessage
	for seq := stats.LastSeq; seq >= stats.FirstSeq && len(out) < limit; seq-- {
		msg, err := p.js.GetMsg(stream, seq)
		if err != nil {
			if err == nats.ErrMsgNotFound {
				continue
			}
			return nil, stats, fmt.Errorf("get msg seq=%d: %w", seq, err)
		}
		out = append(out, parseDLQMsg(msg))
	}
	return out, stats, nil
}

// ReplayDLQ republishes a DLQ message to the ingest subject and deletes it from DLQ.
func (p *Publisher) ReplayDLQ(ctx context.Context, seq uint64) (taskID string, err error) {
	if err := p.requireJS(); err != nil {
		return "", err
	}
	stream := p.cfg.NATS.JetStream.DLQStream
	raw, taskID, err := p.fetchDLQPayload(stream, seq)
	if err != nil {
		return "", err
	}

	var queue model.UGCQueueMessage
	if err := json.Unmarshal(raw, &queue); err != nil {
		return "", fmt.Errorf("invalid ugc payload: %w", err)
	}
	if queue.TaskID != "" {
		taskID = queue.TaskID
	}

	if err := p.PublishUGC(ctx, queue); err != nil {
		return taskID, fmt.Errorf("republish: %w", err)
	}
	if err := p.js.DeleteMsg(stream, seq); err != nil {
		return taskID, fmt.Errorf("delete dlq msg: %w", err)
	}
	return taskID, nil
}

// DiscardDLQ permanently removes a message from the DLQ stream.
func (p *Publisher) DiscardDLQ(ctx context.Context, seq uint64) error {
	if err := p.requireJS(); err != nil {
		return err
	}
	return p.js.DeleteMsg(p.cfg.NATS.JetStream.DLQStream, seq)
}

// ReplayDLQBatch replays multiple sequences.
func (p *Publisher) ReplayDLQBatch(ctx context.Context, seqs []uint64) []model.DLQReplayResult {
	results := make([]model.DLQReplayResult, 0, len(seqs))
	for _, seq := range seqs {
		taskID, err := p.ReplayDLQ(ctx, seq)
		r := model.DLQReplayResult{Sequence: seq, TaskID: taskID, OK: err == nil}
		if err != nil {
			r.Error = err.Error()
		}
		results = append(results, r)
	}
	return results
}

func (p *Publisher) requireJS() error {
	if p == nil || p.js == nil {
		return fmt.Errorf("jetstream not enabled")
	}
	return nil
}

func (p *Publisher) fetchDLQPayload(stream string, seq uint64) ([]byte, string, error) {
	msg, err := p.js.GetMsg(stream, seq)
	if err != nil {
		return nil, "", fmt.Errorf("get dlq msg: %w", err)
	}
	var queue model.UGCQueueMessage
	_ = json.Unmarshal(msg.Data, &queue)
	return msg.Data, queue.TaskID, nil
}

func parseDLQMsg(msg *nats.RawStreamMsg) model.DLQMessage {
	item := model.DLQMessage{
		Sequence: msg.Sequence,
		Subject:  msg.Subject,
		Time:     msg.Time,
	}
	if msg.Header != nil {
		item.DLQReason = msg.Header.Get("X-DLQ-Reason")
	}
	var queue model.UGCQueueMessage
	if err := json.Unmarshal(msg.Data, &queue); err == nil {
		item.TaskID = queue.TaskID
		item.KBType = queue.KBType
		item.Title = queue.Title
	}
	preview := string(msg.Data)
	if len(preview) > 200 {
		preview = preview[:200] + "…"
	}
	item.PayloadPreview = strings.TrimSpace(preview)
	return item
}
