package handler

import (
	"context"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/graydraft/core/repository/postgres"
)

type ToolApprovalLister interface {
	ListApprovals(context.Context, string, int) ([]postgres.ApprovalRecord, error)
}

type ToolApprovalDecider interface {
	Decide(context.Context, string, string, string, string) error
}

type AdminApprovalsHandler struct {
	store func(string) ToolApprovalLister
}

func NewAdminApprovalsHandler(store func(string) ToolApprovalLister) *AdminApprovalsHandler {
	return &AdminApprovalsHandler{store: store}
}

func (h *AdminApprovalsHandler) List(c *gin.Context) {
	tenant := c.Query("tenant_id")
	if tenant == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "tenant_id is required"})
		return
	}
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "50"))
	items, err := h.store(tenant).ListApprovals(c.Request.Context(), c.Query("status"), limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": items})
}

func (h *AdminApprovalsHandler) Decide(c *gin.Context) {
	tenant := c.Query("tenant_id")
	if tenant == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "tenant_id is required"})
		return
	}
	var req struct {
		Status string `json:"status"`
		Actor  string `json:"actor_id"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || (req.Status != "approved" && req.Status != "rejected") {
		c.JSON(http.StatusBadRequest, gin.H{"error": "status must be approved or rejected"})
		return
	}
	actor := req.Actor
	if actor == "" {
		actor = "console-admin"
	}
	decider, ok := h.store(tenant).(ToolApprovalDecider)
	if !ok {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "approval decide unavailable"})
		return
	}
	if err := decider.Decide(c.Request.Context(), c.Param("run_id"), c.Param("call_id"), actor, req.Status); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": req.Status})
}

func (h *ToolGovernanceHandler) ListApprovals(c *gin.Context) {
	tenant, ok := governanceTenant(c)
	if !ok {
		return
	}
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "50"))
	lister, ok := h.store(tenant).(ToolApprovalLister)
	if !ok {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "approval list unavailable"})
		return
	}
	items, err := lister.ListApprovals(c.Request.Context(), c.Query("status"), limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": items})
}
