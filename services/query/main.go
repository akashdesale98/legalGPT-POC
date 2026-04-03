package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/qdrant/go-client/qdrant"

	"testqdrant/internal/config"
	"testqdrant/internal/llm"
	"testqdrant/internal/rag"
	"testqdrant/internal/store"
	"testqdrant/services/query/handler"
	"testqdrant/services/query/middleware"
)

func main() {
	cfg := config.LoadConfig()
	if err := cfg.Validate(); err != nil {
		log.Fatalf("config validation: %v", err)
	}

	// Graceful shutdown context
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	// LLM provider via factory
	provider, err := llm.NewProvider(cfg)
	if err != nil {
		log.Fatalf("llm provider: %v", err)
	}

	// Qdrant client
	qdrantClient, err := qdrant.NewClient(&qdrant.Config{
		Host: cfg.QdrantHost,
		Port: cfg.QdrantPort,
	})
	if err != nil {
		log.Fatalf("qdrant client: %v", err)
	}
	qdrantStore := store.NewQdrantStore(qdrantClient)

	// RAG components
	retriever := rag.NewHybridRetriever(qdrantStore)
	guardrail := rag.NewGuardrail()
	assembler := rag.NewContextAssembler(6000)

	// Handlers
	healthH := &handler.HealthHandler{Store: qdrantStore, Provider: provider}
	searchH := &handler.SearchHandler{Provider: provider, Retriever: retriever, DefaultCollection: cfg.DefaultCollection}
	chatH := &handler.ChatHandler{
		Provider:          provider,
		Retriever:         retriever,
		Guardrail:         guardrail,
		Assembler:         assembler,
		DefaultCollection: cfg.DefaultCollection,
	}

	// Middleware
	rateLimiter := middleware.NewRateLimiter(middleware.DefaultTierLimits())
	authCfg := middleware.AuthConfig{DevToken: cfg.DevToken, DevMode: cfg.DevMode}

	// Gin router
	router := gin.Default()

	// Health endpoints (no auth required)
	router.GET("/api/v1/health/live", healthH.LiveHandler)
	router.GET("/api/v1/health/ready", healthH.ReadyHandler)

	// API endpoints (auth + rate limiting)
	api := router.Group("/api/v1")
	api.Use(middleware.Auth(authCfg))
	api.Use(rateLimiter.Middleware())
	{
		api.POST("/search", searchH.Handle)
		api.POST("/chat", chatH.Handle)
	}

	// HTTP server with graceful shutdown
	addr := fmt.Sprintf(":%d", cfg.APIPort)
	srv := &http.Server{
		Addr:    addr,
		Handler: router,
	}

	// Start server in goroutine
	go func() {
		log.Printf("query-service starting on %s (provider=%s)", addr, provider.Name())
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("server error: %v", err)
		}
	}()

	// Wait for shutdown signal
	<-ctx.Done()
	log.Println("shutting down query-service...")

	// Graceful shutdown with 30s timeout
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Fatalf("server shutdown error: %v", err)
	}
	log.Println("query-service stopped")
}
