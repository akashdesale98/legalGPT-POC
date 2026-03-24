package config

import (
	"fmt"
	"log"
	"os"
	"strconv"
)

// Config holds all runtime settings.
// Loaded from environment variables with sensible defaults for non-secret values.
type Config struct {
	QdrantHost        string
	QdrantPort        int
	OllamaURL         string
	EmbedModel        string
	ChatModel         string
	VectorSize        int
	APIPort           int
	DefaultTopK       int
	DefaultCollection string
	LLMProvider       string // "ollama" | "claude" | "gemini"
	ClaudeAPIKey      string
	GeminiAPIKey      string
	QdrantAPIKey      string
	DatabaseURL       string
	RedisURL          string
	DevToken          string
	DevMode           bool
	LogLevel          string
	OTELEndpoint      string
}

// LoadConfig reads configuration from environment variables.
// Non-secret values have sensible defaults. Secret values (API keys) are optional
// but logged as warnings when missing.
func LoadConfig() Config {
	cfg := Config{
		QdrantHost:        envOrDefault("QDRANT_HOST", "localhost"),
		QdrantPort:        envIntOrDefault("QDRANT_PORT", 6334),
		OllamaURL:         envOrDefault("OLLAMA_URL", "http://localhost:11434"),
		EmbedModel:        envOrDefault("EMBED_MODEL", "nomic-embed-text"),
		ChatModel:         envOrDefault("CHAT_MODEL", "llama3.2"),
		VectorSize:        envIntOrDefault("VECTOR_SIZE", 768),
		APIPort:           envIntOrDefault("API_PORT", 8080),
		DefaultTopK:       envIntOrDefault("DEFAULT_TOP_K", 5),
		DefaultCollection: envOrDefault("DEFAULT_COLLECTION", "constitution"),
		LLMProvider:       envOrDefault("LLM_PROVIDER", "ollama"),
		ClaudeAPIKey:      os.Getenv("CLAUDE_API_KEY"),
		GeminiAPIKey:      os.Getenv("GEMINI_API_KEY"),
		QdrantAPIKey:      os.Getenv("QDRANT_API_KEY"),
		DatabaseURL:       os.Getenv("DATABASE_URL"),
		RedisURL:          envOrDefault("REDIS_URL", "localhost:6379"),
		DevToken:          os.Getenv("DEV_TOKEN"),
		DevMode:           os.Getenv("DEV_MODE") == "true",
		LogLevel:          envOrDefault("LOG_LEVEL", "info"),
		OTELEndpoint:      os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT"),
	}
	return cfg
}

// Validate checks that required configuration is present based on the active provider.
// Returns an error if required fields are missing.
func (c *Config) Validate() error {
	switch c.LLMProvider {
	case "ollama":
		if c.OllamaURL == "" {
			return fmt.Errorf("OLLAMA_URL is required when LLM_PROVIDER=ollama")
		}
	case "claude":
		if c.ClaudeAPIKey == "" {
			return fmt.Errorf("CLAUDE_API_KEY is required when LLM_PROVIDER=claude")
		}
	case "gemini":
		if c.GeminiAPIKey == "" {
			return fmt.Errorf("GEMINI_API_KEY is required when LLM_PROVIDER=gemini")
		}
	default:
		return fmt.Errorf("unknown LLM_PROVIDER: %q (valid: ollama, claude, gemini)", c.LLMProvider)
	}

	if c.DevToken != "" {
		log.Println("WARN: DEV_TOKEN is set — disable before production deployment")
	}

	return nil
}

func envOrDefault(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func envIntOrDefault(key string, fallback int) int {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		log.Printf("WARN: invalid integer for %s=%q, using default %d", key, v, fallback)
		return fallback
	}
	return n
}
