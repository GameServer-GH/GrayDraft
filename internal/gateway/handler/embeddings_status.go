package handler

import (
	"net/http"
	"sync/atomic"

	"github.com/gin-gonic/gin"
)

// ProviderStatus holds live provider feedback for V1-06.
type ProviderStatus struct {
	Available    bool `json:"available"`
	Slots        int  `json:"slots"`
	QueueDepth   int  `json:"queue_depth"`
	RetryAfterMs int  `json:"retry_after_ms"`
}

var currentStatus atomic.Value

func init() {
	currentStatus.Store(ProviderStatus{
		Available:    true,
		Slots:        20,
		QueueDepth:   0,
		RetryAfterMs: 0,
	})
}

// SetProviderStatus updates the live provider status.
func SetProviderStatus(s ProviderStatus) {
	currentStatus.Store(s)
}

// GetProviderStatus returns current provider feedback.
func GetProviderStatus() ProviderStatus {
	v := currentStatus.Load()
	if v == nil {
		return ProviderStatus{Available: true, Slots: 10}
	}
	return v.(ProviderStatus)
}

// EmbeddingStatus handles GET /v1/embeddings/status (V1-06).
func (h *EmbeddingsHandler) EmbeddingStatus(c *gin.Context) {
	s := GetProviderStatus()
	c.JSON(http.StatusOK, gin.H{
		"provider": gin.H{
			"available":      s.Available,
			"slots":          s.Slots,
			"queue_depth":    s.QueueDepth,
			"retry_after_ms": s.RetryAfterMs,
		},
	})
}
