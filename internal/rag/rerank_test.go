package rag

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"testqdrant/internal/store"
)

func testChunks() []store.Chunk {
	return []store.Chunk{
		{ID: "1", Text: "Section 302 BNS defines murder.", Score: 0.8},
		{ID: "2", Text: "Article 21 guarantees right to life.", Score: 0.7},
		{ID: "3", Text: "Section 103 BNS covers right of private defence.", Score: 0.6},
		{ID: "4", Text: "The preamble declares India a sovereign republic.", Score: 0.5},
	}
}

func TestCohereReranker_Rerank(t *testing.T) {
	// Mock Cohere API
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if r.Header.Get("Authorization") != "Bearer test-key" {
			t.Errorf("missing or wrong Authorization header")
		}

		resp := cohereRerankResponse{
			Results: []cohereRerankResult{
				{Index: 2, RelevanceScore: 0.95},
				{Index: 0, RelevanceScore: 0.90},
				{Index: 1, RelevanceScore: 0.50},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	rr := NewCohereReranker("test-key", "rerank-v3.5")
	rr.client = server.Client()

	// Override the URL by creating a custom reranker that targets our test server
	original := rr.client.Transport
	rr.client.Transport = rewriteTransport{base: original, url: server.URL}

	chunks := testChunks()
	reranked, err := rr.Rerank(context.Background(), "what is murder", chunks, 3)
	if err != nil {
		t.Fatalf("Rerank failed: %v", err)
	}

	if len(reranked) != 3 {
		t.Fatalf("expected 3 results, got %d", len(reranked))
	}

	// First result should be chunk index 2 (highest relevance score)
	if reranked[0].ID != "3" {
		t.Errorf("first reranked chunk should be ID=3 (index 2), got ID=%s", reranked[0].ID)
	}
	if reranked[0].Score != 0.95 {
		t.Errorf("first score should be 0.95, got %f", reranked[0].Score)
	}
}

func TestCohereReranker_EmptyChunks(t *testing.T) {
	rr := NewCohereReranker("key", "")
	chunks, err := rr.Rerank(context.Background(), "query", nil, 5)
	if err != nil {
		t.Fatal(err)
	}
	if len(chunks) != 0 {
		t.Error("empty input should return empty output")
	}
}

func TestLocalReranker_Rerank(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req ollamaGenerateRequest
		json.NewDecoder(r.Body).Decode(&req)

		resp := ollamaGenerateResponse{Response: "0.85"}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	rr := NewLocalReranker(server.URL, "bge-reranker-v2-m3")
	chunks := testChunks()

	reranked, err := rr.Rerank(context.Background(), "what is murder", chunks, 2)
	if err != nil {
		t.Fatalf("Rerank failed: %v", err)
	}

	if len(reranked) != 2 {
		t.Fatalf("expected 2 results, got %d", len(reranked))
	}
}

func TestTruncateText(t *testing.T) {
	short := "hello"
	if truncateText(short, 10) != short {
		t.Error("short text should not be truncated")
	}

	long := "abcdefghij"
	result := truncateText(long, 5)
	if result != "abcde..." {
		t.Errorf("expected 'abcde...', got %q", result)
	}
}

// rewriteTransport rewrites request URLs to point at the test server.
type rewriteTransport struct {
	base http.RoundTripper
	url  string
}

func (t rewriteTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	testReq := req.Clone(req.Context())
	testReq.URL.Scheme = "http"
	testReq.URL.Host = t.url[len("http://"):]

	if t.base != nil {
		return t.base.RoundTrip(testReq)
	}
	return http.DefaultTransport.RoundTrip(testReq)
}
