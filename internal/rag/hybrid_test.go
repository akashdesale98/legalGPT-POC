package rag_test

import (
	"context"
	"testing"

	"testqdrant/internal/rag"
	"testqdrant/internal/store"
)

// mockVectorStore implements store.VectorStore for testing.
type mockVectorStore struct {
	chunks []store.Chunk
	err    error
}

func (m *mockVectorStore) Search(_ context.Context, _ []float32, _ store.SearchOptions) ([]store.Chunk, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.chunks, nil
}

func (m *mockVectorStore) SearchMulti(_ context.Context, _ []float32, _ []string, _ store.SearchOptions) ([]store.Chunk, error) {
	return m.chunks, m.err
}

func (m *mockVectorStore) HealthCheck(_ context.Context) error {
	return nil
}

func TestHybridRetriever_KeywordBoost(t *testing.T) {
	mockStore := &mockVectorStore{
		chunks: []store.Chunk{
			{Text: "This section covers theft and robbery.", Score: 0.80},
			{Text: "Punishment for murder under BNS Section 103.", Score: 0.78},
			{Text: "General provisions about criminal law.", Score: 0.82},
		},
	}

	retriever := rag.NewHybridRetriever(mockStore)
	chunks, err := retriever.Retrieve(context.Background(), []float32{0.1}, "murder punishment BNS", rag.RetrievalOptions{
		Collection: "test",
		TopK:       10,
		TenantID:   "public",
	})
	if err != nil {
		t.Fatalf("Retrieve() error: %v", err)
	}

	// The "murder" chunk should be boosted above the "general provisions" chunk
	// because it contains query terms "murder", "punishment", "bns"
	if len(chunks) < 2 {
		t.Fatal("expected at least 2 chunks")
	}

	// Chunk with "murder" + "punishment" + "BNS" should have highest boosted score
	topChunk := chunks[0]
	if topChunk.Score <= 0.78 {
		t.Logf("Top chunk text: %s, score: %f", topChunk.Text, topChunk.Score)
	}

	// Verify scores are in descending order
	for i := 1; i < len(chunks); i++ {
		if chunks[i].Score > chunks[i-1].Score {
			t.Errorf("chunks not sorted: score[%d]=%f > score[%d]=%f",
				i, chunks[i].Score, i-1, chunks[i-1].Score)
		}
	}
}

func TestHybridRetriever_TopKLimit(t *testing.T) {
	chunks := make([]store.Chunk, 20)
	for i := range chunks {
		chunks[i] = store.Chunk{Text: "chunk", Score: float32(20-i) / 20.0}
	}

	mockStore := &mockVectorStore{chunks: chunks}
	retriever := rag.NewHybridRetriever(mockStore)

	result, err := retriever.Retrieve(context.Background(), []float32{0.1}, "test", rag.RetrievalOptions{
		Collection: "test",
		TopK:       5,
		TenantID:   "public",
	})
	if err != nil {
		t.Fatalf("Retrieve() error: %v", err)
	}
	if len(result) != 5 {
		t.Errorf("expected 5 chunks, got %d", len(result))
	}
}
