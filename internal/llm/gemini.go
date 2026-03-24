package llm

import (
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
// Gemini API types
// ---------------------------------------------------------------------------

type geminiContent struct {
	Role  string       `json:"role,omitempty"`
	Parts []geminiPart `json:"parts"`
}

type geminiPart struct {
	Text string `json:"text"`
}

type geminiRequest struct {
	Contents         []geminiContent        `json:"contents"`
	SystemInstruction *geminiContent        `json:"systemInstruction,omitempty"`
	GenerationConfig  *geminiGenerationCfg  `json:"generationConfig,omitempty"`
}

type geminiGenerationCfg struct {
	MaxOutputTokens int     `json:"maxOutputTokens,omitempty"`
	Temperature     float32 `json:"temperature,omitempty"`
}

type geminiCandidate struct {
	Content      geminiContent `json:"content"`
	FinishReason string        `json:"finishReason"`
}

type geminiUsageMetadata struct {
	PromptTokenCount     int `json:"promptTokenCount"`
	CandidatesTokenCount int `json:"candidatesTokenCount"`
	TotalTokenCount      int `json:"totalTokenCount"`
}

type geminiResponse struct {
	Candidates    []geminiCandidate   `json:"candidates"`
	UsageMetadata geminiUsageMetadata `json:"usageMetadata"`
	ModelVersion  string              `json:"modelVersion"`
}

type geminiEmbedRequest struct {
	Model   string        `json:"model"`
	Content geminiContent `json:"content"`
}

type geminiEmbedBatchRequest struct {
	Requests []geminiEmbedRequest `json:"requests"`
}

type geminiEmbedValue struct {
	Values []float32 `json:"values"`
}

type geminiEmbedResponse struct {
	Embeddings []geminiEmbedValue `json:"embeddings"`
}

// ---------------------------------------------------------------------------
// GeminiProvider implements LLMProvider
// ---------------------------------------------------------------------------

type GeminiProvider struct {
	apiKey     string
	model      string
	embedModel string
	baseURL    string
	httpClient *http.Client
}

type GeminiConfig struct {
	APIKey     string
	Model      string
	EmbedModel string
	BaseURL    string
}

func NewGeminiProvider(cfg GeminiConfig) *GeminiProvider {
	baseURL := cfg.BaseURL
	if baseURL == "" {
		baseURL = "https://generativelanguage.googleapis.com"
	}
	model := cfg.Model
	if model == "" {
		model = "gemini-2.5-flash"
	}
	embedModel := cfg.EmbedModel
	if embedModel == "" {
		embedModel = "text-embedding-004"
	}
	return &GeminiProvider{
		apiKey:     cfg.APIKey,
		model:      model,
		embedModel: embedModel,
		baseURL:    baseURL,
		httpClient: &http.Client{
			Timeout: 120 * time.Second,
		},
	}
}

func (p *GeminiProvider) Name() string { return "gemini" }

func (p *GeminiProvider) HealthCheck(ctx context.Context) error {
	url := fmt.Sprintf("%s/v1beta/models/%s", p.baseURL, p.model)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return fmt.Errorf("gemini health check: %w", err)
	}
	req.Header.Set("x-goog-api-key", p.apiKey)
	resp, err := p.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("gemini health check: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("gemini health check: status %d", resp.StatusCode)
	}
	return nil
}

