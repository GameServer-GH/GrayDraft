package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// BatchedEmbedItem mirrors the per-item status in a batch embedding response.
type BatchedEmbedItem struct {
	Index     int        `json:"index"`
	Ok        bool       `json:"ok"`
	Embedding []float32  `json:"embedding,omitempty"`
	ErrorCode string     `json:"error_code,omitempty"`
	Retryable bool       `json:"retryable,omitempty"`
}

type batchedEmbedResponse struct {
	Model     string             `json:"model"`
	Dimension int                `json:"dimension"`
	Items     []BatchedEmbedItem `json:"items"`
}

// EmbedBatch handles POST /v1/embeddings/batch with per-item status.
// V1-04: allows partial failure reporting.
func (h *EmbeddingsHandler) EmbedBatch(c *gin.Context) {
	var req embedRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	vecs, model, err := h.svc.Embed(c.Request.Context(), req.Texts, req.Model)
	dim := 0
	if len(vecs) > 0 && len(vecs[0]) > 0 {
		dim = len(vecs[0])
	}

	resp := batchedEmbedResponse{
		Model:     model,
		Dimension: dim,
		Items:     make([]BatchedEmbedItem, len(req.Texts)),
	}
	for i := range req.Texts {
		if err != nil {
			resp.Items[i] = BatchedEmbedItem{
				Index:     i,
				Ok:        false,
				ErrorCode: "gateway_error",
				Retryable: true,
			}
		} else if i < len(vecs) && len(vecs[i]) > 0 {
			resp.Items[i] = BatchedEmbedItem{
				Index:     i,
				Ok:        true,
				Embedding: vecs[i],
			}
		} else {
			resp.Items[i] = BatchedEmbedItem{
				Index:     i,
				Ok:        false,
				ErrorCode: "empty_vector",
				Retryable: true,
			}
		}
	}
	c.JSON(http.StatusOK, resp)
}
