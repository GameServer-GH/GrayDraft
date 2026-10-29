package handler

import (
	"github.com/gin-gonic/gin"
	"github.com/graydraft/core/service/agent"
	"net/http"
	"strings"
)

type LongMemoryAdminFactory func(string, string) agent.LongMemoryAdmin
type LongMemoryHandler struct{ factory LongMemoryAdminFactory }

func NewLongMemoryHandler(f LongMemoryAdminFactory) *LongMemoryHandler {
	return &LongMemoryHandler{factory: f}
}
func (h *LongMemoryHandler) scope(c *gin.Context) (agent.LongMemoryAdmin, string, string, bool) {
	t := strings.TrimSpace(c.GetHeader("X-Tenant-ID"))
	a := strings.TrimSpace(c.GetHeader("X-Actor-ID"))
	if v, ok := c.Get("agent_tenant_id"); ok {
		t, _ = v.(string)
	}
	if v, ok := c.Get("agent_actor_id"); ok {
		a, _ = v.(string)
	}
	if t == "" || a == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "X-Tenant-ID and X-Actor-ID are required"})
		return nil, "", "", false
	}
	return h.factory(t, a), t, a, true
}
func (h *LongMemoryHandler) List(c *gin.Context) {
	s, t, a, ok := h.scope(c)
	if !ok {
		return
	}
	items, err := s.List(c.Request.Context(), t, a, c.Query("status"), 50)
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	c.JSON(200, gin.H{"items": items})
}
func (h *LongMemoryHandler) Revoke(c *gin.Context) {
	s, t, a, ok := h.scope(c)
	if !ok {
		return
	}
	if err := s.Revoke(c.Request.Context(), t, a, c.Param("memory_id")); err != nil {
		if strings.Contains(err.Error(), "not found") || strings.Contains(err.Error(), "invalid id") {
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *LongMemoryHandler) Resolve(c *gin.Context) {
	s, t, a, ok := h.scope(c)
	if !ok {
		return
	}
	if err := s.ResolveConflict(c.Request.Context(), t, a, c.Param("memory_id")); err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	c.Status(204)
}
