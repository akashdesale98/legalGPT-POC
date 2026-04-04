package middleware

import (
	"crypto/rand"
	"crypto/rsa"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
)

func init() {
	gin.SetMode(gin.TestMode)
}

func generateTestKey(t *testing.T) *rsa.PrivateKey {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generate rsa key: %v", err)
	}
	return key
}

func issueToken(t *testing.T, key *rsa.PrivateKey, tenantID, role, subject string, ttl time.Duration) string {
	t.Helper()
	now := time.Now()
	claims := struct {
		TenantID string `json:"tenant_id"`
		Role     string `json:"role"`
		jwt.RegisteredClaims
	}{
		TenantID: tenantID,
		Role:     role,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   subject,
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(ttl)),
		},
	}
	tok := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	signed, err := tok.SignedString(key)
	if err != nil {
		t.Fatalf("sign token: %v", err)
	}
	return signed
}

func doRequest(t *testing.T, cfg AuthConfig, token string) int {
	t.Helper()
	r := gin.New()
	r.GET("/test", Auth(cfg), func(c *gin.Context) {
		c.Status(http.StatusOK)
	})
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w.Code
}

func TestAuth_ValidToken(t *testing.T) {
	key := generateTestKey(t)
	cfg := AuthConfig{PublicKey: &key.PublicKey}
	token := issueToken(t, key, "tenant-1", "pro", "user-1", time.Hour)

	if got := doRequest(t, cfg, token); got != http.StatusOK {
		t.Fatalf("expected 200, got %d", got)
	}
}

func TestAuth_ExpiredToken(t *testing.T) {
	key := generateTestKey(t)
	cfg := AuthConfig{PublicKey: &key.PublicKey}
	token := issueToken(t, key, "tenant-1", "pro", "user-1", -time.Hour)

	if got := doRequest(t, cfg, token); got != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", got)
	}
}

func TestAuth_WrongSignature(t *testing.T) {
	key := generateTestKey(t)
	otherKey := generateTestKey(t)
	// Sign with otherKey but verify with key
	cfg := AuthConfig{PublicKey: &key.PublicKey}
	token := issueToken(t, otherKey, "tenant-1", "pro", "user-1", time.Hour)

	if got := doRequest(t, cfg, token); got != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", got)
	}
}

func TestAuth_MissingToken(t *testing.T) {
	key := generateTestKey(t)
	cfg := AuthConfig{PublicKey: &key.PublicKey}

	if got := doRequest(t, cfg, ""); got != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", got)
	}
}

func TestAuth_DevMode(t *testing.T) {
	cfg := AuthConfig{DevMode: true, DevToken: "dev-secret"}

	r := gin.New()
	r.GET("/test", Auth(cfg), func(c *gin.Context) {
		role, _ := c.Get("role")
		tenantID, _ := c.Get("tenant_id")
		if role != "admin" || tenantID != "dev-tenant" {
			c.Status(http.StatusInternalServerError)
			return
		}
		c.Status(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("Authorization", "Bearer dev-secret")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 with dev token, got %d", w.Code)
	}
}

func TestAuth_ClaimsInjected(t *testing.T) {
	key := generateTestKey(t)
	cfg := AuthConfig{PublicKey: &key.PublicKey}
	token := issueToken(t, key, "tenant-abc", "enterprise", "user-xyz", time.Hour)

	r := gin.New()
	r.GET("/test", Auth(cfg), func(c *gin.Context) {
		tenantID, _ := c.Get("tenant_id")
		role, _ := c.Get("role")
		userID, _ := c.Get("user_id")
		if tenantID != "tenant-abc" || role != "enterprise" || userID != "user-xyz" {
			c.JSON(http.StatusInternalServerError, gin.H{
				"tenant_id": tenantID,
				"role":      role,
				"user_id":   userID,
			})
			return
		}
		c.Status(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 with correct claims, got %d body=%s", w.Code, w.Body.String())
	}
}
