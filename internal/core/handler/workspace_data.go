package handler

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/graydraft/core/repository/postgres"
	"github.com/graydraft/core/service/answercache"
	"github.com/graydraft/core/service/cache"
)

// WorkspaceDataHandler exposes workspace-scoped data stats and purge for GrayDraft.
type WorkspaceDataHandler struct {
	docs        *postgres.DocumentRepo
	cacheMgr    *cache.Manager
	answerCache *answercache.Manager
}

// NewWorkspaceDataHandler creates handler.
func NewWorkspaceDataHandler(
	docs *postgres.DocumentRepo,
	cacheMgr *cache.Manager,
	answerCache *answercache.Manager,
) *WorkspaceDataHandler {
	return &WorkspaceDataHandler{docs: docs, cacheMgr: cacheMgr, answerCache: answerCache}
}

// Stats handles GET /api/v1/kb/:kb_type/workspace-data/stats
func (h *WorkspaceDataHandler) Stats(c *gin.Context) {
	kbType := strings.TrimSpace(c.Param("kb_type"))
	if kbType == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "missing kb_type"})
		return
	}
	ctx := c.Request.Context()

	docCount, err := h.docs.CountPanelDocuments(ctx, kbType)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	retrieval := 0
	if h.cacheMgr != nil {
		retrieval, err = h.cacheMgr.RedisStatsByKBType(ctx, kbType)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
	}

	answers := 0
	if h.answerCache != nil {
		answers, err = h.answerCache.RedisStatsByKBType(ctx, kbType)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"kb_type":                  kbType,
		"knowledge_documents":      docCount,
		"redis_retrieval_entries":  retrieval,
		"redis_answer_entries":     answers,
	})
}

// Purge handles POST /api/v1/kb/:kb_type/workspace-data/purge
func (h *WorkspaceDataHandler) Purge(c *gin.Context) {
	kbType := strings.TrimSpace(c.Param("kb_type"))
	if kbType == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "missing kb_type"})
		return
	}
	var req struct {
		Target string `json:"target"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	target := strings.ToLower(strings.TrimSpace(req.Target))
	if target != "knowledge" && target != "redis" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "target must be knowledge or redis"})
		return
	}

	ctx := c.Request.Context()
	out := gin.H{"kb_type": kbType, "target": target}

	switch target {
	case "knowledge":
		n, err := h.docs.PurgePanelDocuments(ctx, kbType)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		out["documents_removed"] = n
	case "redis":
		retrievalRemoved := 0
		answerRemoved := 0
		var err error
		if h.cacheMgr != nil {
			retrievalRemoved, err = h.cacheMgr.RedisPurgeByKBType(ctx, kbType)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				return
			}
		}
		if h.answerCache != nil {
			answerRemoved, err = h.answerCache.RedisPurgeByKBType(ctx, kbType)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				return
			}
		}
		out["retrieval_keys_removed"] = retrievalRemoved
		out["answer_keys_removed"] = answerRemoved
	}

	c.JSON(http.StatusOK, out)
}
