package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/graydraft/core/model"
	"github.com/graydraft/core/repository/postgres"
	"github.com/graydraft/core/service/llmclient"
	"github.com/graydraft/core/service/taxonomyevolve"
	"github.com/graydraft/core/service/workspacecontext"
)

// WorkspaceContextHandler serves workspace background CRUD on kb_types partitions.
type WorkspaceContextHandler struct {
	docs *postgres.DocumentRepo
	llm  *llmclient.Client
}

// NewWorkspaceContextHandler creates handler.
func NewWorkspaceContextHandler(docs *postgres.DocumentRepo, llm *llmclient.Client) *WorkspaceContextHandler {
	return &WorkspaceContextHandler{docs: docs, llm: llm}
}

// GetContext handles GET /api/v1/kb/:kb_type/context
func (h *WorkspaceContextHandler) GetContext(c *gin.Context) {
	kbType := c.Param("kb_type")
	if kbType == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "missing kb_type"})
		return
	}
	profile, err := h.docs.GetContextProfile(c.Request.Context(), kbType)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, profile)
}

// PutContext handles PUT /api/v1/kb/:kb_type/context
func (h *WorkspaceContextHandler) PutContext(c *gin.Context) {
	kbType := c.Param("kb_type")
	if kbType == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "missing kb_type"})
		return
	}
	var profile model.ContextProfile
	if err := c.ShouldBindJSON(&profile); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := h.docs.SetContextProfile(c.Request.Context(), kbType, profile); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	profile.KBType = kbType
	taxonomyevolve.EnqueueBlueprint(c.Request.Context(), kbType, &profile, "")
	c.JSON(http.StatusOK, gin.H{"kb_type": kbType, "updated": true, "profile": profile})
}

// EnrichContext handles POST /api/v1/kb/:kb_type/context/enrich
func (h *WorkspaceContextHandler) EnrichContext(c *gin.Context) {
	kbType := c.Param("kb_type")
	if kbType == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "missing kb_type"})
		return
	}
	var req struct {
		Description string `json:"description"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	profile, err := workspacecontext.EnrichFromBrief(c.Request.Context(), h.llm, req.Description)
	if err != nil && profile.IsEmpty() {
		c.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
		return
	}
	profile.KBType = kbType
	if saveErr := h.docs.SetContextProfile(c.Request.Context(), kbType, profile); saveErr != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": saveErr.Error()})
		return
	}
	resp := gin.H{"profile": profile, "saved": true}
	if err != nil {
		resp["warning"] = err.Error()
	}
	taxonomyevolve.EnqueueBlueprint(c.Request.Context(), kbType, &profile, "")
	c.JSON(http.StatusOK, resp)
}
