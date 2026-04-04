package llm

import (
	"fmt"
	"log"

	"testqdrant/internal/config"
)

// NewProvider creates an LLMProvider based on configuration.
// Ollama, Claude, and Gemini providers are each wrapped with a BreakerProvider.
// The router is not wrapped — its inner providers are already wrapped.
func NewProvider(cfg config.Config) (LLMProvider, error) {
	switch cfg.LLMProvider {
	case "ollama":
		p := NewOllamaProvider(OllamaConfig{
			BaseURL:    cfg.OllamaURL,
			Model:      cfg.ChatModel,
			EmbedModel: cfg.EmbedModel,
		})
		return NewBreakerProvider(p), nil

	case "claude":
		p := NewClaudeProvider(ClaudeConfig{
			APIKey: cfg.ClaudeAPIKey,
			Model:  cfg.ChatModel,
		})
		return NewBreakerProvider(p), nil

	case "gemini":
		p := NewGeminiProvider(GeminiConfig{
			APIKey: cfg.GeminiAPIKey,
			Model:  cfg.ChatModel,
		})
		return NewBreakerProvider(p), nil

	case "router":
		// Needs at least one of Gemini or Claude
		var simple, complex LLMProvider
		if cfg.GeminiAPIKey != "" {
			simple = NewGeminiProvider(GeminiConfig{APIKey: cfg.GeminiAPIKey, Model: cfg.ChatModel})
		}
		if cfg.ClaudeAPIKey != "" {
			complex = NewClaudeProvider(ClaudeConfig{APIKey: cfg.ClaudeAPIKey, Model: cfg.ChatModel})
		}
		if simple == nil && complex == nil {
			return nil, fmt.Errorf("router requires GEMINI_API_KEY or CLAUDE_API_KEY")
		}
		// Fall back gracefully if one key is missing
		if simple == nil {
			log.Printf("WARN: GEMINI_API_KEY missing — router using Claude for all queries")
			simple = complex
		}
		if complex == nil {
			log.Printf("WARN: CLAUDE_API_KEY missing — router using Gemini for all queries")
			complex = simple
		}
		return NewRouter(simple, complex, cfg.LLMRouteThreshold), nil

	default:
		return nil, fmt.Errorf("unknown LLM provider: %q", cfg.LLMProvider)
	}
}
