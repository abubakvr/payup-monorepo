package middleware

import (
	"net/http"
	"strings"

	"github.com/abubakvr/payup-backend/services/admin/internal/auth"
	"github.com/abubakvr/payup-backend/services/admin/internal/model"
	"github.com/gin-gonic/gin"
)

// RequireAdmin validates Bearer token and sets admin claims in context. Use for protected admin routes.
func RequireAdmin() gin.HandlerFunc {
	return func(c *gin.Context) {
		raw := c.GetHeader("Authorization")
		if raw == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "missing Authorization header"})
			return
		}
		const prefix = "Bearer "
		if !strings.HasPrefix(raw, prefix) {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid Authorization format"})
			return
		}
		token := strings.TrimPrefix(raw, prefix)
		claims, err := auth.ValidateToken(token)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid or expired token"})
			return
		}
		auth.SetClaims(c, claims)
		c.Next()
	}
}

// RequireSuperAdmin aborts with 403 if the authenticated admin is not a super_admin. Must be used after RequireAdmin.
func RequireSuperAdmin() gin.HandlerFunc {
	return func(c *gin.Context) {
		claims, ok := auth.ClaimsFrom(c)
		if !ok || claims == nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
			return
		}
		if claims.Role != model.RoleSuperAdmin {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
				"error":   "forbidden",
				"message": "only super admin can perform this action",
			})
			return
		}
		c.Next()
	}
}
