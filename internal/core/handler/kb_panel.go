package handler

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/graydraft/core/repository/postgres"
)

// KBPanelHandler serves GrayDraft panel document CRUD under /api/v1/kb/.
type KBPanelHandler struct {
	docs *postgres.DocumentRepo
}

// NewKBPanelHandler creates panel handler.
func NewKBPanelHandler(docs *postgres.DocumentRepo) *KBPanelHandler {
	return &KBPanelHandler{docs: docs}
}

// EnsureKBType handles POST /api/v1/kb/types/ensure
func (h *KBPanelHandler) EnsureKBType(c *gin.Context) {
	var req struct {
		Name        string `json:"name" binding:"required"`
		Description string `json:"description"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := h.docs.EnsureKBType(c.Request.Context(), req.Name, req.Description); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"name": req.Name, "ensured": true})
}

// ListDocuments handles GET /api/v1/kb/:kb_type/documents
func (h *KBPanelHandler) ListDocuments(c *gin.Context) {
	kbType := c.Param("kb_type")
	if kbType == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "missing kb_type"})
		return
	}
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "50"))
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))
	var indexReady *bool
	switch c.Query("index_ready") {
	case "true":
		v := true
		indexReady = &v
	case "false":
		v := false
		indexReady = &v
	}

	items, total, err := h.docs.ListPanelDocuments(c.Request.Context(), kbType, limit, offset, indexReady)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"kb_type": kbType,
		"total":   total,
		"items":   items,
	})
}

// GetDocument handles GET /api/v1/kb/:kb_type/documents/:doc_id
func (h *KBPanelHandler) GetDocument(c *gin.Context) {
	kbType := c.Param("kb_type")
	docID := c.Param("doc_id")
	doc, err := h.docs.GetPanelDocument(c.Request.Context(), kbType, docID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if doc == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "document not found"})
		return
	}
	c.JSON(http.StatusOK, doc)
}

// DeleteDocument handles DELETE /api/v1/kb/:kb_type/documents/:doc_id
func (h *KBPanelHandler) DeleteDocument(c *gin.Context) {
	kbType := c.Param("kb_type")
	docID := c.Param("doc_id")
	ok, err := h.docs.DeletePanelDocument(c.Request.Context(), kbType, docID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if !ok {
		c.JSON(http.StatusNotFound, gin.H{"error": "document not found"})
		return
	}
	c.Status(http.StatusNoContent)
}

// DocumentGraph handles GET /api/v1/kb/:kb_type/documents/graph
func (h *KBPanelHandler) DocumentGraph(c *gin.Context) {
	kbType := c.Param("kb_type")
	if kbType == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "missing kb_type"})
		return
	}
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "120"))
	nodes, err := h.docs.ListPanelDocumentGraph(c.Request.Context(), kbType, limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"kb_type": kbType,
		"total":   len(nodes),
		"nodes":   nodes,
	})
}
