// Package ugc implements the staged ingest pipeline (V1-07, V1-08).
package ugc

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/graydraft/core/model"
	natspub "github.com/graydraft/core/repository/nats"
	"github.com/graydraft/core/repository/postgres"
	"github.com/graydraft/core/observability/trace"
)

// StageMessage is the payload published between stages.
// It carries the original UGC payload plus pipeline metadata and stage-specific results.
type StageMessage struct {
	TraceID    string `json:"trace_id"`
	TaskID     string `json:"task_id"`
	Stage      string `json:"stage"`
	RetryCount int    `json:"retry_count"`
	DeadlineAt string `json:"deadline_at,omitempty"` // RFC3339

	// Original UGC fields.
	DocID      string         `json:"doc_id,omitempty"`
	KBType     string         `json:"kb_type"`
	Title      string         `json:"title"`
	Content    string         `json:"content"`
	SourceURL  string         `json:"source_url,omitempty"`
	MimeType   string         `json:"mime_type"`
	GenerateQA bool           `json:"generate_qa"`
	Tags       []string       `json:"tags,omitempty"`
	Extensions map[string]any `json:"extensions,omitempty"`

	// Stage-specific payloads.
	ParsedText   string `json:"parsed_text,omitempty"`
	ParsedTitle  string `json:"parsed_title,omitempty"`
	MediaManifest string `json:"media_manifest,omitempty"`     // JSON list of media descriptors
	ChunkPayloadJSON string `json:"chunk_payload,omitempty"`   // serialized chunk list
	EmbedPayloadJSON string `json:"embed_payload,omitempty"`   // serialized embeddings
	IndexResultJSON  string `json:"index_result,omitempty"`    // serialized index result
	QAResultJSON     string `json:"qa_result,omitempty"`       // serialized QA result

	// Idempotency.
	ContentHash    string `json:"content_hash,omitempty"`
	ChunkerVersion string `json:"chunker_version,omitempty"`
}

// StagePipeline manages multi-stage ingest via JetStream.
type StagePipeline struct {
	nats *natspub.Publisher
	ugc  *postgres.UGCRepo
}

// NewStagePipeline creates a stage pipeline wrapper.
func NewStagePipeline(nats *natspub.Publisher, ugcRepo *postgres.UGCRepo) *StagePipeline {
	return &StagePipeline{nats: nats, ugc: ugcRepo}
}

// Enabled reports whether staged pipeline JetStream subjects are configured.
func (sp *StagePipeline) Enabled() bool {
	return sp != nil && sp.nats != nil && sp.nats.StageStreamsEnabled()
}

// PublishStage publishes a stage message to the appropriate JetStream subject.
func (sp *StagePipeline) PublishStage(ctx context.Context, msg StageMessage) error {
	if sp.nats == nil {
		return fmt.Errorf("nats not configured")
	}
	data, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("marshal stage message: %w", err)
	}
	subject, err := sp.nats.StageSubject(model.IngestStage(msg.Stage))
	if err != nil {
		return err
	}

	// Write stage event record (best-effort).
	if sp.ugc != nil && msg.TaskID != "" {
		_ = sp.ugc.InsertStageEvent(ctx, postgres.StageEvent{
			TaskID:  msg.TaskID,
			TraceID: msg.TraceID,
			Stage:   msg.Stage,
			Status:  "queued",
			Attempt: msg.RetryCount + 1,
		})
	}

	if err := sp.nats.PublishStageMessage(ctx, subject, msg.TaskID, msg.TraceID, data); err != nil {
		return fmt.Errorf("publish stage %s: %w", msg.Stage, err)
	}

	trace.Logger(ctx).Info("stage published",
		"task_id", msg.TaskID,
		"stage", msg.Stage,
		"retry_count", msg.RetryCount,
		"subject", subject,
	)
	return nil
}

// BeginPipeline starts the pipeline from StageAccepted given a UGC queue message.
func (sp *StagePipeline) BeginPipeline(ctx context.Context, ugc model.UGCQueueMessage) error {
	deadline := time.Now().Add(10 * time.Minute).UTC()
	msg := StageMessage{
		TraceID:    ugc.TraceID,
		TaskID:     ugc.TaskID,
		Stage:      string(model.StageAccepted),
		RetryCount: 0,
		DeadlineAt: deadline.Format(time.RFC3339),
		DocID:      ugc.DocID,
		KBType:     ugc.KBType,
		Title:      ugc.Title,
		Content:    ugc.Content,
		SourceURL:  ugc.SourceURL,
		MimeType:   ugc.MimeType,
		GenerateQA: ugc.GenerateQA,
		Tags:       ugc.Tags,
		Extensions: ugc.Extensions,
	}
	return sp.PublishStage(ctx, msg)
}
