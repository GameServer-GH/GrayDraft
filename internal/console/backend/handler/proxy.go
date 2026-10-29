package handler

import (
	"io"
	"log"
	"net/http"
	"strings"

	"github.com/google/uuid"

	"github.com/gin-gonic/gin"
)

// CoreProxy forwards /api/* to GrayDraft Core.
type CoreProxy struct {
	coreBase string
	client   *http.Client
}

// NewCoreProxy creates a reverse proxy handler.
func NewCoreProxy(coreBase string) *CoreProxy {
	return &CoreProxy{
		coreBase: strings.TrimRight(coreBase, "/"),
		client:   &http.Client{},
	}
}

// Forward proxies the incoming request to Core preserving path and query.
func (p *CoreProxy) Forward(c *gin.Context) {
	target := p.coreBase + c.Request.URL.Path
	if c.Request.URL.RawQuery != "" {
		target += "?" + c.Request.URL.RawQuery
	}

	req, err := http.NewRequestWithContext(c.Request.Context(), c.Request.Method, target, c.Request.Body)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
		return
	}
	for k, vals := range c.Request.Header {
		if strings.EqualFold(k, "Host") {
			continue
		}
		for _, v := range vals {
			req.Header.Add(k, v)
		}
	}
	traceID := strings.TrimSpace(req.Header.Get("X-Trace-Id"))
	if traceID == "" {
		traceID = strings.TrimSpace(req.Header.Get("X-Request-ID"))
	}
	if traceID == "" {
		traceID = uuid.NewString()
	}
	req.Header.Set("X-Trace-Id", traceID)
	req.Header.Set("X-Request-ID", traceID)
	c.Writer.Header().Set("X-Trace-Id", traceID)
	c.Writer.Header().Set("X-Request-ID", traceID)

	resp, err := p.client.Do(req)
	if err != nil {
		log.Printf("core proxy failed trace_id=%s method=%s path=%s error=%v", traceID, c.Request.Method, c.Request.URL.Path, err)
		c.JSON(http.StatusBadGateway, gin.H{"error": "core unreachable: " + err.Error()})
		return
	}
	defer resp.Body.Close()

	for k, vals := range resp.Header {
		for _, v := range vals {
			c.Writer.Header().Add(k, v)
		}
	}
	c.Status(resp.StatusCode)
	fields := proxyLogFields(c)
	log.Printf(
		"core proxy trace_id=%s method=%s path=%s status=%d task_id=%s doc_id=%s kb_type=%s readiness=%s",
		traceID,
		c.Request.Method,
		c.Request.URL.Path,
		resp.StatusCode,
		fields.taskID,
		fields.docID,
		fields.kbType,
		fields.readiness,
	)
	_, _ = io.Copy(c.Writer, resp.Body)
}

type proxyLogContext struct {
	taskID    string
	docID     string
	kbType    string
	readiness string
}

func proxyLogFields(c *gin.Context) proxyLogContext {
	out := proxyLogContext{
		docID:     strings.TrimSpace(c.Query("doc_id")),
		kbType:    strings.TrimSpace(c.Query("kb_type")),
		readiness: strings.TrimSpace(c.Query("readiness")),
	}
	parts := strings.Split(strings.Trim(c.Request.URL.Path, "/"), "/")
	for i, p := range parts {
		if p == "tasks" && i+1 < len(parts) {
			out.taskID = parts[i+1]
		}
		if p == "kb" && i+1 < len(parts) && out.kbType == "" {
			out.kbType = parts[i+1]
		}
		if p == "documents" && i+1 < len(parts) && out.docID == "" {
			out.docID = parts[i+1]
		}
	}
	return out
}
