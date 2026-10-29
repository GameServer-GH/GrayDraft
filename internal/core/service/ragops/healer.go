package ragops

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/graydraft/core/config"
	"github.com/graydraft/core/model"
	"github.com/graydraft/core/repository/postgres"
	"github.com/graydraft/core/service/answercache"
	"github.com/graydraft/core/service/cache"
	"github.com/graydraft/core/service/ugc"
)

// Healer implements RAGOps auto-heal (O-19).
type Healer struct {
	chunks *postgres.ChunkRepo
	docs   *postgres.DocumentRepo
	ugc    *ugc.Service
	cfg    *config.Config
	cache  *cache.Manager
	answer *answercache.Manager
}

// NewHealer creates healer service.
func NewHealer(chunks *postgres.ChunkRepo, docs *postgres.DocumentRepo, ugcSvc *ugc.Service, cfg *config.Config) *Healer {
	return &Healer{chunks: chunks, docs: docs, ugc: ugcSvc, cfg: cfg}
}

// WithCaches enables cache invalidation for direct RAGOps mutations.
func (h *Healer) WithCaches(cacheMgr *cache.Manager, answerCache *answercache.Manager) *Healer {
	if h == nil {
		return nil
	}
	h.cache = cacheMgr
	h.answer = answerCache
	return h
}

func (h *Healer) enabled() bool {
	return h.cfg != nil && h.cfg.RAGOps.Enabled
}

func (h *Healer) strikeDays() int {
	if h.cfg.RAGOps.StrikeDays > 0 {
		return h.cfg.RAGOps.StrikeDays
	}
	return 3
}

func (h *Healer) strikeThreshold() int {
	if h.cfg.RAGOps.StrikeThreshold > 0 {
		return h.cfg.RAGOps.StrikeThreshold
	}
	return 3
}

func (h *Healer) qualityBelow() float64 {
	if h.cfg.RAGOps.QualityBelow > 0 {
		return h.cfg.RAGOps.QualityBelow
	}
	return 0.45
}

// OnDownvote records strikes and may enqueue heal jobs after feedback downvote.
func (h *Healer) OnDownvote(ctx context.Context, req model.ChatFeedbackRequest, chunkIDs []string) {
	if !h.enabled() || len(chunkIDs) == 0 {
		return
	}
	var conf *float64
	if req.Confidence > 0 {
		c := req.Confidence
		conf = &c
	}
	for _, cid := range chunkIDs {
		if err := h.chunks.RecordChunkStrike(ctx, req.KBType, cid, "feedback", conf); err != nil {
			slog.Warn("ragops record strike", "chunk_id", cid, "error", err)
			continue
		}
		strikes, err := h.chunks.CountRecentStrikes(ctx, req.KBType, cid, h.strikeDays())
		if err != nil || strikes < h.strikeThreshold() {
			continue
		}
		docID := ""
		if len(req.DocIDs) > 0 {
			docID = req.DocIDs[0]
		} else {
			docID, _ = h.chunks.ChunkDocID(ctx, req.KBType, cid)
		}
		if docID == "" {
			continue
		}
		reason := fmt.Sprintf("downvote strikes=%d in %dd (confidence=%.2f)", strikes, h.strikeDays(), req.Confidence)
		if _, err := h.chunks.CreateHealJob(ctx, req.KBType, docID, cid, reason, h.cfg.RAGOps.DefaultAction, strikes); err != nil {
			slog.Warn("ragops create heal job", "doc_id", docID, "error", err)
		}
	}
}

