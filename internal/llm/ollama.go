package llm

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// ---------------------------------------------------------------------------
// Ollama API request / response types (preserved from POC)
// ---------------------------------------------------------------------------

type embedRequest struct {
	Model  string `json:"model"`
	Prompt string `json:"prompt"`
}

type embedResponse struct {
	Embedding []float32 `json:"embedding"`
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
// OllamaProvider implements LLMProvider
// ---------------------------------------------------------------------------

// OllamaProvider wraps the Ollama HTTP API as an LLMProvider.
type OllamaProvider struct {
	baseURL    string
	model      string
	embedModel string
	httpClient *http.Client
}

// OllamaConfig holds configuration for the Ollama provider.
type OllamaConfig struct {
	BaseURL    string
	Model      string
	EmbedModel string
}

// NewOllamaProvider creates an OllamaProvider with configured timeouts.
func NewOllamaProvider(cfg OllamaConfig) *OllamaProvider {
	return &OllamaProvider{
		baseURL:    cfg.BaseURL,
		model:      cfg.Model,
		embedModel: cfg.EmbedModel,
		httpClient: &http.Client{
			Timeout: 120 * time.Second,
		},
	}
}

func (p *OllamaProvider) Name() string { return "ollama" }

// HealthCheck verifies Ollama is reachable.
func (p *OllamaProvider) HealthCheck(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, p.baseURL+"/api/tags", nil)
	if err != nil {
		return fmt.Errorf("ollama health check: %w", err)
	}
	resp, err := p.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("ollama health check: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("ollama health check: status %d", resp.StatusCode)
	}
	return nil
}

// Complete sends a chat completion and returns the full response.
// Preserves the core HTTP call logic from POC ollama.go Chat().
func (p *OllamaProvider) Complete(ctx context.Context, req CompletionRequest) (CompletionResponse, error) {
	start := time.Now()

	model := req.Model
	if model == "" {
		model = p.model
	}

	messages := req.Messages
	if len(messages) == 0 {
		if req.SystemPrompt != "" {
			messages = append(messages, ChatMessage{Role: "system", Content: req.SystemPrompt})
		}
		messages = append(messages, ChatMessage{Role: "user", Content: req.Prompt})
	}

	body, err := json.Marshal(chatRequest{
		Model:    model,
		Messages: messages,
		Stream:   false,
	})
	if err != nil {
		return CompletionResponse{}, fmt.Errorf("marshal chat request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, p.baseURL+"/api/chat", bytes.NewBuffer(body))
	if err != nil {
		return CompletionResponse{}, fmt.Errorf("create chat request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := p.httpClient.Do(httpReq)
	if err != nil {
		return CompletionResponse{}, fmt.Errorf("ollama /api/chat: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return CompletionResponse{}, fmt.Errorf("ollama /api/chat returned %d", resp.StatusCode)
	}

	var result chatResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return CompletionResponse{}, fmt.Errorf("decode chat response: %w", err)
	}

	return CompletionResponse{
		Content:      result.Message.Content,
		ModelUsed:    model,
		Latency:      time.Since(start),
		TokensIn:     len(req.Prompt) / 4, // approximation
		TokensOut:    len(result.Message.Content) / 4,
		FinishReason: "stop",
	}, nil
}

// Stream sends a chat completion and streams tokens back via channel.
func (p *OllamaProvider) Stream(ctx context.Context, req CompletionRequest) (<-chan StreamChunk, error) {
	model := req.Model
	if model == "" {
		model = p.model
	}

	messages := req.Messages
	if len(messages) == 0 {
		if req.SystemPrompt != "" {
			messages = append(messages, ChatMessage{Role: "system", Content: req.SystemPrompt})
		}
		messages = append(messages, ChatMessage{Role: "user", Content: req.Prompt})
	}

	body, err := json.Marshal(chatRequest{
		Model:    model,
		Messages: messages,
		Stream:   true,
	})
	if err != nil {
		return nil, fmt.Errorf("marshal chat request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, p.baseURL+"/api/chat", bytes.NewBuffer(body))
	if err != nil {
		return nil, fmt.Errorf("create chat request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := p.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("ollama /api/chat stream: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		resp.Body.Close()
		return nil, fmt.Errorf("ollama /api/chat stream returned %d", resp.StatusCode)
	}

	ch := make(chan StreamChunk, 64)
	go func() {
		defer close(ch)
		defer resp.Body.Close()

		scanner := bufio.NewScanner(resp.Body)
		for scanner.Scan() {
			var cr chatResponse
			if err := json.Unmarshal(scanner.Bytes(), &cr); err != nil {
				ch <- StreamChunk{Error: fmt.Errorf("decode stream chunk: %w", err)}
				return
			}
			ch <- StreamChunk{
				Text: cr.Message.Content,
				Done: cr.Done,
			}
			if cr.Done {
				return
			}
		}
		if err := scanner.Err(); err != nil {
			ch <- StreamChunk{Error: fmt.Errorf("stream read: %w", err)}
		}
	}()

	return ch, nil
}

// Embed generates embeddings for a batch of texts.
// Preserves the core HTTP call logic from POC ollama.go Embed().
func (p *OllamaProvider) Embed(ctx context.Context, texts []string) ([][]float32, error) {
	model := p.embedModel
	if model == "" {
		model = "nomic-embed-text"
	}

	results := make([][]float32, 0, len(texts))
	for _, text := range texts {
		body, err := json.Marshal(embedRequest{Model: model, Prompt: text})
		if err != nil {
			return nil, fmt.Errorf("marshal embed request: %w", err)
		}

		httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, p.baseURL+"/api/embeddings", bytes.NewBuffer(body))
		if err != nil {
			return nil, fmt.Errorf("create embed request: %w", err)
		}
		httpReq.Header.Set("Content-Type", "application/json")

		resp, err := p.httpClient.Do(httpReq)
		if err != nil {
			return nil, fmt.Errorf("ollama /api/embeddings: %w", err)
		}

		if resp.StatusCode != http.StatusOK {
			resp.Body.Close()
			return nil, fmt.Errorf("ollama /api/embeddings returned %d", resp.StatusCode)
		}

		var result embedResponse
		if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
			resp.Body.Close()
			return nil, fmt.Errorf("decode embed response: %w", err)
		}
		resp.Body.Close()

		if len(result.Embedding) == 0 {
			return nil, fmt.Errorf("ollama returned empty embedding for text at index %d", len(results))
		}
		results = append(results, result.Embedding)
	}
	return results, nil
}
