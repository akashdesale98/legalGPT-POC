package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"testqdrant/internal/llm"
	"testqdrant/internal/store"
)

// HealthHandler handles liveness and readiness probes.
type HealthHandler struct {
	Store    store.VectorStore
	Provider llm.LLMProvider
}

// LiveHandler returns 200 if the process is running.
// GET /api/v1/health/live
func (h *HealthHandler) LiveHandler(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"status": "alive"})
}

// ReadyHandler returns 200 if Qdrant and LLM provider are reachable.
// GET /api/v1/health/ready
func (h *HealthHandler) ReadyHandler(c *gin.Context) {
	ctx := c.Request.Context()

	if err := h.Store.HealthCheck(ctx); err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"status": "not_ready",
			"error":  "qdrant: " + err.Error(),
		})
		return
	}

	if err := h.Provider.HealthCheck(ctx); err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"status": "not_ready",
			"error":  "llm_provider: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "ready"})
}
