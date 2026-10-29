package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/graydraft/gateway/service/rerank"
)

// RerankHandler serves rerank API.
type RerankHandler struct {
	svc *rerank.Service
}

// NewRerankHandler creates handler.
func NewRerankHandler(svc *rerank.Service) *RerankHandler {
	return &RerankHandler{svc: svc}
}

type rerankRequest struct {
	Query      string             `json:"query" binding:"required"`
	Candidates []rerank.Candidate `json:"candidates" binding:"required"`
	Model      string             `json:"model"`
	TopN       int                `json:"top_n"`
}

// Rerank handles POST /v1/rerank
func (h *RerankHandler) Rerank(c *gin.Context) {
	var req rerankRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	results, model, err := h.svc.Rerank(c.Request.Context(), req.Query, req.Candidates, req.Model, req.TopN)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"results": results, "model": model})
}
