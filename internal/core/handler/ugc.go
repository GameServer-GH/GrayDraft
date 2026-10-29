package handler

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/graydraft/core/model"
	"github.com/graydraft/core/observability"
	"github.com/graydraft/core/service/kbscope"
	"github.com/graydraft/core/service/ugc"
)

// UGCHandler handles learning-note ingest callbacks.
type UGCHandler struct {
	svc      *ugc.Service
	resolver *kbscope.Resolver
}

// NewUGCHandler creates UGC handler.
func NewUGCHandler(svc *ugc.Service, resolver *kbscope.Resolver) *UGCHandler {
	return &UGCHandler{svc: svc, resolver: resolver}
}

// Notify handles POST /api/v1/ugc/notify
func (h *UGCHandler) Notify(c *gin.Context) {
	var req model.UGCNotifyRequest
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

	resp, err := h.svc.Notify(c.Request.Context(), req)
	channel := ""
	if resp != nil {
		channel = resp.Channel
	}
	observability.RecordUGCNotify(channel, err)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusAccepted, resp)
}

// TaskStatus handles GET /api/v1/ugc/tasks/:task_id
func (h *UGCHandler) TaskStatus(c *gin.Context) {
	taskID := c.Param("task_id")
	var (
		status map[string]any
		err    error
	)
	waited := c.Query("wait") == "true" || c.Query("wait") == "1"
	if waited {
		status, err = h.svc.WaitTaskQueryable(c.Request.Context(), taskID, taskWaitTimeout(c.Query("timeout_ms")), 500*time.Millisecond)
	} else {
		status, err = h.svc.GetTaskStatus(c.Request.Context(), taskID)
	}
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	if waited {
		recordTaskIndexLag(status)
	}
	c.JSON(http.StatusOK, status)
}

// TaskEvents handles GET /api/v1/ugc/tasks/:task_id/events as a readiness SSE stream.
func (h *UGCHandler) TaskEvents(c *gin.Context) {
	taskID := c.Param("task_id")
	timeout := taskWaitTimeout(c.Query("timeout_ms"))
	interval := taskEventInterval(c.Query("interval_ms"))

	c.Header("Content-Type", "text/event-stream; charset=utf-8")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("X-Accel-Buffering", "no")

	flusher, ok := c.Writer.(http.Flusher)
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "streaming not supported"})
		return
	}

	ctx := c.Request.Context()
	deadline := time.NewTimer(timeout)
	defer deadline.Stop()
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	emit := func(event string, payload map[string]any) bool {
		data, _ := json.Marshal(payload)
		_, _ = c.Writer.Write([]byte("event: "))
		_, _ = c.Writer.Write([]byte(event))
		_, _ = c.Writer.Write([]byte("\n"))
		_, _ = c.Writer.Write([]byte("data: "))
		_, _ = c.Writer.Write(data)
		_, _ = c.Writer.Write([]byte("\n\n"))
		flusher.Flush()
		return true
	}

	for {
		status, err := h.svc.GetTaskStatus(ctx, taskID)
		if err != nil {
			emit("error", gin.H{"error": err.Error(), "task_id": taskID})
			return
		}
		recordTaskIndexLag(status)
		emit("status", status)
		if ugc.IsTaskQueryableOrTerminal(status) {
			emit("done", status)
			return
		}
		select {
		case <-ctx.Done():
			return
		case <-deadline.C:
			status["wait_timeout"] = true
			status["wait_timeout_ms"] = timeout.Milliseconds()
			emit("timeout", status)
			return
		case <-ticker.C:
		}
	}
}

func taskWaitTimeout(raw string) time.Duration {
	if raw == "" {
		return 10 * time.Second
	}
	n, err := strconv.Atoi(raw)
	if err != nil || n <= 0 {
		return 10 * time.Second
	}
	if n > 30000 {
		n = 30000
	}
	return time.Duration(n) * time.Millisecond
}

func taskEventInterval(raw string) time.Duration {
	return clampMilliseconds(raw, 500, 100, 5000)
}

func clampMilliseconds(raw string, fallback, minValue, maxValue int) time.Duration {
	if raw == "" {
		return time.Duration(fallback) * time.Millisecond
	}
	n, err := strconv.Atoi(raw)
	if err != nil || n <= 0 {
		return time.Duration(fallback) * time.Millisecond
	}
	if n < minValue {
		n = minValue
	}
	if n > maxValue {
		n = maxValue
	}
	return time.Duration(n) * time.Millisecond
}

func statusString(status map[string]any, key string) string {
	if status == nil {
		return ""
	}
	v, _ := status[key].(string)
	return v
}

func recordTaskIndexLag(status map[string]any) {
	if q, _ := status["queryable"].(bool); !q {
		return
	}
	observability.RecordUGCTaskIndexLag(
		statusString(status, "kb_type"),
		statusString(status, "readiness"),
		status["created_at"],
		time.Now(),
	)
}
