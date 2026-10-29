package handler

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/graydraft/core/repository/postgres"
)

// AdminHandler serves console / ops admin APIs.
type AdminHandler struct {
	chunks *postgres.ChunkRepo
	ugc    *postgres.UGCRepo
}

// NewAdminHandler creates admin handler.
func NewAdminHandler(chunks *postgres.ChunkRepo, ugc *postgres.UGCRepo) *AdminHandler {
	return &AdminHandler{chunks: chunks, ugc: ugc}
}

// Overview handles GET /api/v1/admin/overview
func (h *AdminHandler) Overview(c *gin.Context) {
	ctx := c.Request.Context()

	catalog, err := h.chunks.ListCatalogVersions(ctx)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	docStats, err := h.chunks.DocumentStats(ctx)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	ugcCounts, err := h.ugc.UGCStatusCounts(ctx)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"catalog":      catalog,
		"documents":    docStats,
		"ugc_by_status": ugcCounts,
	})
}

// Catalog handles GET /api/v1/admin/catalog
func (h *AdminHandler) Catalog(c *gin.Context) {
	rows, err := h.chunks.ListCatalogVersions(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": rows})
}

// DocumentStats handles GET /api/v1/admin/documents/stats
func (h *AdminHandler) DocumentStats(c *gin.Context) {
	rows, err := h.chunks.DocumentStats(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": rows})
}

// ListUGCTasks handles GET /api/v1/admin/ugc/tasks
func (h *AdminHandler) ListUGCTasks(c *gin.Context) {
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "50"))
	status := c.Query("status")
	items, err := h.ugc.ListTasks(c.Request.Context(), limit, status)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": items})
}

// ListChunks handles GET /api/v1/admin/chunks (O-08).
func (h *AdminHandler) ListChunks(c *gin.Context) {
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "50"))
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))
	kbType := c.Query("kb_type")
	items, total, err := h.chunks.ListAdminChunks(c.Request.Context(), kbType, limit, offset)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": items, "total": total, "kb_type": kbType})
}
