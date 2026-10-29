package model

import "time"

// DLQStreamStats is JetStream DLQ stream summary.
type DLQStreamStats struct {
	Stream  string `json:"stream"`
	Subject string `json:"subject"`
	Messages uint64 `json:"messages"`
	Bytes    uint64 `json:"bytes"`
	FirstSeq uint64 `json:"first_seq"`
	LastSeq  uint64 `json:"last_seq"`
}

// DLQMessage is one message in the DLQ stream.
type DLQMessage struct {
	Sequence       uint64    `json:"sequence"`
	Subject        string    `json:"subject"`
	DLQReason      string    `json:"dlq_reason,omitempty"`
	TaskID         string    `json:"task_id,omitempty"`
	KBType         string    `json:"kb_type,omitempty"`
	Title          string    `json:"title,omitempty"`
	Time           time.Time `json:"time"`
	PayloadPreview string    `json:"payload_preview,omitempty"`
}

// DLQReplayRequest replays one or more DLQ messages to the ingest subject.
type DLQReplayRequest struct {
	Sequences []uint64 `json:"sequences"`
}

// DLQReplayResult is the outcome for a single sequence.
type DLQReplayResult struct {
	Sequence uint64 `json:"sequence"`
	TaskID   string `json:"task_id,omitempty"`
	OK       bool   `json:"ok"`
	Error    string `json:"error,omitempty"`
}

// DLQDiscardRequest removes messages from the DLQ stream.
type DLQDiscardRequest struct {
	Sequences []uint64 `json:"sequences"`
}
