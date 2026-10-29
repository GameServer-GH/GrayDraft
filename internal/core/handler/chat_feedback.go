package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/graydraft/core/model"
	"github.com/graydraft/core/service/chat"
)

// ChatFeedbackHandler handles answer feedback (up/down vote).
type ChatFeedbackHandler struct {
	svc *chat.FeedbackService
}

// NewChatFeedbackHandler creates the handler.
func NewChatFeedbackHandler(svc *chat.FeedbackService) *ChatFeedbackHandler {
	return &ChatFeedbackHandler{svc: svc}
}

// Submit handles POST /api/v1/chat/feedback
func (h *ChatFeedbackHandler) Submit(c *gin.Context) {
	var req model.ChatFeedbackRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := chat.ValidateFeedbackRequest(req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	resp, err := h.svc.ApplyFeedback(c.Request.Context(), req)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if !resp.Accepted {
		c.JSON(http.StatusOK, resp)
		return
	}
	c.JSON(http.StatusOK, resp)
}
