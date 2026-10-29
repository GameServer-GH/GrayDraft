// Package ugc implements large file manifest generation (V1-10).
// When a file is too large for direct API body transfer, a presigned OSS URL
// is generated and the Worker downloads it asynchronously.
package ugc

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/graydraft/core/model"
	natspub "github.com/graydraft/core/repository/nats"
	"github.com/graydraft/core/observability/trace"
)

const (
	// MaxInlineContentBytes is the maximum content size that can be sent directly via API.
	MaxInlineContentBytes = 512 * 1024 // 512 KB
	DefaultLargeFileBucket = "graydraft-uploads"
)

// LargeFilePublisher handles routing large files through async OSS download.
type LargeFilePublisher struct {
	nats    *natspub.Publisher
	subject string
}

// NewLargeFilePublisher creates a large file publisher.
func NewLargeFilePublisher(nats *natspub.Publisher) *LargeFilePublisher {
	return &LargeFilePublisher{
		nats:    nats,
		subject: "graydraft.ingest.large_file", // dedicated subject for large file manifests
	}
}

// NeedsLargeFileUpload checks if the content is too large for inline API transfer.
func NeedsLargeFileUpload(content string) bool {
	return len(content) > MaxInlineContentBytes
}

// PublishLargeFileManifest sends a manifest to the large file ingest subject
// for Worker-side download and processing.
func (lfp *LargeFilePublisher) PublishLargeFileManifest(
	ctx context.Context,
	msg model.UGCQueueMessage,
	presignedURL string,
	bucket string,
	objectKey string,
	fileSize int64,
	fileHash string,
) error {
	if lfp.nats == nil || !lfp.nats.JetStreamEnabled() {
		return fmt.Errorf("nats jetstream required for large file processing")
	}

	manifest := model.LargeFileManifest{
		TraceID:         msg.TraceID,
		TaskID:          msg.TaskID,
		DocID:           msg.DocID,
		KBType:          msg.KBType,
		Title:           msg.Title,
		SourceURL:       msg.SourceURL,
		MimeType:        msg.MimeType,
		GenerateQA:      msg.GenerateQA,
		Tags:            msg.Tags,
		Extensions:      msg.Extensions,
		StorageProvider: "s3",
		Bucket:          bucket,
		ObjectKey:       objectKey,
		PresignedURL:    presignedURL,
		FileSize:        fileSize,
		FileHash:        fileHash,
	}

	data, err := json.Marshal(manifest)
	if err != nil {
		return fmt.Errorf("marshal large file manifest: %w", err)
	}

	if err := lfp.nats.PublishStageMessage(ctx, lfp.subject, msg.TaskID, msg.TraceID, data); err != nil {
		return fmt.Errorf("publish large file manifest: %w", err)
	}

	trace.Logger(ctx).Info("large file manifest published",
		"task_id", msg.TaskID,
		"kb_type", msg.KBType,
		"file_size", fileSize,
		"bucket", bucket,
		"object_key", objectKey,
	)
	return nil
}
