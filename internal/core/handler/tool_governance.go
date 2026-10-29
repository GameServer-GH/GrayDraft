package handler

import (
	"context"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

type ToolGovernanceAdmin interface {
	CreateApproval(context.Context, string, string, string, string, string, time.Time) error
	Decide(context.Context, string, string, string, string) error
}

type ToolGovernanceHandler struct {
	store func(string) ToolGovernanceAdmin
}

func NewToolGovernanceHandler(store func(string) ToolGovernanceAdmin) *ToolGovernanceHandler {
	return &ToolGovernanceHandler{store: store}
}

func governanceTenant(c *gin.Context) (string, bool) {
	if v, ok := c.Get("agent_tenant_id"); ok {
		if tenant, ok := v.(string); ok && strings.TrimSpace(tenant) != "" {
			return tenant, true
		}
	}
	if tenant := strings.TrimSpace(c.GetHeader("X-Tenant-ID")); tenant != "" {
		return tenant, true
	}
	c.JSON(http.StatusUnauthorized, gin.H{"error": "tenant context required"})
	return "", false
}

func governanceActor(c *gin.Context) (string, bool) {
	if v, ok := c.Get("agent_actor_id"); ok {
		if actor, ok := v.(string); ok && strings.TrimSpace(actor) != "" {
			return actor, true
		}
	}
	if actor := strings.TrimSpace(c.GetHeader("X-Actor-ID")); actor != "" {
		return actor, true
	}
	c.JSON(http.StatusUnauthorized, gin.H{"error": "actor context required"})
	return "", false
}

func (h *ToolGovernanceHandler) RequestApproval(c *gin.Context) {
	tenant, ok := governanceTenant(c)
	if !ok {
		return
	}
	actor, ok := governanceActor(c)
	if !ok {
		return
	}
	var req struct {
		RunID         string `json:"run_id" binding:"required"`
		CallID        string `json:"call_id" binding:"required"`
		ToolName      string `json:"tool_name" binding:"required"`
		ArgumentsHash string `json:"arguments_hash" binding:"required"`
		TTLSeconds    int    `json:"ttl_seconds"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if req.TTLSeconds <= 0 || req.TTLSeconds > 3600 {
		req.TTLSeconds = 300
	}
	expires := time.Now().Add(time.Duration(req.TTLSeconds) * time.Second)
	if err := h.store(tenant).CreateApproval(c, actor, req.RunID, req.CallID, req.ToolName, req.ArgumentsHash, expires); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusAccepted, gin.H{"tenant_id": tenant, "status": "pending", "expires_at": expires})
}

func (h *ToolGovernanceHandler) Decide(c *gin.Context) {
	tenant, ok := governanceTenant(c)
	if !ok {
		return
	}
	actor, ok := governanceActor(c)
	if !ok {
		return
	}
	var req struct {
		Status string `json:"status"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || (req.Status != "approved" && req.Status != "rejected") {
		c.JSON(http.StatusBadRequest, gin.H{"error": "status must be approved or rejected"})
		return
	}
	if err := h.store(tenant).Decide(c, c.Param("run_id"), c.Param("call_id"), actor, req.Status); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": req.Status})
}
