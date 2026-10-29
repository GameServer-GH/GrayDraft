package handler

import (
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/graydraft/gateway/service/embedder"
	"github.com/graydraft/gateway/service/provider"
)

// EmbeddingsHandler serves embedding API.
type EmbeddingsHandler struct {
	svc *embedder.Service
}

// NewEmbeddingsHandler creates handler.
func NewEmbeddingsHandler(svc *embedder.Service) *EmbeddingsHandler {
	return &EmbeddingsHandler{svc: svc}
}

type embedRequest struct {
	Texts []string `json:"texts" binding:"required"`
	Model string   `json:"model"`
}

type embedResponse struct {
	Vectors   [][]float32 `json:"vectors"`
	Model     string      `json:"model"`
	Dimension int         `json:"dimension"`
}

// Embed handles POST /v1/embeddings
func (h *EmbeddingsHandler) Embed(c *gin.Context) {
	// V1-30: circuit breaker check
	br := provider.GetBreaker("embedding")
	if !br.Allow() {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "circuit breaker open", "retry_after_ms": 30000})
		_ = c.Error(fmt.Errorf("circuit breaker open"))
		return
	}
	var req embedRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	vecs, model, err := h.svc.Embed(c.Request.Context(), req.Texts, req.Model)
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": err.Error()})
		return
	}
	dim := 0
	if len(vecs) > 0 {
		dim = len(vecs[0])
	}
	c.JSON(http.StatusOK, embedResponse{Vectors: vecs, Model: model, Dimension: dim})
}
