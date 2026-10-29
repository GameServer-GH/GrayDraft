package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/graydraft/core/service/classifier"
)

// ClassifyHandler exposes query classification for debugging.
type ClassifyHandler struct {
	clf *classifier.Classifier
}

// NewClassifyHandler creates classify handler.
func NewClassifyHandler(clf *classifier.Classifier) *ClassifyHandler {
	return &ClassifyHandler{clf: clf}
}

type classifyRequest struct {
	Query string `json:"query" binding:"required"`
}

// Classify handles POST /api/v1/classifier/preview
func (h *ClassifyHandler) Classify(c *gin.Context) {
	var req classifyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if h.clf == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "classifier disabled"})
		return
	}
	plan := h.clf.ClassifyAndRoute(req.Query)
	c.JSON(http.StatusOK, plan)
}
