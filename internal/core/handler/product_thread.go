package handler

import (
	"encoding/json"
	"errors"
	"net/http"
	"regexp"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/graydraft/core/repository/postgres"
	"github.com/jackc/pgx/v5"
)

const maxProductThreadBytes = 400_000

var productThreadKeyPattern = regexp.MustCompile(`^[A-Za-z0-9._~%-]{1,200}$`)

type ProductThreadHandler struct{ repo *postgres.ProductThreadRepo }

func NewProductThreadHandler(repo *postgres.ProductThreadRepo) *ProductThreadHandler {
	return &ProductThreadHandler{repo: repo}
}

func (h *ProductThreadHandler) scope(c *gin.Context) (string, string, bool) {
	tenant := strings.TrimSpace(c.GetHeader("X-Tenant-ID"))
	actor := strings.TrimSpace(c.GetHeader("X-Actor-ID"))
	if v, ok := c.Get("agent_tenant_id"); ok {
		if s, _ := v.(string); strings.TrimSpace(s) != "" {
			tenant = strings.TrimSpace(s)
		}
	}
	if v, ok := c.Get("agent_actor_id"); ok {
		if s, _ := v.(string); strings.TrimSpace(s) != "" {
			actor = strings.TrimSpace(s)
		}
	}
	if tenant == "" || actor == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "X-Tenant-ID and X-Actor-ID are required"})
		return "", "", false
	}
	return tenant, actor, true
}

func (h *ProductThreadHandler) Get(c *gin.Context) {
	tenant, actor, ok := h.scope(c)
	if !ok {
		return
	}
	key := strings.TrimSpace(c.Query("thread_key"))
	if !productThreadKeyPattern.MatchString(key) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid thread_key"})
		return
	}
	row, err := h.repo.Get(c.Request.Context(), tenant, actor, key)
	if errors.Is(err, pgx.ErrNoRows) {
		c.JSON(http.StatusOK, gin.H{"thread": nil})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"thread_key": row.ThreadKey,
		"payload":    json.RawMessage(row.Payload),
		"updated_at": row.UpdatedAt,
	})
}

func (h *ProductThreadHandler) Put(c *gin.Context) {
	tenant, actor, ok := h.scope(c)
	if !ok {
		return
	}
	var in struct {
		ThreadKey string          `json:"thread_key"`
		Payload   json.RawMessage `json:"payload"`
	}
	if err := c.ShouldBindJSON(&in); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid body"})
		return
	}
	key := strings.TrimSpace(in.ThreadKey)
	if !productThreadKeyPattern.MatchString(key) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid thread_key"})
		return
	}
	if len(in.Payload) == 0 || len(in.Payload) > maxProductThreadBytes || !json.Valid(in.Payload) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid payload"})
		return
	}
	if err := h.repo.Put(c.Request.Context(), tenant, actor, key, in.Payload); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *ProductThreadHandler) Delete(c *gin.Context) {
	tenant, actor, ok := h.scope(c)
	if !ok {
		return
	}
	key := strings.TrimSpace(c.Query("thread_key"))
	var err error
	if key == "" {
		err = h.repo.DeleteAll(c.Request.Context(), tenant, actor)
	} else if !productThreadKeyPattern.MatchString(key) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid thread_key"})
		return
	} else {
		err = h.repo.Delete(c.Request.Context(), tenant, actor, key)
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.Status(http.StatusNoContent)
}
