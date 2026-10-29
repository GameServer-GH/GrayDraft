package handler

import (
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/graydraft/core/auth"
	"github.com/graydraft/core/config"
)

// AuthHandler issues JWTs for Console / Admin access.
type AuthHandler struct {
	cfg *config.Config
}

// NewAuthHandler creates auth endpoints.
func NewAuthHandler(cfg *config.Config) *AuthHandler {
	return &AuthHandler{cfg: cfg}
}

type loginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type loginResponse struct {
	Token     string `json:"token"`
	ExpiresAt string `json:"expires_at"`
	Subject   string `json:"subject"`
}

// Login handles POST /api/v1/auth/login
func (h *AuthHandler) Login(c *gin.Context) {
	if !auth.Enabled(h.cfg.Auth.JWTSecret) {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"error": "jwt_auth_disabled",
			"hint":  "set JWT_SECRET to enable Console login",
		})
		return
	}
	var req loginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid body"})
		return
	}
	user := strings.TrimSpace(req.Username)
	pass := req.Password
	if user == "" || pass == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "username and password required"})
		return
	}
	if user != h.cfg.Auth.AdminUser || pass != h.cfg.Auth.AdminPassword {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid credentials"})
		return
	}
	ttl := time.Duration(h.cfg.Auth.TokenTTLHours) * time.Hour
	if ttl <= 0 {
		ttl = 24 * time.Hour
	}
	token, exp, err := auth.Issue(h.cfg.Auth.JWTSecret, user, ttl)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, loginResponse{
		Token:     token,
		ExpiresAt: exp.UTC().Format(time.RFC3339),
		Subject:   user,
	})
}

// Me handles GET /api/v1/auth/me (requires AdminAuth middleware).
func (h *AuthHandler) Me(c *gin.Context) {
	sub, _ := c.Get("admin_subject")
	c.JSON(http.StatusOK, gin.H{"subject": sub, "role": "admin"})
}

type serviceTokenRequest struct {
	Subject string `json:"subject"`
	TTLHours int   `json:"ttl_hours"`
}

// IssueServiceToken handles POST /api/v1/auth/service-token (AdminAuth).
// Mints an agent_service JWT for BFF / automation clients.
func (h *AuthHandler) IssueServiceToken(c *gin.Context) {
	if !auth.Enabled(h.cfg.Auth.JWTSecret) {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "jwt_auth_disabled"})
		return
	}
	var req serviceTokenRequest
	_ = c.ShouldBindJSON(&req)
	subject := strings.TrimSpace(req.Subject)
	if subject == "" {
		subject = "ghost-bff"
	}
	ttl := time.Duration(req.TTLHours) * time.Hour
	if ttl <= 0 {
		ttl = 12 * time.Hour
	}
	if ttl > 7*24*time.Hour {
		ttl = 7 * 24 * time.Hour
	}
	token, exp, err := auth.IssueService(h.cfg.Auth.JWTSecret, subject, ttl)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, loginResponse{
		Token:     token,
		ExpiresAt: exp.UTC().Format(time.RFC3339),
		Subject:   subject,
	})
}
