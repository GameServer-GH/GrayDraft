package ugc

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/graydraft/core/config"
	"github.com/graydraft/core/model"
	"github.com/graydraft/core/observability/trace"
	natspub "github.com/graydraft/core/repository/nats"
	"github.com/graydraft/core/repository/postgres"
	"github.com/graydraft/core/service/answercache"
	"github.com/graydraft/core/service/cache"
)

// Service handles UGC notify and dispatch to NATS / Worker.
type Service struct {
	cfg           *config.Config
	ugcRepo       *postgres.UGCRepo
	nats          *natspub.Publisher
	worker        *WorkerClient
	cacheMgr      *cache.Manager
	answerCache   *answercache.Manager
	stagePipeline *StagePipeline
}

// NewService creates UGC service.
func NewService(cfg *config.Config, ugcRepo *postgres.UGCRepo, nats *natspub.Publisher, worker *WorkerClient) *Service {
	return &Service{cfg: cfg, ugcRepo: ugcRepo, nats: nats, worker: worker}
}

// WithCaches enables best-effort retrieval/answer cache purge after a document mutation is accepted.
func (s *Service) WithCaches(cacheMgr *cache.Manager, answerCache *answercache.Manager) *Service {
	if s == nil {
		return nil
	}
	s.cacheMgr = cacheMgr
	s.answerCache = answerCache
	return s
}

// WithStagePipeline enables the staged JetStream ingest pipeline (V1-07).
func (s *Service) WithStagePipeline(sp *StagePipeline) *Service {
	if s == nil {
		return nil
	}
	s.stagePipeline = sp
	return s
}

// Notify validates and queues UGC for async ingestion.
func (s *Service) Notify(ctx context.Context, req model.UGCNotifyRequest) (*model.UGCNotifyResponse, error) {
	if req.MimeType == "" {
		req.MimeType = "text/plain"
	}
	generateQA := true
	if req.GenerateQA != nil {
		generateQA = *req.GenerateQA
	}

	exists, err := s.ugcRepo.KBTypeExists(ctx, req.KBType)
	if err != nil {
		return nil, fmt.Errorf("validate kb_type: %w", err)
	}
	if !exists {
		return nil, fmt.Errorf("unknown kb_type: %s", req.KBType)
	}

	traceID := trace.FromContext(ctx)
	taskID := uuid.New()
	var docID *uuid.UUID
	if req.DocID != "" {
		parsed, err := uuid.Parse(req.DocID)
		if err != nil {
			return nil, fmt.Errorf("invalid doc_id: %w", err)
		}
		docID = &parsed
	}

	if err := s.ugcRepo.CreateTask(ctx, taskID, req.KBType, req.Title, docID); err != nil {
		return nil, fmt.Errorf("create task: %w", err)
	}

	msg := model.UGCQueueMessage{
		TraceID:    traceID,
		TaskID:     taskID.String(),
		KBType:     req.KBType,
		Title:      req.Title,
		Content:    req.Content,
		SourceURL:  req.SourceURL,
		MimeType:   req.MimeType,
		GenerateQA: generateQA,
		Tags:       req.Tags,
		Extensions: req.Extensions,
	}
	if docID != nil {
		msg.DocID = docID.String()
	}

	// Prefer NATS async path
	if s.nats != nil {
		// V1-07: use staged pipeline when JetStream stage_streams_enabled.
		if s.nats.StageStreamsEnabled() && s.stagePipeline != nil {
			if err := s.stagePipeline.BeginPipeline(ctx, msg); err == nil {
				channel := "jetstream_stage"
				trace.Logger(ctx).Info("ugc queued",
					"task_id", taskID.String(),
					"kb_type", req.KBType,
					"channel", channel,
					"doc_id", msg.DocID,
				)
				s.purgeCaches(ctx, req.KBType)
				return &model.UGCNotifyResponse{
					TraceID:  traceID,
					TaskID:   taskID.String(),
					DocID:    msg.DocID,
					Accepted: true,
					Channel:  channel,
					Message:  "queued for staged async ingestion",
				}, nil
			}
			trace.Logger(ctx).Warn("staged pipeline publish failed", "task_id", taskID.String(), "error", err)
		}

		if err := s.nats.PublishUGC(ctx, msg); err == nil {
			channel := "nats"
			if s.nats.JetStreamEnabled() {
				channel = "jetstream"
			}
			trace.Logger(ctx).Info("ugc queued",
				"task_id", taskID.String(),
				"kb_type", req.KBType,
				"channel", channel,
				"doc_id", msg.DocID,
			)
			s.purgeCaches(ctx, req.KBType)
			return &model.UGCNotifyResponse{
				TraceID:  traceID,
				TaskID:   taskID.String(),
				DocID:    msg.DocID,
				Accepted: true,
				Channel:  channel,
				Message:  "queued for async ingestion",
			}, nil
		}
		trace.Logger(ctx).Warn("nats publish failed", "task_id", taskID.String(), "error", err)
	}

	// Degrade: synchronous Worker ingest
	if s.cfg.UGC.SyncFallback && s.worker != nil {
		result, err := s.worker.IngestUGC(ctx, msg)
		if err != nil {
			_ = s.ugcRepo.UpdateTaskFailed(ctx, taskID, err.Error())
			return nil, fmt.Errorf("sync ingest failed: %w", err)
		}
		docUUID, _ := uuid.Parse(result.DocID)
		_ = s.ugcRepo.UpdateTaskCompleted(ctx, taskID, docUUID, result.ChunkCount)
		trace.Logger(ctx).Info("ugc sync_fallback completed",
			"task_id", taskID.String(),
			"doc_id", result.DocID,
			"kb_type", req.KBType,
		)
		s.purgeCaches(ctx, req.KBType)
		return &model.UGCNotifyResponse{
			TraceID:  traceID,
			TaskID:   taskID.String(),
			DocID:    result.DocID,
			Accepted: true,
			Channel:  "sync_fallback",
			Message:  "ingested synchronously via worker",
		}, nil
	}

	_ = s.ugcRepo.UpdateTaskFailed(ctx, taskID, "nats unavailable and sync_fallback disabled")
	return nil, fmt.Errorf("unable to dispatch ugc: nats unavailable")
}

