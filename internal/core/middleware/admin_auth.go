package middleware

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/graydraft/core/auth"
)

// AdminAuth protects Admin routes when JWT_SECRET is configured.
func AdminAuth(jwtSecret string) gin.HandlerFunc {
	return func(c *gin.Context) {
		if !auth.Enabled(jwtSecret) {
			c.Next()
			return
		}
		tok := auth.BearerToken(c.GetHeader("Authorization"))
		claims, err := auth.Validate(jwtSecret, tok)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error": "unauthorized",
				"hint":  "POST /api/v1/auth/login with valid credentials",
			})
			return
		}
		c.Set("admin_subject", claims.Subject)
		c.Next()
	}
}
