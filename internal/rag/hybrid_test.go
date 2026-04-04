package rag_test

import (
	"context"
	"testing"

	"testqdrant/internal/rag"
	"testqdrant/internal/store"
)

// mockVectorStore implements store.VectorStore for testing.
// It respects TopK in SearchOptions.
type mockVectorStore struct {
	chunks []store.Chunk
	err    error
}

func (m *mockVectorStore) Search(_ context.Context, _ []float32, opts store.SearchOptions) ([]store.Chunk, error) {
	if m.err != nil {
		return nil, m.err
	}
	result := m.chunks
	if opts.TopK > 0 && len(result) > opts.TopK {
		result = result[:opts.TopK]
	}
	return result, nil
}

func (m *mockVectorStore) SearchMulti(_ context.Context, _ []float32, _ []string, opts store.SearchOptions) ([]store.Chunk, error) {
	if m.err != nil {
		return nil, m.err
	}
	result := m.chunks
	if opts.TopK > 0 && len(result) > opts.TopK {
		result = result[:opts.TopK]
	}
	return result, nil
}

func (m *mockVectorStore) HealthCheck(_ context.Context) error {
	return nil
}

// mockHybridStore implements store.VectorStore + hybridSearcher for testing hybrid path.
type mockHybridStore struct {
	mockVectorStore
	hybridChunks []store.Chunk
	hybridErr    error
	hybridCalled bool
}

func (m *mockHybridStore) SearchHybrid(_ context.Context, _ []float32, _ []uint32, _ []float32, opts store.SearchOptions) ([]store.Chunk, error) {
	m.hybridCalled = true
	if m.hybridErr != nil {
		return nil, m.hybridErr
	}
	result := m.hybridChunks
	if opts.TopK > 0 && len(result) > opts.TopK {
		result = result[:opts.TopK]
	}
	return result, nil
}

// TestHybridRetriever_DenseOnly verifies fallback when vocab is missing.
func TestHybridRetriever_DenseOnly(t *testing.T) {
	mockStore := &mockVectorStore{
		chunks: []store.Chunk{
			{Text: "Punishment for murder under BNS Section 103.", Score: 0.82},
			{Text: "This section covers theft and robbery.", Score: 0.80},
			{Text: "General provisions about criminal law.", Score: 0.78},
		},
	}

	// vocabDir points to a non-existent directory → dense-only fallback
	retriever := rag.NewHybridRetriever(mockStore, "testdata/nonexistent_vocab")
	chunks, err := retriever.Retrieve(context.Background(), []float32{0.1}, "murder punishment BNS", rag.RetrievalOptions{
		Collection: "test",
		TopK:       10,
		TenantID:   "public",
	})
	if err != nil {
		t.Fatalf("Retrieve() error: %v", err)
	}
	if len(chunks) == 0 {
		t.Fatal("expected at least 1 chunk")
	}
}

// TestHybridRetriever_TopKLimit verifies TopK is respected.
func TestHybridRetriever_TopKLimit(t *testing.T) {
	chunks := make([]store.Chunk, 20)
	for i := range chunks {
		chunks[i] = store.Chunk{Text: "chunk", Score: float32(20-i) / 20.0}
	}

	mockStore := &mockVectorStore{chunks: chunks}
	retriever := rag.NewHybridRetriever(mockStore, "testdata/nonexistent_vocab")

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

// TestHybridRetriever_MultiCollection verifies dense fan-out for multi-collection queries.
func TestHybridRetriever_MultiCollection(t *testing.T) {
	mockStore := &mockVectorStore{
		chunks: []store.Chunk{
			{Text: "BNS result", Score: 0.9},
			{Text: "BNSS result", Score: 0.8},
		},
	}

	retriever := rag.NewHybridRetriever(mockStore, "testdata/nonexistent_vocab")
	result, err := retriever.Retrieve(context.Background(), []float32{0.1}, "query", rag.RetrievalOptions{
		Collections: []string{"bns", "bnss"},
		TopK:        10,
		TenantID:    "public",
	})
	if err != nil {
		t.Fatalf("Retrieve() error: %v", err)
	}
	if len(result) == 0 {
		t.Fatal("expected results from multi-collection search")
	}
}

// TestHybridRetriever_HybridPath verifies hybrid search is used when vocab exists.
func TestHybridRetriever_HybridPath(t *testing.T) {
	hybridChunks := []store.Chunk{
		{Text: "Hybrid result 1", Score: 0.95},
		{Text: "Hybrid result 2", Score: 0.85},
	}
	mockStore := &mockHybridStore{
		hybridChunks: hybridChunks,
	}

	// Point vocabDir to our test fixture
	retriever := rag.NewHybridRetriever(mockStore, "testdata/vocab")
	result, err := retriever.Retrieve(context.Background(), []float32{0.1}, "murder punishment", rag.RetrievalOptions{
		Collection: "test",
		TopK:       10,
		TenantID:   "public",
	})
	if err != nil {
		t.Fatalf("Retrieve() error: %v", err)
	}
	if !mockStore.hybridCalled {
		t.Error("expected SearchHybrid to be called when vocab exists")
	}
	if len(result) == 0 {
		t.Fatal("expected hybrid results")
	}
}