// Scan enqueues heal jobs for low-quality chunks with repeated strikes.
func (h *Healer) Scan(ctx context.Context) ([]postgres.HealJob, error) {
	if !h.enabled() {
		return nil, fmt.Errorf("ragops disabled")
	}
	candidates, err := h.chunks.ListLowQualityChunks(
		ctx, h.qualityBelow(), h.strikeDays(), h.strikeThreshold(), 100,
	)
	if err != nil {
		return nil, err
	}
	var jobs []postgres.HealJob
	for _, c := range candidates {
		reason := fmt.Sprintf("scan quality=%.2f strikes=%d", c.QualityScore, c.StrikeDays)
		job, err := h.chunks.CreateHealJob(ctx, c.KBType, c.DocID, c.ChunkID, reason, h.cfg.RAGOps.DefaultAction, c.StrikeDays)
		if err != nil {
			continue
		}
		jobs = append(jobs, *job)
	}
	return jobs, nil
}

// Run executes one heal job (rechunk | demote).
func (h *Healer) Run(ctx context.Context, jobID string) (*postgres.HealJob, error) {
	job, err := h.chunks.GetHealJob(ctx, jobID)
	if err != nil {
		return nil, err
	}
	if job.Status != "pending" && job.Status != "failed" {
		return job, nil
	}
	_ = h.chunks.UpdateHealJobStatus(ctx, jobID, "running", nil)

	switch job.Action {
	case "demote":
		if job.ChunkID != "" {
			_ = h.chunks.DemoteChunk(ctx, job.KBType, job.ChunkID, 0.15)
		}
		h.purgeCaches(ctx, job.KBType, "demote")
		meta, _ := json.Marshal(map[string]string{"result": "demoted"})
		_ = h.chunks.UpdateHealJobStatus(ctx, jobID, "completed", meta)
	case "rechunk":
		if h.ugc == nil {
			_ = h.chunks.UpdateHealJobStatus(ctx, jobID, "failed", json.RawMessage(`{"error":"ugc unavailable"}`))
			break
		}
		detail, err := h.docs.GetPanelDocument(ctx, job.KBType, job.DocID)
		if err != nil || detail == nil {
			meta, _ := json.Marshal(map[string]string{"error": "document not found"})
			_ = h.chunks.UpdateHealJobStatus(ctx, jobID, "failed", meta)
			break
		}
		genQA := true
		resp, err := h.ugc.Notify(ctx, model.UGCNotifyRequest{
			KBType:     job.KBType,
			DocID:      job.DocID,
			Title:      detail.Title,
			Content:    detail.Content,
			SourceURL:  detail.SourceURL,
			GenerateQA: &genQA,
		})
		if err != nil {
			meta, _ := json.Marshal(map[string]string{"error": err.Error()})
			_ = h.chunks.UpdateHealJobStatus(ctx, jobID, "failed", meta)
			break
		}
		meta, _ := json.Marshal(map[string]string{"task_id": resp.TaskID, "channel": resp.Channel})
		_ = h.chunks.UpdateHealJobStatus(ctx, jobID, "completed", meta)
	default:
		meta, _ := json.Marshal(map[string]string{"error": "unknown action"})
		_ = h.chunks.UpdateHealJobStatus(ctx, jobID, "failed", meta)
	}
	return h.chunks.GetHealJob(ctx, jobID)
}

// ListJobs returns heal jobs.
func (h *Healer) ListJobs(ctx context.Context, status string, limit int) ([]postgres.HealJob, error) {
	return h.chunks.ListHealJobs(ctx, status, limit)
}

func (h *Healer) purgeCaches(ctx context.Context, kbType, reason string) {
	if kbType == "" {
		return
	}
	if h.cache != nil {
		n, err := h.cache.RedisPurgeByKBType(ctx, kbType)
		if err != nil {
			slog.Warn("ragops retrieval cache purge failed", "kb_type", kbType, "reason", reason, "error", err)
		} else {
			slog.Info("ragops retrieval cache purged", "kb_type", kbType, "reason", reason, "removed", n)
		}
	}
	if h.answer != nil {
		n, err := h.answer.RedisPurgeByKBType(ctx, kbType)
		if err != nil {
			slog.Warn("ragops answer cache purge failed", "kb_type", kbType, "reason", reason, "error", err)
		} else {
			slog.Info("ragops answer cache purged", "kb_type", kbType, "reason", reason, "removed", n)
		}
	}
}
