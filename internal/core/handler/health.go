package handler

import (
	"context"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/graydraft/core/repository/postgres"
)

// HealthHandler serves health endpoints.
type HealthHandler struct {
	db          *postgres.Client
	nats        NATSPinger
	cache       CachePinger
	answerCache CachePinger
}

// NATSPinger checks NATS connectivity.
type NATSPinger interface {
	Ping() error
}

// CachePinger checks Redis cache connectivity.
type CachePinger interface {
	Ping(ctx context.Context) error
}

// NewHealthHandler creates a health handler.
func NewHealthHandler(db *postgres.Client, nats NATSPinger, cache CachePinger, answerCache CachePinger) *HealthHandler {
	return &HealthHandler{db: db, nats: nats, cache: cache, answerCache: answerCache}
}

// Health returns service and database status.
func (h *HealthHandler) Health(c *gin.Context) {
	status := "ok"
	dbStatus := "ok"
	code := http.StatusOK

	ctx, cancel := context.WithTimeout(c.Request.Context(), 2*time.Second)
	defer cancel()
	if err := h.db.Pool().Ping(ctx); err != nil {
		dbStatus = "unavailable"
		status = "degraded"
		code = http.StatusServiceUnavailable
	}

	natsStatus := "disabled"
	if h.nats != nil {
		natsStatus = "ok"
		if jp, ok := h.nats.(interface{ JetStreamEnabled() bool }); ok && jp.JetStreamEnabled() {
			natsStatus = "jetstream"
		}
		if err := h.nats.Ping(); err != nil {
			natsStatus = "unavailable"
			if status == "ok" {
				status = "degraded"
			}
		}
	}

	cacheStatus := "disabled"
	if h.cache != nil {
		cacheStatus = "ok"
		if err := h.cache.Ping(ctx); err != nil {
			cacheStatus = "unavailable"
			if status == "ok" {
				status = "degraded"
			}
		}
	}

	answerCacheStatus := "disabled"
	if h.answerCache != nil {
		answerCacheStatus = "ok"
		if err := h.answerCache.Ping(ctx); err != nil {
			answerCacheStatus = "unavailable"
			if status == "ok" {
				status = "degraded"
			}
		}
	}

	c.JSON(code, gin.H{
		"status":       status,
		"service":      "graydraft-core",
		"database":     dbStatus,
		"nats":         natsStatus,
		"cache":        cacheStatus,
		"answer_cache": answerCacheStatus,
	})
}
