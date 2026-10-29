package handler

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/graydraft/core/repository/postgres"
)

// AdminCostHandler serves model cost summary APIs.
type AdminCostHandler struct {
	cost            *postgres.CostRepo
	dailyTokenLimit int64
}

func NewAdminCostHandler(cost *postgres.CostRepo, dailyTokenLimit int64) *AdminCostHandler {
	return &AdminCostHandler{cost: cost, dailyTokenLimit: dailyTokenLimit}
}

func (h *AdminCostHandler) Summary(c *gin.Context) {
	if h == nil || h.cost == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "cost store unavailable"})
		return
	}
	since := time.Now().Add(-24 * time.Hour)
	if hours, err := strconv.Atoi(c.DefaultQuery("hours", "24")); err == nil && hours > 0 && hours <= 24*30 {
		since = time.Now().Add(-time.Duration(hours) * time.Hour)
	}
	tenantID := c.Query("tenant_id")
	rows, err := h.cost.Summary(c.Request.Context(), tenantID, since)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	daily, err := h.cost.DailyTotals(c.Request.Context(), tenantID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"since":             since.UTC().Format(time.RFC3339),
		"tenant_id":         tenantID,
		"daily_tokens":      daily,
		"daily_token_limit": h.dailyTokenLimit,
		"budget_exceeded":   h.dailyTokenLimit > 0 && daily > h.dailyTokenLimit,
		"rows":              rows,
	})
}
