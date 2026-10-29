package handler

import (
	"context"
	"github.com/gin-gonic/gin"
	"github.com/graydraft/core/service/agent"
	"net/http"
	"strings"
)

type MemoryStore interface {
	DeleteConversation(context.Context, string) error
	ExportConversation(context.Context, string) ([]agent.MemoryMessage, error)
}
type MemoryStoreFactory func(tenantID, actorID string) MemoryStore
type AgentMemoryHandler struct{ factory MemoryStoreFactory }

func NewAgentMemoryHandler(f MemoryStoreFactory) *AgentMemoryHandler {
	return &AgentMemoryHandler{factory: f}
}
func (h *AgentMemoryHandler) scope(c *gin.Context) (MemoryStore, bool) {
	tenant := strings.TrimSpace(c.GetHeader("X-Tenant-ID"))
	actor := strings.TrimSpace(c.GetHeader("X-Actor-ID"))
	if v, ok := c.Get("agent_tenant_id"); ok {
		tenant, _ = v.(string)
	}
	if v, ok := c.Get("agent_actor_id"); ok {
		actor, _ = v.(string)
	}
	if tenant == "" || actor == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "X-Tenant-ID and X-Actor-ID are required"})
		return nil, false
	}
	return h.factory(tenant, actor), true
}
func (h *AgentMemoryHandler) Export(c *gin.Context) {
	store, ok := h.scope(c)
	if !ok {
		return
	}
	msgs, err := store.ExportConversation(c.Request.Context(), c.Param("conversation_id"))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"messages": msgs})
}
func (h *AgentMemoryHandler) Delete(c *gin.Context) {
	store, ok := h.scope(c)
	if !ok {
		return
	}
	if err := store.DeleteConversation(c.Request.Context(), c.Param("conversation_id")); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.Status(http.StatusNoContent)
}
