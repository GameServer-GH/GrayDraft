package middleware

import (
	"github.com/gin-gonic/gin"
	"github.com/graydraft/core/auth"
	"net/http"
	"strings"
)

// AgentAuth establishes the trusted actor context for v2 Agent APIs. In
// production the actor comes from the signed JWT; development keeps header
// compatibility until the product identity provider is configured.
func AgentAuth(jwtSecret string) gin.HandlerFunc {
	return func(c *gin.Context) {
		if !auth.Enabled(jwtSecret) {
			c.Next()
			return
		}
		claims, err := auth.ValidateRoles(jwtSecret, auth.BearerToken(c.GetHeader("Authorization")), auth.RoleAdmin, auth.RoleAgent)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
			return
		}
		tenant := strings.TrimSpace(c.GetHeader("X-Tenant-ID"))
		if tenant == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "tenant context required"})
			return
		}
		c.Set("agent_tenant_id", tenant)
		c.Set("agent_actor_id", claims.Subject)
		c.Next()
	}
}
