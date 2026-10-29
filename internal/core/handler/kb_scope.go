package handler

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/graydraft/core/model"
	"github.com/graydraft/core/repository/postgres"
	"github.com/graydraft/core/service/kbscope"
)

// KBScopeHandler serves tenant/knowledge-base PaaS APIs.
type KBScopeHandler struct {
	resolver *kbscope.Resolver
	docs     *postgres.DocumentRepo
}

// NewKBScopeHandler creates tenant KB handler.
func NewKBScopeHandler(resolver *kbscope.Resolver, docs *postgres.DocumentRepo) *KBScopeHandler {
	return &KBScopeHandler{resolver: resolver, docs: docs}
}

// EnsureTenant handles POST /api/v1/tenants/ensure
func (h *KBScopeHandler) EnsureTenant(c *gin.Context) {
	var req model.EnsureTenantRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := h.resolver.EnsureTenant(c.Request.Context(), req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"tenant_id": req.TenantID, "ensured": true})
}

// EnsureKnowledgeBase handles POST /api/v1/tenants/:tenant_id/kbs/ensure
func (h *KBScopeHandler) EnsureKnowledgeBase(c *gin.Context) {
	tenantID := c.Param("tenant_id")
	var req model.EnsureKnowledgeBaseRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	row, err := h.resolver.EnsureKnowledgeBase(c.Request.Context(), tenantID, req)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, row)
}

func (h *KBScopeHandler) resolvePartition(c *gin.Context) (tenantID, kbID, partition string, ok bool) {
	tenantID = c.Param("tenant_id")
	kbID = c.Param("kb_id")
	part, err := h.resolver.ResolveKBType(c.Request.Context(), tenantID, kbID, "")
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return "", "", "", false
	}
	return tenantID, kbID, part, true
}

// ListKnowledgeBases handles GET /api/v1/tenants/:tenant_id/kbs (O-13b).
func (h *KBScopeHandler) ListKnowledgeBases(c *gin.Context) {
	tenantID := c.Param("tenant_id")
	rows, err := h.resolver.ListKnowledgeBases(c.Request.Context(), tenantID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"tenant_id": tenantID, "items": rows})
}

// ListDocuments handles GET /api/v1/tenants/:tenant_id/kbs/:kb_id/documents
func (h *KBScopeHandler) ListDocuments(c *gin.Context) {
	tenantID, kbID, partition, ok := h.resolvePartition(c)
	if !ok {
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
	items, total, err := h.docs.ListPanelDocuments(c.Request.Context(), partition, limit, offset, indexReady)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"tenant_id":          tenantID,
		"kb_id":              kbID,
		"storage_partition":  partition,
		"kb_type":            partition,
		"total":              total,
		"items":              items,
	})
}

// GetDocument handles GET .../documents/:doc_id
func (h *KBScopeHandler) GetDocument(c *gin.Context) {
	_, _, partition, ok := h.resolvePartition(c)
	if !ok {
		return
	}
	docID := c.Param("doc_id")
	doc, err := h.docs.GetPanelDocument(c.Request.Context(), partition, docID)
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

// DeleteDocument handles DELETE .../documents/:doc_id
func (h *KBScopeHandler) DeleteDocument(c *gin.Context) {
	_, _, partition, ok := h.resolvePartition(c)
	if !ok {
		return
	}
	docID := c.Param("doc_id")
	okDel, err := h.docs.DeletePanelDocument(c.Request.Context(), partition, docID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if !okDel {
		c.JSON(http.StatusNotFound, gin.H{"error": "document not found"})
		return
	}
	c.Status(http.StatusNoContent)
}

// DocumentGraph handles GET .../documents/graph
func (h *KBScopeHandler) DocumentGraph(c *gin.Context) {
	tenantID, kbID, partition, ok := h.resolvePartition(c)
	if !ok {
		return
	}
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "120"))
	nodes, err := h.docs.ListPanelDocumentGraph(c.Request.Context(), partition, limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"tenant_id":         tenantID,
		"kb_id":             kbID,
		"storage_partition": partition,
		"kb_type":           partition,
		"total":             len(nodes),
		"nodes":             nodes,
	})
}
func ResolveKBType(c *gin.Context, resolver *kbscope.Resolver, tenantID, kbID, legacyKBType string) (string, error) {
	if tenantID == "" && kbID == "" {
		tenantID = c.GetHeader("X-GrayDraft-Tenant-Id")
		kbID = c.GetHeader("X-GrayDraft-KB-Id")
	}
	return resolver.ResolveKBType(c.Request.Context(), tenantID, kbID, legacyKBType)
}
