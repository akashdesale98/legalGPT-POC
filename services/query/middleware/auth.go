package middleware

import (
	"crypto/rsa"
	"log"
	"net/http"
	"os"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
)

// AuthConfig holds auth middleware configuration.
type AuthConfig struct {
	DevToken  string // Static dev token (dev mode only)
	DevMode   bool   // Must be true to allow dev token path
	PublicKey *rsa.PublicKey
}

// LegalGPTClaims are the JWT claims embedded in every access token.
type LegalGPTClaims struct {
	TenantID string `json:"tenant_id"`
	Role     string `json:"role"`
	jwt.RegisteredClaims
}

// LoadAuthConfig builds AuthConfig from path strings.
// publicKeyPath may be empty — JWT auth is then disabled (dev mode only).
func LoadAuthConfig(devToken string, devMode bool, publicKeyPath string) AuthConfig {
	cfg := AuthConfig{
		DevToken: devToken,
		DevMode:  devMode,
	}
	if publicKeyPath != "" {
		data, err := os.ReadFile(publicKeyPath)
		if err != nil {
			log.Printf("WARN: JWT public key load failed (%s): %v — JWT auth disabled", publicKeyPath, err)
			return cfg
		}
		key, err := jwt.ParseRSAPublicKeyFromPEM(data)
		if err != nil {
			log.Printf("WARN: JWT public key parse failed: %v — JWT auth disabled", err)
			return cfg
		}
		cfg.PublicKey = key
	}
	return cfg
}

// Auth returns a Gin middleware that validates Bearer tokens.
//   - DEV_MODE=true + matching DEV_TOKEN: inject fake admin claims.
//   - RS256 JWT (when public key configured): validate and extract claims.
//   - Otherwise: 401.
func Auth(cfg AuthConfig) gin.HandlerFunc {
	return func(c *gin.Context) {
		auth := c.GetHeader("Authorization")

		// No auth header
		if auth == "" {
			if cfg.DevMode && cfg.DevToken == "" {
				// Fully open dev mode (no token required)
				c.Set("user_id", "dev-user")
				c.Set("tenant_id", "dev-tenant")
				c.Set("role", "admin")
				c.Next()
				return
			}
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
			return
		}

		// Extract Bearer token — reject non-Bearer scheme
		token := strings.TrimPrefix(auth, "Bearer ")
		if token == auth {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
			return
		}

		// DEV_MODE: static dev token path
		if cfg.DevMode && cfg.DevToken != "" && token == cfg.DevToken {
			log.Println("WARN: dev token used — disable before prod")
			c.Set("user_id", "dev-user")
			c.Set("tenant_id", "dev-tenant")
			c.Set("role", "admin")
			c.Next()
			return
		}

		// Production: RS256 JWT validation
		if cfg.PublicKey == nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
			return
		}

		claims := &LegalGPTClaims{}
		_, err := jwt.ParseWithClaims(token, claims, func(t *jwt.Token) (any, error) {
			if _, ok := t.Method.(*jwt.SigningMethodRSA); !ok {
				return nil, jwt.ErrSignatureInvalid
			}
			return cfg.PublicKey, nil
		}, jwt.WithValidMethods([]string{"RS256"}))
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
			return
		}

		c.Set("user_id", claims.Subject)
		c.Set("tenant_id", claims.TenantID)
		c.Set("role", claims.Role)
		c.Next()
	}
}

// RequireRole returns a Gin middleware that enforces a specific role on the request context.
// Must be used after the Auth middleware which sets the "role" key.
func RequireRole(role string) gin.HandlerFunc {
	return func(c *gin.Context) {
		if r, _ := c.Get("role"); r != role {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "forbidden"})
			return
		}
		c.Next()
	}
}