// GetTaskStatus returns task record for polling.
func (s *Service) GetTaskStatus(ctx context.Context, taskID string) (map[string]any, error) {
	id, err := uuid.Parse(taskID)
	if err != nil {
		return nil, fmt.Errorf("invalid task_id")
	}
	return s.ugcRepo.GetTask(ctx, id)
}

// WaitTaskQueryable polls task readiness until it becomes queryable, terminal, or timeout.
func (s *Service) WaitTaskQueryable(ctx context.Context, taskID string, timeout, interval time.Duration) (map[string]any, error) {
	if timeout <= 0 {
		return s.GetTaskStatus(ctx, taskID)
	}
	if timeout > 30*time.Second {
		timeout = 30 * time.Second
	}
	if interval <= 0 {
		interval = 500 * time.Millisecond
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	for {
		status, err := s.GetTaskStatus(ctx, taskID)
		if err != nil {
			return nil, err
		}
		if isTaskQueryableOrTerminal(status) {
			return status, nil
		}
		timer := time.NewTimer(interval)
		select {
		case <-ctx.Done():
			timer.Stop()
			status["wait_timeout"] = true
			status["wait_timeout_ms"] = timeout.Milliseconds()
			return status, nil
		case <-timer.C:
		}
	}
}

func isTaskQueryableOrTerminal(status map[string]any) bool {
	if v, ok := status["queryable"].(bool); ok && v {
		return true
	}
	switch strings.TrimSpace(fmt.Sprint(status["readiness"])) {
	case "failed", "ready", "completed_without_doc", "completed_without_chunks", "completed_chunk_mismatch":
		return true
	}
	return false
}

// IsTaskQueryableOrTerminal reports whether task status has reached queryable or a terminal blocked state.
func IsTaskQueryableOrTerminal(status map[string]any) bool {
	return isTaskQueryableOrTerminal(status)
}

func (s *Service) purgeCaches(ctx context.Context, kbType string) {
	kbType = strings.TrimSpace(kbType)
	if kbType == "" {
		return
	}
	if s.cacheMgr != nil {
		n, err := s.cacheMgr.RedisPurgeByKBType(ctx, kbType)
		if err != nil {
			trace.Logger(ctx).Warn("ugc retrieval cache purge failed", "kb_type", kbType, "error", err)
		} else {
			trace.Logger(ctx).Info("ugc retrieval cache purged", "kb_type", kbType, "removed", n)
		}
	}
	if s.answerCache != nil {
		n, err := s.answerCache.RedisPurgeByKBType(ctx, kbType)
		if err != nil {
			trace.Logger(ctx).Warn("ugc answer cache purge failed", "kb_type", kbType, "error", err)
		} else {
			trace.Logger(ctx).Info("ugc answer cache purged", "kb_type", kbType, "removed", n)
		}
	}
}
