package llm_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"testqdrant/internal/llm"
)

// mockOllamaServer creates a test server that mimics Ollama API responses.
func mockOllamaServer(t *testing.T) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/tags":
			json.NewEncoder(w).Encode(map[string]interface{}{
				"models": []map[string]interface{}{
					{"name": "llama3.2:latest"},
				},
			})
		case "/api/embeddings":
			json.NewEncoder(w).Encode(map[string]interface{}{
				"embedding": []float32{0.1, 0.2, 0.3, 0.4, 0.5},
			})
		case "/api/chat":
			json.NewEncoder(w).Encode(map[string]interface{}{
				"message": map[string]interface{}{
					"role":    "assistant",
					"content": "Article 21 guarantees the right to life and personal liberty.",
				},
				"done": true,
			})
		default:
			http.NotFound(w, r)
		}
	}))
}

func TestOllamaProvider_Complete(t *testing.T) {
	srv := mockOllamaServer(t)
	defer srv.Close()

	provider := llm.NewOllamaProvider(llm.OllamaConfig{
		BaseURL:    srv.URL,
		Model:      "llama3.2",
		EmbedModel: "nomic-embed-text",
	})

	ctx := context.Background()
	resp, err := provider.Complete(ctx, llm.CompletionRequest{
		Prompt:       "What is Article 21?",
		SystemPrompt: "You are a legal expert.",
	})
	if err != nil {
		t.Fatalf("Complete() error: %v", err)
	}
	if resp.Content == "" {
		t.Error("Complete() returned empty content")
	}
	if resp.ModelUsed != "llama3.2" {
		t.Errorf("ModelUsed = %q, want %q", resp.ModelUsed, "llama3.2")
	}
}

func TestOllamaProvider_Embed(t *testing.T) {
	srv := mockOllamaServer(t)
	defer srv.Close()

	provider := llm.NewOllamaProvider(llm.OllamaConfig{
		BaseURL:    srv.URL,
		Model:      "llama3.2",
		EmbedModel: "nomic-embed-text",
	})

	ctx := context.Background()
	embeddings, err := provider.Embed(ctx, []string{"test query"})
	if err != nil {
		t.Fatalf("Embed() error: %v", err)
	}
	if len(embeddings) != 1 {
		t.Fatalf("Embed() returned %d embeddings, want 1", len(embeddings))
	}
	if len(embeddings[0]) != 5 {
		t.Errorf("Embedding dimension = %d, want 5", len(embeddings[0]))
	}
}

func TestOllamaProvider_ContextCancellation(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(5 * time.Second) // Simulate slow response
	}))
	defer srv.Close()

	provider := llm.NewOllamaProvider(llm.OllamaConfig{
		BaseURL: srv.URL,
		Model:   "llama3.2",
	})

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	_, err := provider.Complete(ctx, llm.CompletionRequest{Prompt: "test"})
	if err == nil {
		t.Error("Complete() should fail on cancelled context")
	}
}

func TestOllamaProvider_HealthCheck(t *testing.T) {
	srv := mockOllamaServer(t)
	defer srv.Close()

	provider := llm.NewOllamaProvider(llm.OllamaConfig{
		BaseURL: srv.URL,
		Model:   "llama3.2",
	})

	err := provider.HealthCheck(context.Background())
	if err != nil {
		t.Errorf("HealthCheck() error: %v", err)
	}
}

func TestOllamaProvider_Name(t *testing.T) {
	provider := llm.NewOllamaProvider(llm.OllamaConfig{})
	if provider.Name() != "ollama" {
		t.Errorf("Name() = %q, want %q", provider.Name(), "ollama")
	}
}

func TestClaudeProvider_Name(t *testing.T) {
	provider := llm.NewClaudeProvider(llm.ClaudeConfig{APIKey: "test"})
	if provider.Name() != "claude" {
		t.Errorf("Name() = %q, want %q", provider.Name(), "claude")
	}
}

func TestGeminiProvider_Name(t *testing.T) {
	provider := llm.NewGeminiProvider(llm.GeminiConfig{APIKey: "test"})
	if provider.Name() != "gemini" {
		t.Errorf("Name() = %q, want %q", provider.Name(), "gemini")
	}
}

func TestClaudeProvider_EmbedReturnsError(t *testing.T) {
	provider := llm.NewClaudeProvider(llm.ClaudeConfig{APIKey: "test"})
	_, err := provider.Embed(context.Background(), []string{"test"})
	if err == nil {
		t.Error("Claude Embed() should return error (not supported)")
	}
}

// mockClaudeServer creates a test server that mimics Claude Messages API.
func mockClaudeServer(t *testing.T) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v1/messages" {
			json.NewEncoder(w).Encode(map[string]interface{}{
				"content": []map[string]interface{}{
					{"type": "text", "text": "Legal answer from Claude."},
				},
				"model":       "claude-sonnet-4-5-20250514",
				"stop_reason":  "end_turn",
				"usage": map[string]interface{}{
					"input_tokens":  100,
					"output_tokens": 50,
				},
			})
		} else {
			http.NotFound(w, r)
		}
	}))
}

func TestClaudeProvider_Complete(t *testing.T) {
	srv := mockClaudeServer(t)
	defer srv.Close()

	provider := llm.NewClaudeProvider(llm.ClaudeConfig{
		APIKey:  "test-key",
		BaseURL: srv.URL,
	})

	resp, err := provider.Complete(context.Background(), llm.CompletionRequest{
		Prompt:       "What is BNS Section 103?",
		SystemPrompt: "You are a legal expert.",
	})
	if err != nil {
		t.Fatalf("Complete() error: %v", err)
	}
	if resp.Content == "" {
		t.Error("Complete() returned empty content")
	}
	if resp.TokensIn != 100 {
		t.Errorf("TokensIn = %d, want 100", resp.TokensIn)
	}
}

// mockGeminiServer creates a test server for Gemini.
func mockGeminiServer(t *testing.T) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "GET" {
			// Model info check
			json.NewEncoder(w).Encode(map[string]interface{}{"name": "models/gemini-2.5-flash"})
		} else if r.Method == "POST" {
			json.NewEncoder(w).Encode(map[string]interface{}{
				"candidates": []map[string]interface{}{
					{
						"content": map[string]interface{}{
							"parts": []map[string]interface{}{
								{"text": "Legal answer from Gemini."},
							},
						},
						"finishReason": "STOP",
					},
				},
				"usageMetadata": map[string]interface{}{
					"promptTokenCount":     80,
					"candidatesTokenCount": 40,
					"totalTokenCount":      120,
				},
			})
		}
	}))
}

func TestGeminiProvider_Complete(t *testing.T) {
	srv := mockGeminiServer(t)
	defer srv.Close()

	provider := llm.NewGeminiProvider(llm.GeminiConfig{
		APIKey:  "test-key",
		BaseURL: srv.URL,
	})

	resp, err := provider.Complete(context.Background(), llm.CompletionRequest{
		Prompt: "What is Article 21?",
	})
	if err != nil {
		t.Fatalf("Complete() error: %v", err)
	}
	if resp.Content == "" {
		t.Error("Complete() returned empty content")
	}
}
