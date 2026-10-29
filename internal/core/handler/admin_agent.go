package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/graydraft/core/service/agent"
)

// AdminAgentHandler exposes read-only Agent trace APIs for the ops console.
type AdminAgentHandler struct {
	runs AgentRunStore
}

func NewAdminAgentHandler(runs AgentRunStore) *AdminAgentHandler {
	return &AdminAgentHandler{runs: runs}
}

func (h *AdminAgentHandler) RunTrace(c *gin.Context) {
	if h == nil || h.runs == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "agent store unavailable"})
		return
	}
	tenant := c.Query("tenant_id")
	if tenant == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "tenant_id is required"})
		return
	}
	runID := c.Param("run_id")
	events, err := h.runs.ListEvents(c.Request.Context(), tenant, runID, 0, 5000)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, agent.BuildRunTrace(runID, tenant, c.Query("policy_version"), events))
}
