package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
)

// ---------------------------------------------------------------------------
// Request / Response types
// ---------------------------------------------------------------------------

type embedRequest struct {
	Model  string `json:"model"`
	Prompt string `json:"prompt"`
}

type embedResponse struct {
	Embedding []float32 `json:"embedding"`
}

// ChatMessage is a single turn in a conversation.
type ChatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type chatRequest struct {
	Model    string        `json:"model"`
	Messages []ChatMessage `json:"messages"`
	Stream   bool          `json:"stream"`
}

type chatResponse struct {
	Message ChatMessage `json:"message"`
	Done    bool        `json:"done"`
}

// ---------------------------------------------------------------------------
// OllamaClient
// ---------------------------------------------------------------------------

// OllamaClient wraps the Ollama HTTP API.
type OllamaClient struct {
	BaseURL string
	http    *http.Client
}

func NewOllamaClient(baseURL string) *OllamaClient {
	return &OllamaClient{
		BaseURL: baseURL,
		http:    &http.Client{},
	}
}

// Embed returns an embedding vector for the given text.
func (c *OllamaClient) Embed(model, text string) ([]float32, error) {
	body, _ := json.Marshal(embedRequest{Model: model, Prompt: text})

	resp, err := c.http.Post(c.BaseURL+"/api/embeddings", "application/json", bytes.NewBuffer(body))
	if err != nil {
		return nil, fmt.Errorf("ollama /api/embeddings: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("ollama /api/embeddings returned %d", resp.StatusCode)
	}

	var result embedResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode embed response: %w", err)
	}
	if len(result.Embedding) == 0 {
		return nil, fmt.Errorf("ollama returned empty embedding")
	}
	return result.Embedding, nil
}

// Chat sends a multi-turn chat request and returns the assistant reply.
func (c *OllamaClient) Chat(model string, messages []ChatMessage) (string, error) {
	body, _ := json.Marshal(chatRequest{
		Model:    model,
		Messages: messages,
		Stream:   false,
	})

	resp, err := c.http.Post(c.BaseURL+"/api/chat", "application/json", bytes.NewBuffer(body))
	if err != nil {
		return "", fmt.Errorf("ollama /api/chat: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("ollama /api/chat returned %d", resp.StatusCode)
	}

	var result chatResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("decode chat response: %w", err)
	}
	return result.Message.Content, nil
}
