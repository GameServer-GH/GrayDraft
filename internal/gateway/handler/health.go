package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/graydraft/gateway/config"
)

// HealthHandler serves health check.
type HealthHandler struct {
	cfg *config.Config
}

// NewHealthHandler creates health handler.
func NewHealthHandler(cfg *config.Config) *HealthHandler {
	return &HealthHandler{cfg: cfg}
}

// Health returns OK status and configured models.
func (h *HealthHandler) Health(c *gin.Context) {
	mode := "api"
	if !h.cfg.APIReady() {
		mode = "local_fallback"
	}
	c.JSON(http.StatusOK, gin.H{
		"status":  "ok",
		"service": h.cfg.App.Name,
		"mode":    mode,
		"models": gin.H{
			"embedding": gin.H{
				"provider": h.cfg.Models.Embedding.Provider,
				"model":    h.cfg.Models.Embedding.Model,
			},
			"llm_fast": gin.H{
				"provider": h.cfg.Models.LLMFast.Provider,
				"model":    h.cfg.Models.LLMFast.Model,
			},
			"llm": gin.H{
				"provider": h.cfg.Models.LLM.Provider,
				"model":    h.cfg.Models.LLM.Model,
				"role":     "complex",
			},
			"rerank": gin.H{
				"provider": h.cfg.Models.Rerank.Provider,
				"model":    h.cfg.Models.Rerank.Model,
			},
		},
	})
}
