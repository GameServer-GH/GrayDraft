package handler

import (
	"net/http"
	"os"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/graydraft/core/model"
	"github.com/graydraft/core/observability/agentlog"
	"github.com/graydraft/core/service/kbscope"
	"github.com/graydraft/core/service/retriever"
)

// RetrievalHandler handles search API requests.
type RetrievalHandler struct {
	retriever *retriever.Hybrid
	resolver  *kbscope.Resolver
}

// NewRetrievalHandler creates a retrieval handler.
func NewRetrievalHandler(r *retriever.Hybrid, resolver *kbscope.Resolver) *RetrievalHandler {
	return &RetrievalHandler{retriever: r, resolver: resolver}
}

// Search handles POST /api/v1/retrieval/search
func (h *RetrievalHandler) Search(c *gin.Context) {
	var req model.SearchRequest
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

	logRoot := strings.TrimSpace(os.Getenv("AGENT_LOG_ROOT"))
	if logRoot == "" {
		logRoot = "."
	}
	sess, _ := agentlog.StartFromContext(c.Request.Context(), logRoot, "retrieval-search")
	var runErr error
	defer func() {
		if sess != nil {
			sess.Finish(runErr)
		}
	}()
	if sess != nil {
		sess.Step("request_in", map[string]any{
			"query":             req.Query,
			"kb_type":           req.KBType,
			"top_k":             req.TopK,
			"workspace_context": len(strings.TrimSpace(req.WorkspaceContext)),
		})
	}

	resp, err := h.retriever.Search(c.Request.Context(), req)
	if err != nil {
		runErr = err
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if sess != nil {
		sess.StepData("response_out", resp)
	}
	c.JSON(http.StatusOK, resp)
}

// Diagnose handles POST /api/v1/retrieval/diagnose
func (h *RetrievalHandler) Diagnose(c *gin.Context) {
	var req model.SearchRequest
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
	if req.TopK <= 0 {
		req.TopK = 5
	}
	resp, err := h.retriever.Diagnose(c.Request.Context(), req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, resp)
}
