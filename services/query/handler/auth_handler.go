package handler

import (
	"context"
	"crypto/rsa"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"

	"testqdrant/internal/store"
)

const (
	tokenTTL    = time.Hour
	bcryptCost  = 12
)

// tokenStore is the minimal DB interface the AuthHandler needs.
type tokenStore interface {
	GetOrCreateTenant(ctx context.Context, slug, name, plan string) (*store.Tenant, error)
	GetUserByEmailAndTenant(ctx context.Context, tenantID, email string) (*store.User, error)
}

// AuthHandler issues RS256 JWTs via client_credentials flow.
type AuthHandler struct {
	privateKey *rsa.PrivateKey
	db         tokenStore
}

// NewAuthHandler loads the RSA private key and wires the DB.
// Returns nil if the key cannot be loaded (caller should check).
func NewAuthHandler(privateKeyPath string, db tokenStore) *AuthHandler {
	data, err := os.ReadFile(privateKeyPath)
	if err != nil {
		log.Printf("AUTH: private key load failed (%s): %v — token issuance disabled", privateKeyPath, err)
		return nil
	}
	key, err := jwt.ParseRSAPrivateKeyFromPEM(data)
	if err != nil {
		log.Printf("AUTH: private key parse failed: %v — token issuance disabled", err)
		return nil
	}
	return &AuthHandler{privateKey: key, db: db}
}

// TokenRequest is the body for POST /auth/token.
type TokenRequest struct {
	ClientID     string `json:"client_id" binding:"required"`     // user email
	ClientSecret string `json:"client_secret" binding:"required"` // raw token/password
	TenantSlug   string `json:"tenant_slug" binding:"required"`
}

// TokenResponse is returned on successful authentication.
type TokenResponse struct {
	AccessToken string `json:"access_token"`
	ExpiresIn   int    `json:"expires_in"`
	TokenType   string `json:"token_type"`
}

// IssueToken handles POST /api/v1/auth/token.
// Validates client_id (email) + client_secret (bcrypt) against the users table,
// then issues an RS256 JWT with tenant_id and role claims.
func (h *AuthHandler) IssueToken(c *gin.Context) {
	var req TokenRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	ctx := c.Request.Context()

	// Resolve tenant
	tenant, err := h.db.GetOrCreateTenant(ctx, req.TenantSlug, req.TenantSlug, "free")
	if err != nil {
		log.Printf("AUTH: GetOrCreateTenant slug=%s err=%v", req.TenantSlug, err)
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	// Fetch user
	user, err := h.db.GetUserByEmailAndTenant(ctx, tenant.ID, req.ClientID)
	if err != nil {
		// Avoid timing differences that leak user existence
		_ = bcrypt.CompareHashAndPassword([]byte("$2a$12$dummy"), []byte(req.ClientSecret))
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	// Verify tenant ownership
	if user.TenantID != tenant.ID {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	// Verify secret
	if err := bcrypt.CompareHashAndPassword([]byte(user.HashedToken), []byte(req.ClientSecret)); err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	// Issue token
	now := time.Now()
	claims := struct {
		TenantID string `json:"tenant_id"`
		Role     string `json:"role"`
		jwt.RegisteredClaims
	}{
		TenantID: tenant.ID,
		Role:     user.Role,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   user.ID,
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(tokenTTL)),
		},
	}

	tok := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	signed, err := tok.SignedString(h.privateKey)
	if err != nil {
		log.Printf("AUTH: sign token err=%v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "token signing failed"})
		return
	}

	c.JSON(http.StatusOK, TokenResponse{
		AccessToken: signed,
		ExpiresIn:   int(tokenTTL.Seconds()),
		TokenType:   "Bearer",
	})
}

// HashSecret bcrypt-hashes a raw secret at cost 12. Used during user provisioning.
func HashSecret(secret string) (string, error) {
	h, err := bcrypt.GenerateFromPassword([]byte(secret), bcryptCost)
	if err != nil {
		return "", fmt.Errorf("hash secret: %w", err)
	}
	return string(h), nil
}