func (p *GeminiProvider) Complete(ctx context.Context, req CompletionRequest) (CompletionResponse, error) {
	start := time.Now()

	model := req.Model
	if model == "" {
		model = p.model
	}
	maxTokens := req.MaxTokens
	if maxTokens == 0 {
		maxTokens = 4096
	}

	gemReq := geminiRequest{
		Contents: buildGeminiContents(req),
		GenerationConfig: &geminiGenerationCfg{
			MaxOutputTokens: maxTokens,
			Temperature:     req.Temperature,
		},
	}
	if req.SystemPrompt != "" {
		gemReq.SystemInstruction = &geminiContent{
			Parts: []geminiPart{{Text: req.SystemPrompt}},
		}
	}

	body, err := json.Marshal(gemReq)
	if err != nil {
		return CompletionResponse{}, fmt.Errorf("marshal gemini request: %w", err)
	}

	url := fmt.Sprintf("%s/v1beta/models/%s:generateContent", p.baseURL, model)
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewBuffer(body))
	if err != nil {
		return CompletionResponse{}, fmt.Errorf("create gemini request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("x-goog-api-key", p.apiKey)

	resp, err := p.httpClient.Do(httpReq)
	if err != nil {
		return CompletionResponse{}, fmt.Errorf("gemini generateContent: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return CompletionResponse{}, fmt.Errorf("gemini generateContent returned %d: %s", resp.StatusCode, string(respBody))
	}

	var result geminiResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return CompletionResponse{}, fmt.Errorf("decode gemini response: %w", err)
	}

	var contentBuilder strings.Builder
	finishReason := ""
	if len(result.Candidates) > 0 {
		for _, part := range result.Candidates[0].Content.Parts {
			contentBuilder.WriteString(part.Text)
		}
		finishReason = result.Candidates[0].FinishReason
	}
	content := contentBuilder.String()

	return CompletionResponse{
		Content:      content,
		TokensIn:     result.UsageMetadata.PromptTokenCount,
		TokensOut:    result.UsageMetadata.CandidatesTokenCount,
		ModelUsed:    model,
		Latency:      time.Since(start),
		FinishReason: finishReason,
	}, nil
}

func (p *GeminiProvider) Stream(ctx context.Context, req CompletionRequest) (<-chan StreamChunk, error) {
	model := req.Model
	if model == "" {
		model = p.model
	}
	maxTokens := req.MaxTokens
	if maxTokens == 0 {
		maxTokens = 4096
	}

	gemReq := geminiRequest{
		Contents: buildGeminiContents(req),
		GenerationConfig: &geminiGenerationCfg{
			MaxOutputTokens: maxTokens,
			Temperature:     req.Temperature,
		},
	}
	if req.SystemPrompt != "" {
		gemReq.SystemInstruction = &geminiContent{
			Parts: []geminiPart{{Text: req.SystemPrompt}},
		}
	}

	body, err := json.Marshal(gemReq)
	if err != nil {
		return nil, fmt.Errorf("marshal gemini stream request: %w", err)
	}

	url := fmt.Sprintf("%s/v1beta/models/%s:streamGenerateContent?alt=sse", p.baseURL, model)
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewBuffer(body))
	if err != nil {
		return nil, fmt.Errorf("create gemini stream request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("x-goog-api-key", p.apiKey)

	resp, err := p.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("gemini stream: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		return nil, fmt.Errorf("gemini stream returned %d: %s", resp.StatusCode, string(respBody))
	}

	ch := make(chan StreamChunk, 64)
	go func() {
		defer close(ch)
		defer resp.Body.Close()

		decoder := json.NewDecoder(resp.Body)
		for {
			var result geminiResponse
			if err := decoder.Decode(&result); err != nil {
				if err != io.EOF {
					ch <- StreamChunk{Error: fmt.Errorf("gemini stream decode: %w", err)}
				}
				ch <- StreamChunk{Done: true}
				return
			}
			if len(result.Candidates) > 0 {
				for _, part := range result.Candidates[0].Content.Parts {
					if part.Text != "" {
						ch <- StreamChunk{Text: part.Text}
					}
				}
				if result.Candidates[0].FinishReason != "" {
					ch <- StreamChunk{Done: true}
					return
				}
			}
		}
	}()

	return ch, nil
}

func (p *GeminiProvider) Embed(ctx context.Context, texts []string) ([][]float32, error) {
	// Gemini supports batch embedding
	requests := make([]geminiEmbedRequest, len(texts))
	for i, text := range texts {
		requests[i] = geminiEmbedRequest{
			Model: fmt.Sprintf("models/%s", p.embedModel),
			Content: geminiContent{
				Parts: []geminiPart{{Text: text}},
			},
		}
	}

	body, err := json.Marshal(geminiEmbedBatchRequest{Requests: requests})
	if err != nil {
		return nil, fmt.Errorf("marshal gemini embed request: %w", err)
	}

	url := fmt.Sprintf("%s/v1beta/models/%s:batchEmbedContents", p.baseURL, p.embedModel)
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewBuffer(body))
	if err != nil {
		return nil, fmt.Errorf("create gemini embed request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("x-goog-api-key", p.apiKey)

	resp, err := p.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("gemini batchEmbedContents: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("gemini batchEmbedContents returned %d: %s", resp.StatusCode, string(respBody))
	}

	var result geminiEmbedResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode gemini embed response: %w", err)
	}

	embeddings := make([][]float32, len(result.Embeddings))
	for i, e := range result.Embeddings {
		embeddings[i] = e.Values
	}
	return embeddings, nil
}

func buildGeminiContents(req CompletionRequest) []geminiContent {
	if len(req.Messages) > 0 {
		contents := make([]geminiContent, 0, len(req.Messages))
		for _, m := range req.Messages {
			if m.Role == "system" {
				continue
			}
			role := m.Role
			if role == "assistant" {
				role = "model"
			}
			contents = append(contents, geminiContent{
				Role:  role,
				Parts: []geminiPart{{Text: m.Content}},
			})
		}
		return contents
	}
	return []geminiContent{
		{Role: "user", Parts: []geminiPart{{Text: req.Prompt}}},
	}
}
