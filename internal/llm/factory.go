package llm

import (
	"fmt"

	"testqdrant/internal/config"
)

// NewProvider creates an LLMProvider based on configuration.
func NewProvider(cfg config.Config) (LLMProvider, error) {
	switch cfg.LLMProvider {
	case "ollama":
		return NewOllamaProvider(OllamaConfig{
			BaseURL:    cfg.OllamaURL,
			Model:      cfg.ChatModel,
			EmbedModel: cfg.EmbedModel,
		}), nil

	case "claude":
		return NewClaudeProvider(ClaudeConfig{
			APIKey: cfg.ClaudeAPIKey,
			Model:  cfg.ChatModel,
		}), nil

	case "gemini":
		return NewGeminiProvider(GeminiConfig{
			APIKey: cfg.GeminiAPIKey,
			Model:  cfg.ChatModel,
		}), nil

	default:
		return nil, fmt.Errorf("unknown LLM provider: %q", cfg.LLMProvider)
	}
}
