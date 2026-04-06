package rag

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"sort"
	"time"

	"testqdrant/internal/store"
)

// Reranker scores and reorders retrieved chunks by relevance to the query.
type Reranker interface {
	Rerank(ctx context.Context, query string, chunks []store.Chunk, topN int) ([]store.Chunk, error)
}

// CohereReranker calls the Cohere Rerank API.
type CohereReranker struct {
	apiKey string
	model  string
	client *http.Client
}

// NewCohereReranker creates a CohereReranker.
// model defaults to "rerank-v3.5" if empty.
func NewCohereReranker(apiKey, model string) *CohereReranker {
	if model == "" {
		model = "rerank-v3.5"
	}
	return &CohereReranker{
		apiKey: apiKey,
		model:  model,
		client: &http.Client{Timeout: 30 * time.Second},
	}
}

type cohereRerankRequest struct {
	Model     string   `json:"model"`
	Query     string   `json:"query"`
	Documents []string `json:"documents"`
	TopN      int      `json:"top_n"`
}

type cohereRerankResponse struct {
	Results []cohereRerankResult `json:"results"`
}

type cohereRerankResult struct {
	Index          int     `json:"index"`
	RelevanceScore float64 `json:"relevance_score"`
}

func (r *CohereReranker) Rerank(ctx context.Context, query string, chunks []store.Chunk, topN int) ([]store.Chunk, error) {
	if len(chunks) == 0 {
		return chunks, nil
	}
	if topN <= 0 || topN > len(chunks) {
		topN = len(chunks)
	}

	start := time.Now()

	docs := make([]string, len(chunks))
	for i, c := range chunks {
		docs[i] = c.Text
	}

	body, err := json.Marshal(cohereRerankRequest{
		Model:     r.model,
		Query:     query,
		Documents: docs,
		TopN:      topN,
	})
	if err != nil {
		return nil, fmt.Errorf("cohere rerank: marshal: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://api.cohere.com/v2/rerank", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("cohere rerank: request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+r.apiKey)

	resp, err := r.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("cohere rerank: request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("cohere rerank: read body: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("cohere rerank: status %d: %s", resp.StatusCode, string(respBody))
	}

	var result cohereRerankResponse
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("cohere rerank: unmarshal: %w", err)
	}

	reranked := make([]store.Chunk, 0, len(result.Results))
	for _, r := range result.Results {
		if r.Index < len(chunks) {
			c := chunks[r.Index]
			c.Score = float32(r.RelevanceScore)
			reranked = append(reranked, c)
		}
	}

	log.Printf("RERANK: provider=cohere model=%s input=%d output=%d duration_ms=%d",
		r.model, len(chunks), len(reranked), time.Since(start).Milliseconds())
	return reranked, nil
}

// LocalReranker calls a local cross-encoder model hosted via Ollama or any
// OpenAI-compatible endpoint that supports a reranking/scoring prompt.
// It sends each (query, document) pair to the model and sorts by score.
type LocalReranker struct {
	baseURL string
	model   string
	client  *http.Client
}

// NewLocalReranker creates a LocalReranker targeting an Ollama instance.
// model should be a cross-encoder model (e.g., "bge-reranker-v2-m3").
func NewLocalReranker(baseURL, model string) *LocalReranker {
	if baseURL == "" {
		baseURL = "http://localhost:11434"
	}
	return &LocalReranker{
		baseURL: baseURL,
		model:   model,
		client:  &http.Client{Timeout: 60 * time.Second},
	}
}

type ollamaGenerateRequest struct {
	Model  string `json:"model"`
	Prompt string `json:"prompt"`
	Stream bool   `json:"stream"`
}

type ollamaGenerateResponse struct {
	Response string `json:"response"`
}

func (r *LocalReranker) Rerank(ctx context.Context, query string, chunks []store.Chunk, topN int) ([]store.Chunk, error) {
	if len(chunks) == 0 {
		return chunks, nil
	}
	if topN <= 0 || topN > len(chunks) {
		topN = len(chunks)
	}

	start := time.Now()

	type scored struct {
		index int
		score float64
	}
	scores := make([]scored, len(chunks))

	for i, chunk := range chunks {
		prompt := fmt.Sprintf(
			"On a scale from 0.0 to 1.0, rate how relevant the following document is to the query. "+
				"Respond with ONLY a decimal number, nothing else.\n\n"+
				"Query: %s\n\nDocument: %s\n\nRelevance score:",
			query, truncateText(chunk.Text, 1500),
		)

		body, err := json.Marshal(ollamaGenerateRequest{
			Model:  r.model,
			Prompt: prompt,
			Stream: false,
		})
		if err != nil {
			scores[i] = scored{index: i, score: float64(chunk.Score)}
			continue
		}

		req, err := http.NewRequestWithContext(ctx, http.MethodPost, r.baseURL+"/api/generate", bytes.NewReader(body))
		if err != nil {
			scores[i] = scored{index: i, score: float64(chunk.Score)}
			continue
		}
		req.Header.Set("Content-Type", "application/json")

		resp, err := r.client.Do(req)
		if err != nil {
			scores[i] = scored{index: i, score: float64(chunk.Score)}
			continue
		}

		var genResp ollamaGenerateResponse
		if err := json.NewDecoder(resp.Body).Decode(&genResp); err != nil {
			resp.Body.Close()
			scores[i] = scored{index: i, score: float64(chunk.Score)}
			continue
		}
		resp.Body.Close()

		var s float64
		if _, err := fmt.Sscanf(genResp.Response, "%f", &s); err != nil || s < 0 || s > 1 {
			scores[i] = scored{index: i, score: float64(chunk.Score)}
			continue
		}
		scores[i] = scored{index: i, score: s}
	}

	sort.Slice(scores, func(i, j int) bool {
		return scores[i].score > scores[j].score
	})

	reranked := make([]store.Chunk, 0, topN)
	for i := 0; i < topN && i < len(scores); i++ {
		c := chunks[scores[i].index]
		c.Score = float32(scores[i].score)
		reranked = append(reranked, c)
	}

	log.Printf("RERANK: provider=local model=%s input=%d output=%d duration_ms=%d",
		r.model, len(chunks), len(reranked), time.Since(start).Milliseconds())
	return reranked, nil
}

// truncateText truncates text to maxChars to avoid sending huge prompts.
func truncateText(text string, maxChars int) string {
	if len(text) <= maxChars {
		return text
	}
	return text[:maxChars] + "..."
}
