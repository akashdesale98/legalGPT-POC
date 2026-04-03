package handler_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"

	"testqdrant/internal/llm"
	"testqdrant/internal/rag"
	"testqdrant/internal/store"
	"testqdrant/services/query/handler"
)

// --- Mock LLM Provider ---

type mockProvider struct {
	completeResp llm.CompletionResponse
	embedResp    [][]float32
	streamChunks []llm.StreamChunk
	err          error
}

func (m *mockProvider) Complete(_ context.Context, _ llm.CompletionRequest) (llm.CompletionResponse, error) {
	return m.completeResp, m.err
}

func (m *mockProvider) Stream(_ context.Context, _ llm.CompletionRequest) (<-chan llm.StreamChunk, error) {
	if m.err != nil {
		return nil, m.err
	}
	ch := make(chan llm.StreamChunk, len(m.streamChunks))
	for _, c := range m.streamChunks {
		ch <- c
	}
	close(ch)
	return ch, nil
}

func (m *mockProvider) Embed(_ context.Context, texts []string) ([][]float32, error) {
	if m.err != nil {
		return nil, m.err
	}
	if m.embedResp != nil {
		return m.embedResp, nil
	}
	result := make([][]float32, len(texts))
	for i := range result {
		result[i] = []float32{0.1, 0.2, 0.3}
	}
	return result, nil
}

func (m *mockProvider) Name() string                        { return "mock" }
func (m *mockProvider) HealthCheck(_ context.Context) error { return nil }

// --- Mock Vector Store ---

type mockStore struct {
	chunks []store.Chunk
}

func (m *mockStore) Search(_ context.Context, _ []float32, _ store.SearchOptions) ([]store.Chunk, error) {
	return m.chunks, nil
}

func (m *mockStore) SearchMulti(_ context.Context, _ []float32, _ []string, _ store.SearchOptions) ([]store.Chunk, error) {
	return m.chunks, nil
}

func (m *mockStore) HealthCheck(_ context.Context) error { return nil }

// --- Tests ---

func setupRouter(chatH *handler.ChatHandler) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.POST("/api/v1/chat", chatH.Handle)
	return r
}

func TestChat_HappyPath(t *testing.T) {
	ms := &mockStore{
		chunks: []store.Chunk{
			{Text: "Section 103 BNS — Punishment for murder.", Score: 0.85, Metadata: map[string]string{
				"article_number": "103",
				"article_title":  "Punishment for murder",
				"source":         "bns",
			}},
		},
	}

	mp := &mockProvider{
		streamChunks: []llm.StreamChunk{
			{Text: "Section 103 BNS provides "},
			{Text: "punishment for murder.", Done: true},
		},
	}

	chatH := &handler.ChatHandler{
		Provider:  mp,
		Retriever: rag.NewHybridRetriever(ms),
		Guardrail: rag.NewGuardrail(),
		Assembler: rag.NewContextAssembler(6000),
	}

	body, _ := json.Marshal(handler.ChatRequest{
		Query: "What is the punishment for murder?",
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/chat", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")

	router := setupRouter(chatH)
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200. Body: %s", w.Code, w.Body.String())
	}

	respBody := w.Body.String()

	// Check SSE events are present
	if !strings.Contains(respBody, "event: chunk") {
		t.Error("missing 'chunk' SSE event")
	}
	if !strings.Contains(respBody, "event: sources") {
		t.Error("missing 'sources' SSE event")
	}
	if !strings.Contains(respBody, "event: meta") {
		t.Error("missing 'meta' SSE event")
	}
	if !strings.Contains(respBody, "event: done") {
		t.Error("missing 'done' SSE event")
	}
}

func TestChat_AbstainOnLowConfidence(t *testing.T) {
	ms := &mockStore{
		chunks: []store.Chunk{
			{Text: "Unrelated text.", Score: 0.50, Metadata: map[string]string{}},
		},
	}

	mp := &mockProvider{}

	chatH := &handler.ChatHandler{
		Provider:  mp,
		Retriever: rag.NewHybridRetriever(ms),
		Guardrail: rag.NewGuardrail(),
		Assembler: rag.NewContextAssembler(6000),
	}

	body, _ := json.Marshal(handler.ChatRequest{
		Query: "What is quantum physics?",
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/chat", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")

	router := setupRouter(chatH)
	router.ServeHTTP(w, req)

	respBody := w.Body.String()
	if !strings.Contains(respBody, "event: abstain") {
		t.Error("expected 'abstain' SSE event for low confidence query")
	}
}

func TestChat_RejectsEmptyQuery(t *testing.T) {
	chatH := &handler.ChatHandler{
		Provider:  &mockProvider{},
		Retriever: rag.NewHybridRetriever(&mockStore{}),
		Guardrail: rag.NewGuardrail(),
		Assembler: rag.NewContextAssembler(6000),
	}

	body, _ := json.Marshal(map[string]string{})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/chat", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")

	router := setupRouter(chatH)
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400 for empty query", w.Code)
	}
}
