package llm

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// ---------------------------------------------------------------------------
// Claude (Anthropic) API types
// ---------------------------------------------------------------------------

type claudeMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type claudeRequest struct {
	Model     string          `json:"model"`
	MaxTokens int             `json:"max_tokens"`
	System    string          `json:"system,omitempty"`
	Messages  []claudeMessage `json:"messages"`
	Stream    bool            `json:"stream,omitempty"`
}

type claudeUsage struct {
	InputTokens  int `json:"input_tokens"`
	OutputTokens int `json:"output_tokens"`
}

type claudeContent struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

type claudeResponse struct {
	Content    []claudeContent `json:"content"`
	Model      string          `json:"model"`
	StopReason string          `json:"stop_reason"`
	Usage      claudeUsage     `json:"usage"`
}

type claudeStreamEvent struct {
	Type  string `json:"type"`
	Delta struct {
		Type string `json:"type"`
		Text string `json:"text"`
	} `json:"delta"`
}

type claudeErrorResponse struct {
	Error struct {
		Type    string `json:"type"`
		Message string `json:"message"`
	} `json:"error"`
}

// ---------------------------------------------------------------------------
// ClaudeProvider implements LLMProvider
// ---------------------------------------------------------------------------

type ClaudeProvider struct {
	apiKey     string
	model      string
	baseURL    string
	httpClient *http.Client
}

type ClaudeConfig struct {
	APIKey  string
	Model   string
	BaseURL string
}

func NewClaudeProvider(cfg ClaudeConfig) *ClaudeProvider {
	baseURL := cfg.BaseURL
	if baseURL == "" {
		baseURL = "https://api.anthropic.com"
	}
	model := cfg.Model
	if model == "" {
		model = "claude-sonnet-4-5-20250514"
	}
	return &ClaudeProvider{
		apiKey:  cfg.APIKey,
		model:   model,
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: 120 * time.Second,
		},
	}
}

func (p *ClaudeProvider) Name() string { return "claude" }

func (p *ClaudeProvider) HealthCheck(ctx context.Context) error {
	// Send a minimal request to verify API key works
	req := CompletionRequest{
		Prompt:    "ping",
		MaxTokens: 1,
	}
	_, err := p.Complete(ctx, req)
	return err
}

func (p *ClaudeProvider) Complete(ctx context.Context, req CompletionRequest) (CompletionResponse, error) {
	start := time.Now()

	model := req.Model
	if model == "" {
		model = p.model
	}
	maxTokens := req.MaxTokens
	if maxTokens == 0 {
		maxTokens = 4096
	}

	messages := buildClaudeMessages(req)

	body, err := json.Marshal(claudeRequest{
		Model:     model,
		MaxTokens: maxTokens,
		System:    req.SystemPrompt,
		Messages:  messages,
		Stream:    false,
	})
	if err != nil {
		return CompletionResponse{}, fmt.Errorf("marshal claude request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, p.baseURL+"/v1/messages", bytes.NewBuffer(body))
	if err != nil {
		return CompletionResponse{}, fmt.Errorf("create claude request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("x-api-key", p.apiKey)
	httpReq.Header.Set("anthropic-version", "2023-06-01")

	resp, err := p.httpClient.Do(httpReq)
	if err != nil {
		return CompletionResponse{}, fmt.Errorf("claude /v1/messages: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return CompletionResponse{}, fmt.Errorf("claude /v1/messages returned %d: %s", resp.StatusCode, string(respBody))
	}

	var result claudeResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return CompletionResponse{}, fmt.Errorf("decode claude response: %w", err)
	}

	var contentBuilder strings.Builder
	for _, c := range result.Content {
		if c.Type == "text" {
			contentBuilder.WriteString(c.Text)
		}
	}
	content := contentBuilder.String()

	return CompletionResponse{
		Content:      content,
		TokensIn:     result.Usage.InputTokens,
		TokensOut:    result.Usage.OutputTokens,
		ModelUsed:    result.Model,
		Latency:      time.Since(start),
		FinishReason: result.StopReason,
	}, nil
}

func (p *ClaudeProvider) Stream(ctx context.Context, req CompletionRequest) (<-chan StreamChunk, error) {
	model := req.Model
	if model == "" {
		model = p.model
	}
	maxTokens := req.MaxTokens
	if maxTokens == 0 {
		maxTokens = 4096
	}

	messages := buildClaudeMessages(req)

	body, err := json.Marshal(claudeRequest{
		Model:     model,
		MaxTokens: maxTokens,
		System:    req.SystemPrompt,
		Messages:  messages,
		Stream:    true,
	})
	if err != nil {
		return nil, fmt.Errorf("marshal claude stream request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, p.baseURL+"/v1/messages", bytes.NewBuffer(body))
	if err != nil {
		return nil, fmt.Errorf("create claude stream request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("x-api-key", p.apiKey)
	httpReq.Header.Set("anthropic-version", "2023-06-01")

	resp, err := p.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("claude stream: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		return nil, fmt.Errorf("claude stream returned %d: %s", resp.StatusCode, string(respBody))
	}

	ch := make(chan StreamChunk, 64)
	go func() {
		defer close(ch)
		defer resp.Body.Close()

		// Parse Claude SSE format line-by-line using bufio.Scanner.
		// Each event has "event: <type>\ndata: <json>\n\n" structure.
		scanner := bufio.NewScanner(resp.Body)
		for scanner.Scan() {
			line := bytes.TrimSpace(scanner.Bytes())

			if !bytes.HasPrefix(line, []byte("data: ")) {
				continue
			}
			data := bytes.TrimPrefix(line, []byte("data: "))
			if string(data) == "[DONE]" {
				ch <- StreamChunk{Done: true}
				return
			}

			var event claudeStreamEvent
			if jsonErr := json.Unmarshal(data, &event); jsonErr != nil {
				continue
			}
			if event.Type == "content_block_delta" && event.Delta.Text != "" {
				ch <- StreamChunk{Text: event.Delta.Text}
			}
			if event.Type == "message_stop" {
				ch <- StreamChunk{Done: true}
				return
			}
		}
		if err := scanner.Err(); err != nil {
			ch <- StreamChunk{Error: fmt.Errorf("claude stream read: %w", err)}
		}
	}()

	return ch, nil
}

// Embed is not natively supported by Claude — returns an error directing callers
// to use a dedicated embedding provider (Cohere, Ollama, etc.).
func (p *ClaudeProvider) Embed(_ context.Context, _ []string) ([][]float32, error) {
	return nil, fmt.Errorf("claude provider does not support embeddings — use a dedicated embedding provider (cohere, ollama)")
}

func buildClaudeMessages(req CompletionRequest) []claudeMessage {
	if len(req.Messages) > 0 {
		msgs := make([]claudeMessage, 0, len(req.Messages))
		for _, m := range req.Messages {
			if m.Role == "system" {
				continue // system goes in the system field
			}
			msgs = append(msgs, claudeMessage{Role: m.Role, Content: m.Content})
		}
		if len(msgs) == 0 {
			msgs = append(msgs, claudeMessage{Role: "user", Content: req.Prompt})
		}
		return msgs
	}
	return []claudeMessage{{Role: "user", Content: req.Prompt}}
}
