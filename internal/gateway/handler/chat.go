package handler

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/graydraft/gateway/service/llm"
	"github.com/graydraft/gateway/service/provider"
)

// ChatHandler serves chat completion API.
type ChatHandler struct {
	svc *llm.Service
}

// NewChatHandler creates chat handler.
func NewChatHandler(svc *llm.Service) *ChatHandler {
	return &ChatHandler{svc: svc}
}

type streamDeltaJSON struct {
	Content          string `json:"content,omitempty"`
	ReasoningContent string `json:"reasoning_content,omitempty"`
}

type streamChunkJSON struct {
	ID      string `json:"id"`
	Object  string `json:"object"`
	Choices []struct {
		Index        int             `json:"index"`
		Delta        streamDeltaJSON `json:"delta"`
		FinishReason *string         `json:"finish_reason,omitempty"`
	} `json:"choices"`
}

// Completions handles POST /v1/chat/completions (stream only for now).
func (h *ChatHandler) Completions(c *gin.Context) {
	// V1-30: circuit breaker check
	br := provider.GetBreaker("chat")
	if !br.Allow() {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "circuit breaker open", "retry_after_ms": 30000})
		return
	}
	var req llm.ChatRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if !req.Stream {
		c.JSON(http.StatusBadRequest, gin.H{"error": "only stream=true is supported"})
		return
	}

	c.Header("Content-Type", "text/event-stream; charset=utf-8")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("X-Accel-Buffering", "no")

	flusher, ok := c.Writer.(http.Flusher)
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "streaming not supported"})
		return
	}

	id := "chatcmpl-" + uuid.New().String()

	err := h.svc.StreamCompletion(c.Request.Context(), req, func(d llm.StreamDelta) error {
		if d.Done && d.FinishReason == "" {
			_, err := c.Writer.Write([]byte("data: [DONE]\n\n"))
			flusher.Flush()
			return err
		}
		if !d.Done && d.Content == "" && d.Reasoning == "" {
			return nil
		}
		chunk := streamChunkJSON{
			ID: id, Object: "chat.completion.chunk",
			Choices: []struct {
				Index        int             `json:"index"`
				Delta        streamDeltaJSON `json:"delta"`
				FinishReason *string         `json:"finish_reason,omitempty"`
			}{{
				Index: 0,
				Delta: streamDeltaJSON{
					Content:          d.Content,
					ReasoningContent: d.Reasoning,
				},
			}},
		}
		if d.FinishReason != "" {
			chunk.Choices[0].FinishReason = &d.FinishReason
		}
		data, _ := json.Marshal(chunk)
		if _, err := fmt.Fprintf(c.Writer, "data: %s\n\n", data); err != nil {
			return err
		}
		if d.Done {
			if _, err := c.Writer.Write([]byte("data: [DONE]\n\n")); err != nil {
				return err
			}
		}
		flusher.Flush()
		return nil
	})
	if err != nil {
		errObj, _ := json.Marshal(gin.H{"error": err.Error()})
		_, _ = fmt.Fprintf(c.Writer, "data: %s\n\n", errObj)
		flusher.Flush()
	}
}
