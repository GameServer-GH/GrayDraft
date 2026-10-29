package handler

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/graydraft/core/config"
	"github.com/graydraft/core/observability"
	"github.com/graydraft/core/repository/postgres"
	"github.com/graydraft/core/service/ragops"
)

// AdminOpsHandler serves O-17~O-19 admin APIs.
type AdminOpsHandler struct {
	chunks *postgres.ChunkRepo
	cfg    *config.Config
	healer *ragops.Healer
}

// NewAdminOpsHandler creates ops admin handler.
func NewAdminOpsHandler(chunks *postgres.ChunkRepo, cfg *config.Config, healer *ragops.Healer) *AdminOpsHandler {
	return &AdminOpsHandler{chunks: chunks, cfg: cfg, healer: healer}
}

// RefreshGlobalIndex POST /api/v1/admin/global-index/refresh
func (h *AdminOpsHandler) RefreshGlobalIndex(c *gin.Context) {
	start := time.Now()
	concurrent := true
	if h.cfg != nil {
		concurrent = h.cfg.GlobalIndex.RefreshConcurrent
	}
	if v := c.Query("concurrent"); v == "false" {
		concurrent = false
	}
	if err := h.chunks.RefreshGlobalIndex(c.Request.Context(), concurrent); err != nil {
		observability.RecordBackfill("global_index_refresh", "fail", time.Since(start))
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	stats, _ := h.chunks.GlobalIndexStats(c.Request.Context())
	observability.RecordBackfill("global_index_refresh", "ok", time.Since(start))
	c.JSON(http.StatusOK, gin.H{"ok": true, "stats": stats})
}

// GlobalIndexStats GET /api/v1/admin/global-index/stats
func (h *AdminOpsHandler) GlobalIndexStats(c *gin.Context) {
	stats, err := h.chunks.GlobalIndexStats(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, stats)
}

// ApplyColdTier POST /api/v1/admin/tier/apply
func (h *AdminOpsHandler) ApplyColdTier(c *gin.Context) {
	days := 90
	if h.cfg != nil && h.cfg.StorageTier.ColdAfterDays > 0 {
		days = h.cfg.StorageTier.ColdAfterDays
	}
	if v := c.Query("cold_after_days"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			days = n
		}
	}
	n, err := h.chunks.ApplyColdTier(c.Request.Context(), days)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	stats, _ := h.chunks.TierStats(c.Request.Context())
	c.JSON(http.StatusOK, gin.H{"cold_marked": n, "stats": stats})
}

// TierStats GET /api/v1/admin/tier/stats
func (h *AdminOpsHandler) TierStats(c *gin.Context) {
	stats, err := h.chunks.TierStats(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, stats)
}

// RAGOpsScan POST /api/v1/admin/ragops/scan
func (h *AdminOpsHandler) RAGOpsScan(c *gin.Context) {
	if h.healer == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "ragops healer unavailable"})
		return
	}
	jobs, err := h.healer.Scan(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"enqueued": len(jobs), "jobs": jobs})
}

// RAGOpsListJobs GET /api/v1/admin/ragops/jobs
func (h *AdminOpsHandler) RAGOpsListJobs(c *gin.Context) {
	if h.healer == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "ragops healer unavailable"})
		return
	}
	status := c.Query("status")
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "50"))
	jobs, err := h.healer.ListJobs(c.Request.Context(), status, limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": jobs})
}

// RAGOpsRunJob POST /api/v1/admin/ragops/jobs/:id/run
func (h *AdminOpsHandler) RAGOpsRunJob(c *gin.Context) {
	if h.healer == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "ragops healer unavailable"})
		return
	}
	job, err := h.healer.Run(c.Request.Context(), c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, job)
}
