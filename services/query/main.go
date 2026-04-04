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
	"testqdrant/internal/db"
	"testqdrant/internal/llm"
	"testqdrant/internal/rag"
	"testqdrant/internal/store"
	"testqdrant/internal/worker"
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

	// PostgreSQL (optional in dev mode)
	var pgRepo *store.PostgresRepo
	if cfg.PostgresDSN != "" {
		repo, pgErr := store.NewPostgresRepo(ctx, cfg.PostgresDSN)
		if pgErr != nil {
			if cfg.DevMode {
				log.Printf("WARN: postgres unavailable (dev mode): %v", pgErr)
			} else {
				log.Fatalf("postgres: %v", pgErr)
			}
		} else {
			pgRepo = repo
			defer pgRepo.Close()

			if err := db.RunMigrations(ctx, repo.Pool()); err != nil {
				log.Fatalf("migrations: %v", err)
			}

			// Load IPC→BNS mapping
			mappings, loadErr := db.LoadIPCMappings("scripts/ipc_bns_map.json")
			if loadErr != nil {
				log.Printf("WARN: ipc_bns_map not loaded: %v", loadErr)
			} else if upsertErr := pgRepo.BulkUpsertIPCMapping(ctx, mappings); upsertErr != nil {
				log.Printf("WARN: ipc_bns_map upsert: %v", upsertErr)
			}
		}
	} else if !cfg.DevMode {
		log.Fatal("POSTGRES_DSN is required when DEV_MODE=false")
	} else {
		log.Println("WARN: POSTGRES_DSN not set — running without persistent storage (dev mode)")
	}

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

	// Worker pool (backpressure)
	workerPool := worker.NewPool(cfg.WorkerPoolSize)

	// Handlers
	healthH := &handler.HealthHandler{Store: qdrantStore, Provider: provider}
	if pgRepo != nil {
		healthH.DB = pgRepo
	}
	searchH := &handler.SearchHandler{Provider: provider, Retriever: retriever, DefaultCollection: cfg.DefaultCollection}
	chatH := &handler.ChatHandler{
		Provider:          provider,
		Retriever:         retriever,
		Guardrail:         guardrail,
		Assembler:         assembler,
		DefaultCollection: cfg.DefaultCollection,
		Pool:              workerPool,
	}
	if pgRepo != nil {
		healthH.Pool = workerPool
	}
	adminH := &handler.AdminHandler{}
	if pgRepo != nil {
		adminH.DB = pgRepo
	}

	// Middleware
	rateLimiter := middleware.NewRateLimiter(middleware.DefaultTierLimits())
	authCfg := middleware.LoadAuthConfig(cfg.DevToken, cfg.DevMode, cfg.JWTPublicKeyPath)

	// Auth token handler (needs private key for issuance)
	var authH *handler.AuthHandler
	if cfg.JWTPrivateKeyPath != "" && pgRepo != nil {
		authH = handler.NewAuthHandler(cfg.JWTPrivateKeyPath, pgRepo)
	}

	// Gin router
	router := gin.Default()

	// Health endpoints (no auth required)
	router.GET("/api/v1/health/live", healthH.LiveHandler)
	router.GET("/api/v1/health/ready", healthH.ReadyHandler)

	// Auth endpoint (no auth middleware — issues tokens)
	if authH != nil {
		router.POST("/api/v1/auth/token", authH.IssueToken)
	}

	// API endpoints (auth + rate limiting)
	api := router.Group("/api/v1")
	api.Use(middleware.Auth(authCfg))
	api.Use(rateLimiter.Middleware())
	{
		api.POST("/search", searchH.Handle)
		api.POST("/chat", chatH.Handle)
	}

	// Admin endpoints (auth + role check)
	admin := router.Group("/api/v1/admin")
	admin.Use(middleware.Auth(authCfg))
	admin.Use(middleware.RequireRole("admin"))
	admin.GET("/routing-stats", adminH.RoutingStats)

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
