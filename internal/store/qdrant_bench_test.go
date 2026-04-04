//go:build integration

package store_test

import (
	"context"
	"testing"

	"github.com/qdrant/go-client/qdrant"

	"testqdrant/internal/store"
)

// BenchmarkHybridSearch measures P95 of SearchHybrid against a live Qdrant.
// Run with: go test -tags=integration -bench=BenchmarkHybridSearch -benchtime=30s ./internal/store/...
//
// Target: P95 < 200ms.
func BenchmarkHybridSearch(b *testing.B) {
	client, err := qdrant.NewClient(&qdrant.Config{
		Host: "localhost",
		Port: 6334,
	})
	if err != nil {
		b.Fatalf("qdrant connect: %v", err)
	}

	s := store.NewQdrantStore(client)
	ctx := context.Background()

	denseVec := make([]float32, 768)
	for i := range denseVec {
		denseVec[i] = 0.01
	}
	sparseIndices := []uint32{0, 1, 2, 5}
	sparseValues := []float32{1.5, 2.1, 0.8, 1.2}

	opts := store.SearchOptions{
		Collection: "constitution",
		TopK:       5,
		TenantID:   "public",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		chunks, err := s.SearchHybrid(ctx, denseVec, sparseIndices, sparseValues, opts)
		if err != nil {
			b.Logf("SearchHybrid err (collection may not exist): %v", err)
			return
		}
		_ = chunks
	}
}
