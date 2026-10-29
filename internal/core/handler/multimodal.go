package handler

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"
	"github.com/graydraft/core/repository/postgres"
	"github.com/graydraft/core/service/multimodal"
)

// MultimodalHandler exposes 4.0 multimodal APIs.
type MultimodalHandler struct {
	proc  *multimodal.Processor
	cache *postgres.MultimodalCacheRepo
}

// NewMultimodalHandler creates handler.
func NewMultimodalHandler(proc *multimodal.Processor, cache *postgres.MultimodalCacheRepo) *MultimodalHandler {
	return &MultimodalHandler{proc: proc, cache: cache}
}

// GetPromptDefaults handles GET /api/v1/multimodal/prompt-defaults
func (h *MultimodalHandler) GetPromptDefaults(c *gin.Context) {
	base := multimodal.DefaultBasePrompt
	if h.proc != nil {
		base = h.proc.BasePrompt()
	}
	c.JSON(http.StatusOK, gin.H{
		"base_prompt": base,
		"enabled":     h.proc != nil && h.proc.Enabled(),
	})
}

// GetMedia handles GET /api/v1/multimodal/media/:md5 — visual RAG metadata (4.0 §5.2).
func (h *MultimodalHandler) GetMedia(c *gin.Context) {
	md5 := c.Param("md5")
	if md5 == "" || h.cache == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
		return
	}
	mime, route, text, structured, err := h.cache.GetMediaMeta(c.Request.Context(), md5)
	if errors.Is(err, pgx.ErrNoRows) {
		c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	preview := text
	if len(preview) > 2000 {
		preview = preview[:2000] + "..."
	}
	c.JSON(http.StatusOK, gin.H{
		"md5":           md5,
		"mime_type":     mime,
		"route":         route,
		"media_url":     multimodal.MediaRefURL(md5),
		"preview":       preview,
		"structured_kv": structured,
	})
}
