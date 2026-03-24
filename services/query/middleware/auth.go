package middleware

import (
	"log"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

// AuthConfig holds auth middleware configuration.
type AuthConfig struct {
	DevToken string // If set, accepts this as a valid Bearer token (dev mode only)
	DevMode  bool   // Must be explicitly true to allow unauthenticated requests
}

// Auth returns a Gin middleware that validates auth tokens.
// Phase 1: Accepts a static dev token from config.
// Production: Will validate JWTs from Ory Hydra.
func Auth(cfg AuthConfig) gin.HandlerFunc {
	return func(c *gin.Context) {
		auth := c.GetHeader("Authorization")

		// No auth header — only allow in explicit dev mode
		if auth == "" {
			if cfg.DevMode && cfg.DevToken != "" {
				log.Println("WARN: unauthenticated request — dev mode")
				c.Set("user_id", "dev-user")
				c.Set("tenant_id", "public")
				c.Set("tier", "free")
				c.Next()
				return
			}
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error": "missing Authorization header",
			})
			return
		}

		// Extract Bearer token
		token := strings.TrimPrefix(auth, "Bearer ")
		if token == auth {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error": "Authorization header must use Bearer scheme",
			})
			return
		}

		// Phase 1: Dev token validation
		if cfg.DevToken != "" && token == cfg.DevToken {
			log.Println("WARN: dev token used — disable before prod")
			c.Set("user_id", "dev-user")
			c.Set("tenant_id", "public")
			c.Set("tier", "pro")
			c.Next()
			return
		}

		// TODO: Phase 2 — JWT validation via Ory Hydra
		// For now, reject unknown tokens
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
			"error": "invalid token",
		})
	}
}
