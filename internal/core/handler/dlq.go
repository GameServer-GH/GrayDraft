package handler

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/graydraft/core/model"
	"github.com/graydraft/core/observability"
	natspub "github.com/graydraft/core/repository/nats"
	"github.com/graydraft/core/repository/postgres"
)

// DLQHandler admin APIs for JetStream dead-letter queue.
type DLQHandler struct {
	nats *natspub.Publisher
	ugc  *postgres.UGCRepo
}

// NewDLQHandler creates DLQ admin handler.
func NewDLQHandler(nats *natspub.Publisher, ugc *postgres.UGCRepo) *DLQHandler {
	return &DLQHandler{nats: nats, ugc: ugc}
}

// Stats handles GET /api/v1/admin/dlq/stats
func (h *DLQHandler) Stats(c *gin.Context) {
	if h.nats == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "nats not configured"})
		return
	}
	stats, err := h.nats.DLQStats(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
		return
	}
	observability.SetDLQMessages(float64(stats.Messages))
	c.JSON(http.StatusOK, stats)
}

// List handles GET /api/v1/admin/dlq/messages
func (h *DLQHandler) List(c *gin.Context) {
	if h.nats == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "nats not configured"})
		return
	}
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "50"))
	items, stats, err := h.nats.ListDLQ(c.Request.Context(), limit)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": items, "stream": stats})
}

// Replay handles POST /api/v1/admin/dlq/replay
func (h *DLQHandler) Replay(c *gin.Context) {
	if h.nats == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "nats not configured"})
		return
	}
	var req model.DLQReplayRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if len(req.Sequences) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "sequences required"})
		return
	}
	if len(req.Sequences) > 50 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "max 50 sequences per request"})
		return
	}

	ctx := c.Request.Context()
	results := h.nats.ReplayDLQBatch(ctx, req.Sequences)
	for i := range results {
		if !results[i].OK || results[i].TaskID == "" || h.ugc == nil {
			continue
		}
		id, err := uuid.Parse(results[i].TaskID)
		if err != nil {
			continue
		}
		_ = h.ugc.ResetTaskPending(ctx, id)
	}
	c.JSON(http.StatusOK, gin.H{"results": results})
}

// Discard handles POST /api/v1/admin/dlq/discard
func (h *DLQHandler) Discard(c *gin.Context) {
	if h.nats == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "nats not configured"})
		return
	}
	var req model.DLQDiscardRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if len(req.Sequences) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "sequences required"})
		return
	}

	ctx := c.Request.Context()
	var failed []gin.H
	for _, seq := range req.Sequences {
		if err := h.nats.DiscardDLQ(ctx, seq); err != nil {
			failed = append(failed, gin.H{"sequence": seq, "error": err.Error()})
		}
	}
	c.JSON(http.StatusOK, gin.H{
		"discarded": len(req.Sequences) - len(failed),
		"failed":    failed,
	})
}
