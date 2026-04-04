package handler

import (
	"context"
	"net/http"

	"github.com/gin-gonic/gin"

	"testqdrant/internal/llm"
	"testqdrant/internal/store"
)

// DBPinger is satisfied by *store.PostgresRepo.
type DBPinger interface {
	Ping(ctx context.Context) error
}

// WorkerPoolStats is the subset of worker.Pool used by HealthHandler.
// Defined as an interface to avoid import cycles.
type WorkerPoolStats interface {
	ActiveWorkers() int
	Capacity() int
}

// HealthHandler handles liveness and readiness probes.
type HealthHandler struct {
	Store    store.VectorStore
	Provider llm.LLMProvider
	DB       DBPinger    // optional; nil when Postgres is not configured
	Pool     WorkerPoolStats // optional; nil when worker pool is not configured
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

	resp := gin.H{"status": "ready", "qdrant": "ok", "llm": "ok"}

	if h.DB != nil {
		if err := h.DB.Ping(ctx); err != nil {
			resp["postgres"] = "unavailable"
		} else {
			resp["postgres"] = "ok"
		}
	} else {
		resp["postgres"] = "unavailable"
	}

	c.JSON(http.StatusOK, resp)
}
