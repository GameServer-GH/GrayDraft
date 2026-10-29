package nexus

import (
	"context"
	"fmt"
)

// KnowledgeGraphNode is one document with optional embedding for galaxy layout.
type KnowledgeGraphNode struct {
	ID         string          `json:"id"`
	Title      string          `json:"title"`
	Extensions map[string]any  `json:"extensions"`
	Embedding  []float32       `json:"embedding,omitempty"`
}

// KnowledgeGraphResponse is GET .../documents/graph.
type KnowledgeGraphResponse struct {
	KBType string               `json:"kb_type"`
	Total  int                  `json:"total"`
	Nodes  []KnowledgeGraphNode `json:"nodes"`
}

// GetDocumentGraph loads indexed documents with representative embeddings.
func (c *Client) GetDocumentGraph(ctx context.Context, scope KnowledgeScope, limit int) (*KnowledgeGraphResponse, error) {
	if limit <= 0 {
		limit = 120
	}
	url := fmt.Sprintf("%s/api/v1/tenants/%s/kbs/%s/documents/graph?limit=%d",
		c.base(), scope.TenantID, scope.KBID, limit)
	var out KnowledgeGraphResponse
	if err := c.getJSON(ctx, url, &out); err != nil {
		return nil, err
	}
	return &out, nil
}
