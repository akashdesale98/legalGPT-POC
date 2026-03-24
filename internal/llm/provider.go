package llm

import (
	"context"
	"time"
)

// ChatMessage is a single turn in a conversation.
type ChatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// CompletionRequest holds parameters for an LLM completion.
type CompletionRequest struct {
	Prompt        string
	SystemPrompt  string
	Messages      []ChatMessage
	Model         string
	MaxTokens     int
	Temperature   float32
	StopSequences []string
}

// CompletionResponse holds the result of a completion.
type CompletionResponse struct {
	Content      string
	TokensIn     int
	TokensOut    int
	ModelUsed    string
	Latency      time.Duration
	FinishReason string
}

// StreamChunk is a single chunk from a streaming response.
type StreamChunk struct {
	Text  string
	Done  bool
	Error error
}

// LLMProvider is the port — all LLM backends implement this interface.
type LLMProvider interface {
	// Complete sends a prompt and waits for the complete response.
	Complete(ctx context.Context, req CompletionRequest) (CompletionResponse, error)

	// Stream sends a prompt and returns a channel of chunks for SSE delivery.
	Stream(ctx context.Context, req CompletionRequest) (<-chan StreamChunk, error)

	// Embed generates vector embeddings for a batch of texts.
	Embed(ctx context.Context, texts []string) ([][]float32, error)

	// Name returns the provider identifier (e.g., "ollama", "claude", "gemini").
	Name() string

	// HealthCheck verifies the provider is reachable.
	HealthCheck(ctx context.Context) error
}
