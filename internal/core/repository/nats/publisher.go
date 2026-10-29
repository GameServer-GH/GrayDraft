package nats

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/graydraft/core/config"
	"github.com/graydraft/core/model"
	"github.com/graydraft/core/observability/trace"
	"github.com/nats-io/nats.go"
)

// Publisher publishes UGC messages to NATS (JetStream when enabled).
type Publisher struct {
	conn    *nats.Conn
	js      nats.JetStreamContext
	subject string
	cfg     *config.Config
}

// NewPublisher connects to NATS and optionally bootstraps JetStream streams.
func NewPublisher(cfg *config.Config) (*Publisher, error) {
	if !cfg.NATS.Enabled || cfg.NATS.URL == "" {
		return nil, nil
	}
	nc, err := nats.Connect(
		cfg.NATS.URL,
		nats.Name("graydraft-core"),
		nats.Timeout(time.Duration(cfg.NATS.ConnectTimeoutSeconds)*time.Second),
		nats.RetryOnFailedConnect(true),
		nats.MaxReconnects(10),
	)
	if err != nil {
		return nil, fmt.Errorf("nats connect: %w", err)
	}

	p := &Publisher{
		conn:    nc,
		subject: cfg.NATS.UGCSubject,
		cfg:     cfg,
	}

	if cfg.NATS.JetStream.Enabled {
		js, err := nc.JetStream()
		if err != nil {
			nc.Close()
			return nil, fmt.Errorf("jetstream: %w", err)
		}
		if err := ensureStreams(js, cfg); err != nil {
			nc.Close()
			return nil, err
		}
		p.js = js
	}

	return p, nil
}

// PublishUGC publishes an ingest message (JetStream with dedup by task_id when enabled).
func (p *Publisher) PublishUGC(ctx context.Context, msg model.UGCQueueMessage) error {
	if p == nil || p.conn == nil {
		return fmt.Errorf("nats publisher not configured")
	}
	data, err := json.Marshal(msg)
	if err != nil {
		return err
	}

	hdr := nats.Header{}
	if msg.TaskID != "" {
		hdr.Set(trace.HeaderTaskID, msg.TaskID)
	}
	if msg.TraceID != "" {
		hdr.Set(trace.HeaderTraceID, msg.TraceID)
		hdr.Set(trace.HeaderRequestID, msg.TraceID)
	}

	if p.js != nil {
		opts := []nats.PubOpt{}
		if msg.TaskID != "" {
			opts = append(opts, nats.MsgId(msg.TaskID))
		}
		_, err = p.js.PublishMsg(&nats.Msg{
			Subject: p.subject,
			Data:    data,
			Header:  hdr,
		}, opts...)
		return err
	}

	if len(hdr) > 0 {
		return p.conn.PublishMsg(&nats.Msg{Subject: p.subject, Data: data, Header: hdr})
	}
	return p.conn.Publish(p.subject, data)
}

// JetStreamEnabled reports whether publishes are persisted in JetStream.
func (p *Publisher) JetStreamEnabled() bool {
	return p != nil && p.js != nil
}

// Ping checks connectivity (and JetStream when enabled).
func (p *Publisher) Ping() error {
	if p == nil || p.conn == nil {
		return fmt.Errorf("nats not configured")
	}
	if !p.conn.IsConnected() {
		return fmt.Errorf("nats disconnected")
	}
	if err := p.conn.FlushTimeout(2 * time.Second); err != nil {
		return err
	}
	if p.js != nil {
		if _, err := p.js.AccountInfo(); err != nil {
			return fmt.Errorf("jetstream: %w", err)
		}
	}
	return nil
}

// Close drains the connection.
func (p *Publisher) Close() {
	if p != nil && p.conn != nil {
		p.conn.Drain()
	}
}

// StageStreamsEnabled reports whether the staged pipeline subjects are configured.
func (p *Publisher) StageStreamsEnabled() bool {
	return p != nil && p.cfg != nil && p.cfg.NATS.JetStream.StageStreamsEnabled
}

// StageSubject returns the JetStream subject for a given ingest stage.
func (p *Publisher) StageSubject(stage model.IngestStage) (string, error) {
	if p == nil || p.cfg == nil {
		return "", fmt.Errorf("publisher not configured")
	}
	switch stage {
	case model.StageAccepted:
		return p.cfg.NATS.StageSubjects.Accepted, nil
	case model.StageParsed:
		return p.cfg.NATS.StageSubjects.Parsed, nil
	case model.StageMediaExtracted:
		return p.cfg.NATS.StageSubjects.MediaExtracted, nil
	case model.StageChunked:
		return p.cfg.NATS.StageSubjects.Chunked, nil
	case model.StageEmbedded:
		return p.cfg.NATS.StageSubjects.Embedded, nil
	case model.StageIndexed:
		return p.cfg.NATS.StageSubjects.Indexed, nil
	case model.StageQA:
		return p.cfg.NATS.StageSubjects.QA, nil
	default:
		return "", fmt.Errorf("unknown stage: %s", stage)
	}
}

// PublishStageMessage publishes a raw stage message to a JetStream subject with dedup headers.
func (p *Publisher) PublishStageMessage(ctx context.Context, subject, taskID, traceID string, data []byte) error {
	if p == nil || p.conn == nil {
		return fmt.Errorf("nats publisher not configured")
	}
	hdr := nats.Header{}
	if taskID != "" {
		hdr.Set(trace.HeaderTaskID, taskID)
	}
	if traceID != "" {
		hdr.Set(trace.HeaderTraceID, traceID)
		hdr.Set(trace.HeaderRequestID, traceID)
	}
	if p.js != nil {
		opts := []nats.PubOpt{}
		if taskID != "" {
			opts = append(opts, nats.MsgId(taskID))
		}
		_, err := p.js.PublishMsg(&nats.Msg{
			Subject: subject,
			Data:    data,
			Header:  hdr,
		}, opts...)
		return err
	}
	if len(hdr) > 0 {
		return p.conn.PublishMsg(&nats.Msg{Subject: subject, Data: data, Header: hdr})
	}
	return p.conn.Publish(subject, data)
}
