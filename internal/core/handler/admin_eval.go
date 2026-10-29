package handler

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/graydraft/core/repository/postgres"
	"github.com/graydraft/core/service/agent"
)

// AdminEvalHandler triggers and lists agent evaluation runs.
type AdminEvalHandler struct {
	svc  *agent.EvalService
	eval *postgres.EvalRepo
}

func NewAdminEvalHandler(svc *agent.EvalService, eval *postgres.EvalRepo) *AdminEvalHandler {
	return &AdminEvalHandler{svc: svc, eval: eval}
}

type runEvalRequest struct {
	DatasetPath   string `json:"dataset_path"`
	TenantID      string `json:"tenant_id"`
	AgentID       string `json:"agent_id"`
	Model         string `json:"model"`
	PolicyVersion string `json:"policy_version"`
	UseOracle     bool   `json:"use_oracle"`
}

func (h *AdminEvalHandler) Run(c *gin.Context) {
	if h == nil || h.svc == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "eval service unavailable"})
		return
	}
	var req runEvalRequest
	_ = c.ShouldBindJSON(&req)
	rec, err := h.svc.RunGolden(c.Request.Context(), agent.RunGoldenRequest{
		DatasetPath:   req.DatasetPath,
		TenantID:      req.TenantID,
		AgentID:       req.AgentID,
		Model:         req.Model,
		PolicyVersion: req.PolicyVersion,
		UseOracle:     req.UseOracle,
	})
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, rec)
}

func (h *AdminEvalHandler) List(c *gin.Context) {
	if h == nil || h.svc == nil || h.svc.Store == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "eval service unavailable"})
		return
	}
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	items, err := h.svc.Store.List(c.Request.Context(), c.Query("tenant_id"), limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": items})
}

func (h *AdminEvalHandler) Quality(c *gin.Context) {
	if h == nil || h.eval == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "eval store unavailable"})
		return
	}
	since := time.Now().Add(-7 * 24 * time.Hour)
	if days, err := strconv.Atoi(c.DefaultQuery("days", "7")); err == nil && days > 0 && days <= 90 {
		since = time.Now().Add(-time.Duration(days) * 24 * time.Hour)
	}
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "10"))
	summary, err := h.eval.QualitySummary(c.Request.Context(), c.Query("tenant_id"), since, limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, summary)
}
