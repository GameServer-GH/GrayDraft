package handler

import (
	"context"
	"github.com/gin-gonic/gin"
	"github.com/graydraft/core/service/agent"
	"net/http"
	"strings"
)

type ToolRegistryAdmin interface {
	Upsert(context.Context, agent.ToolSpec) error
	Publish(context.Context, string, string) error
	BindTenant(context.Context, string, string, string, []string) error
	DisableTenantTool(context.Context, string, string) error
}
type ToolRegistryHandler struct{ store ToolRegistryAdmin }

func NewToolRegistryHandler(s ToolRegistryAdmin) *ToolRegistryHandler {
	return &ToolRegistryHandler{store: s}
}
func (h *ToolRegistryHandler) tenant(c *gin.Context) (string, bool) {
	t := strings.TrimSpace(c.GetHeader("X-Tenant-ID"))
	if v, ok := c.Get("agent_tenant_id"); ok {
		t, _ = v.(string)
	}
	if t == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "tenant context required"})
		return "", false
	}
	return t, true
}
func (h *ToolRegistryHandler) Publish(c *gin.Context) {
	var s agent.ToolSpec
	if c.ShouldBindJSON(&s) != nil {
		c.JSON(400, gin.H{"error": "invalid tool spec"})
		return
	}
	if err := h.store.Upsert(c, s); err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	if err := h.store.Publish(c, s.Name, s.Version); err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	c.JSON(200, gin.H{"status": "active", "name": s.Name, "version": s.Version})
}
func (h *ToolRegistryHandler) Bind(c *gin.Context) {
	tenant, ok := h.tenant(c)
	if !ok {
		return
	}
	var req struct {
		Name, Version string
		Scopes        []string
	}
	if c.ShouldBindJSON(&req) != nil || req.Name == "" || req.Version == "" {
		c.JSON(400, gin.H{"error": "name and version required"})
		return
	}
	if err := h.store.BindTenant(c, tenant, req.Name, req.Version, req.Scopes); err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	c.Status(204)
}
func (h *ToolRegistryHandler) Disable(c *gin.Context) {
	tenant, ok := h.tenant(c)
	if !ok {
		return
	}
	if err := h.store.DisableTenantTool(c, tenant, c.Param("name")); err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	c.Status(204)
}
