package handler

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"os"

	"github.com/graydraft/core/model"
	"github.com/graydraft/core/observability"
	"github.com/graydraft/core/observability/agentlog"
	"github.com/graydraft/core/service/chat"
	"github.com/graydraft/core/service/dynconfig"
	"github.com/graydraft/core/service/kbscope"
)

// ChatHandler handles streaming RAG chat.
type ChatHandler struct {
	svc      *chat.Service
	resolver *kbscope.Resolver
	dynCfg   *dynconfig.Manager
}

// NewChatHandler creates chat handler.
func NewChatHandler(svc *chat.Service, resolver *kbscope.Resolver, dynCfg *dynconfig.Manager) *ChatHandler {
	return &ChatHandler{svc: svc, resolver: resolver, dynCfg: dynCfg}
}

// Stream handles POST /api/v1/chat/stream (SSE).
func (h *ChatHandler) Stream(c *gin.Context) {
	var req model.ChatRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	kbType, err := ResolveKBType(c, h.resolver, req.TenantID, req.KBID, req.KBType)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	req.KBType = kbType
	req = chat.ApplyEduProfile(req)
	// V1-39: workspace-level chat concurrency limit
	if h.dynCfg != nil && h.dynCfg.Hot().RateLimitEnabled == 1 {
		// simple concurrency gate: signal via header; detailed impl deferred to middleware
		c.Header("X-Rate-Limit-Remaining", fmt.Sprintf("%d", h.dynCfg.Hot().ChatConcurrency))
	}

	if err := chat.ValidateRequest(req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
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

	logRoot := strings.TrimSpace(os.Getenv("AGENT_LOG_ROOT"))
	if logRoot == "" {
		logRoot = "."
	}
	sess, _ := agentlog.StartFromContext(c.Request.Context(), logRoot, "chat-stream")
	var runErr error
	defer func() {
		if sess != nil {
			sess.Finish(runErr)
		}
	}()
	if sess != nil {
		sess.Step("request_in", map[string]any{
			"query":      req.Query,
			"kb_type":    req.KBType,
			"chat_mode":  req.ChatMode,
			"top_k":      req.TopK,
			"session_id": req.SessionID,
		})
	}

	start := time.Now()
	var (
		strategy       string
		cacheHit       string
		answerCacheHit string
		queryIntent    string
		chunksUsed     int
		streamErr      error
		recorded       bool
		answerBuilder  strings.Builder
	)

	record := func(err error) {
		if recorded {
			return
		}
		recorded = true
		observability.LogChat(observability.ChatRecord{
			KBType:            req.KBType,
			AnswerCacheHit:    answerCacheHit,
			RetrievalCacheHit: cacheHit,
			QueryIntent:       queryIntent,
			Strategy:          strategy,
			ChunksUsed:        chunksUsed,
			Duration:          time.Since(start),
			Err:               err,
		})
	}

	err = h.svc.Stream(c.Request.Context(), req, func(ev model.ChatStreamEvent) error {
		switch ev.Type {
		case "meta":
			strategy = ev.Strategy
			cacheHit = ev.CacheHit
			answerCacheHit = ev.AnswerCacheHit
			queryIntent = ev.QueryIntent
			chunksUsed = ev.ChunksUsed
			if sess != nil && len(ev.RetrievalChunkPreviews) > 0 {
				sess.Step("retrieval_chunks", map[string]any{
					"previews": ev.RetrievalChunkPreviews,
				})
			}
		case "chunk":
			answerBuilder.WriteString(ev.Delta)
		case "done":
			record(nil)
		case "error":
			streamErr = fmt.Errorf("%s", ev.Message)
		}
		if sess != nil {
			sess.Step("sse_event", chatEventLogFields(ev))
		}
		data, _ := json.Marshal(ev)
		_, _ = c.Writer.Write([]byte("data: "))
		_, _ = c.Writer.Write(data)
		_, _ = c.Writer.Write([]byte("\n\n"))
		flusher.Flush()
		return nil
	})

	if err != nil {
		runErr = err
		record(err)
		errEv, _ := json.Marshal(model.ChatStreamEvent{Type: "error", Message: err.Error()})
		_, _ = c.Writer.Write([]byte("data: "))
		_, _ = c.Writer.Write(errEv)
		_, _ = c.Writer.Write([]byte("\n\n"))
		flusher.Flush()
		return
	}
	if streamErr != nil {
		runErr = streamErr
		record(streamErr)
	}
	if !recorded {
		record(nil)
	}
	if sess != nil {
		answer := answerBuilder.String()
		if answer != "" {
			sess.Step("final_answer", map[string]any{
				"answer":     answer,
				"answer_len": len([]rune(answer)),
			})
		}
	}
}

func chatEventLogFields(ev model.ChatStreamEvent) map[string]any {
	fields := map[string]any{
		"type":     ev.Type,
		"strategy": ev.Strategy,
		"chunks":   ev.ChunksUsed,
	}
	switch ev.Type {
	case "meta":
		fields["cache_hit"] = ev.CacheHit
		fields["answer_cache_hit"] = ev.AnswerCacheHit
		fields["total"] = ev.Total
		fields["query_intent"] = ev.QueryIntent
		fields["confidence"] = ev.Confidence
		fields["gate_score"] = ev.GateScore
		fields["gate_threshold"] = ev.GateThreshold
		fields["gate_reason"] = ev.GateReason
		fields["semantic_4d_filter_applied"] = ev.Semantic4DFilter
		if ev.QuerySemantic4D != nil {
			fields["query_4d_method"] = ev.QuerySemantic4D.Method
			fields["query_4d_domain"] = ev.QuerySemantic4D.Domain
			fields["query_4d_entities"] = ev.QuerySemantic4D.Entities
			fields["query_4d_scene"] = ev.QuerySemantic4D.Scene
			fields["query_4d_style"] = ev.QuerySemantic4D.Style
		}
		if ev.RetrievalTrace != nil {
			fields["waterfall_tier"] = ev.RetrievalTrace.WaterfallTier
			fields["waterfall_suffix"] = ev.RetrievalTrace.WaterfallSuffix
			fields["entity_align"] = ev.RetrievalTrace.EntityAlign
			fields["entities_final"] = ev.RetrievalTrace.EntitiesFinal
		}
	case "done":
		fields["confidence"] = ev.Confidence
		fields["answer_confidence"] = ev.AnswerConfidence
		fields["sources"] = len(ev.Sources)
	case "error":
		fields["message"] = ev.Message
	}
	return fields
}
