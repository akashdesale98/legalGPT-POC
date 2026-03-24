package store

import "context"

// Chunk represents a single retrieved chunk with metadata.
type Chunk struct {
	ID            string            `json:"id"`
	Text          string            `json:"text"`
	Score         float32           `json:"score"`
	Metadata      map[string]string `json:"metadata"`
	ContentHash   string            `json:"content_hash,omitempty"`
	SupersededBy  string            `json:"superseded_by,omitempty"`
	EffectiveDate string            `json:"effective_date,omitempty"`
}

// SearchOptions controls vector search behaviour.
type SearchOptions struct {
	Collection string
	TopK       int
	TenantID   string
	Filters    map[string]string
}

// VectorStore is the port for vector database operations.
type VectorStore interface {
	// Search performs a vector similarity search and returns ranked chunks.
	Search(ctx context.Context, vector []float32, opts SearchOptions) ([]Chunk, error)

	// SearchMulti fans out searches across multiple collections in parallel.
	SearchMulti(ctx context.Context, vector []float32, collections []string, opts SearchOptions) ([]Chunk, error)

	// HealthCheck verifies the vector store is reachable.
	HealthCheck(ctx context.Context) error
}
