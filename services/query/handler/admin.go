package handler

import (
	"context"
	"log"
	"net/http"

	"github.com/gin-gonic/gin"

	"testqdrant/internal/store"
)

// queryHistoryStore is the minimal interface the AdminHandler needs.
type queryHistoryStore interface {
	GetQueryHistory(ctx context.Context, tenantID string, limit int) ([]store.QueryHistoryEntry, error)
}

// AdminHandler handles admin-only endpoints.
type AdminHandler struct {
	DB queryHistoryStore
}

// RoutingStats returns routing distribution for last 1000 queries.
// GET /admin/routing-stats — requires role=admin
func (h *AdminHandler) RoutingStats(c *gin.Context) {
	if h.DB == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "database not available"})
		return
	}

	tenantID, _ := c.Get("tenant_id")
	tenant, _ := tenantID.(string)
	if tenant == "" {
		tenant = "public"
	}

	entries, err := h.DB.GetQueryHistory(c.Request.Context(), tenant, 1000)
	if err != nil {
		log.Printf("ADMIN: routing-stats query failed tenant=%s err=%v", tenant, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to retrieve query history"})
		return
	}

	byProvider := make(map[string]int)
	for _, e := range entries {
		if e.ProviderUsed != "" {
			byProvider[e.ProviderUsed]++
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"total":       len(entries),
		"by_provider": byProvider,
	})
}
