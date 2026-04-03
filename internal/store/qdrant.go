package store

import (
	"context"
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/qdrant/go-client/qdrant"
)

const (
	QdrantSearchTimeout = 5 * time.Second
)

// QdrantStore implements VectorStore using the Qdrant vector database.
type QdrantStore struct {
	Client *qdrant.Client
}

// Compile-time interface check.
var _ VectorStore = (*QdrantStore)(nil)

func NewQdrantStore(client *qdrant.Client) *QdrantStore {
	return &QdrantStore{Client: client}
}

// HealthCheck verifies Qdrant is reachable.
func (s *QdrantStore) HealthCheck(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	_, err := s.Client.ListCollections(ctx)
	if err != nil {
		return fmt.Errorf("qdrant health check: %w", err)
	}
	return nil
}

// Search performs a nearest-neighbour query with mandatory tenant isolation.
// Preserved core logic from POC rag.go SearchQdrant function.
func (s *QdrantStore) Search(ctx context.Context, vector []float32, opts SearchOptions) ([]Chunk, error) {
	ctx, cancel := context.WithTimeout(ctx, QdrantSearchTimeout)
	defer cancel()

	topK := opts.TopK
	if topK <= 0 {
		topK = 5
	}

	// Mandatory tenant isolation filter
	filter := &qdrant.Filter{
		Should: []*qdrant.Condition{
			qdrant.NewMatch("tenant_id", "public"),
		},
	}
	if opts.TenantID != "" && opts.TenantID != "public" {
		filter.Should = append(filter.Should, qdrant.NewMatch("tenant_id", opts.TenantID))
	}

	// Additional filters from opts.Filters
	if len(opts.Filters) > 0 {
		must := make([]*qdrant.Condition, 0, len(opts.Filters))
		for k, v := range opts.Filters {
			must = append(must, qdrant.NewMatch(k, v))
		}
		filter.Must = must
	}

	points, err := s.Client.Query(ctx, &qdrant.QueryPoints{
		CollectionName: opts.Collection,
		Query:          qdrant.NewQuery(vector...),
		WithPayload:    qdrant.NewWithPayload(true),
		Limit:          qdrant.PtrOf(uint64(topK)),
		Filter:         filter,
	})
	if err != nil {
		return nil, fmt.Errorf("qdrant query: %w", err)
	}

	chunks := make([]Chunk, 0, len(points))
	for _, p := range points {
		chunk := Chunk{
			Score:    p.Score,
			Metadata: make(map[string]string),
		}
		for key, val := range p.Payload {
			strVal := val.GetStringValue()
			switch key {
			case "text":
				chunk.Text = strVal
			case "content_hash":
				chunk.ContentHash = strVal
			case "superseded_by":
				chunk.SupersededBy = strVal
			case "effective_date":
				chunk.EffectiveDate = strVal
			default:
				if strVal != "" {
					chunk.Metadata[key] = strVal
				}
			}
		}
		chunks = append(chunks, chunk)
	}
	return chunks, nil
}

// SearchMulti fans out searches across multiple collections in parallel.
func (s *QdrantStore) SearchMulti(ctx context.Context, vector []float32, collections []string, opts SearchOptions) ([]Chunk, error) {
	var (
		mu       sync.Mutex
		allChunks []Chunk
		errs     []error
		wg       sync.WaitGroup
	)

	for _, coll := range collections {
		wg.Add(1)
		go func(collection string) {
			defer wg.Done()
			collOpts := opts
			collOpts.Collection = collection
			chunks, err := s.Search(ctx, vector, collOpts)
			mu.Lock()
			defer mu.Unlock()
			if err != nil {
				errs = append(errs, fmt.Errorf("collection %s: %w", collection, err))
				return
			}
			allChunks = append(allChunks, chunks...)
		}(coll)
	}
	wg.Wait()

	if len(errs) > 0 && len(allChunks) == 0 {
		return nil, fmt.Errorf("all collection searches failed: %v", errs)
	}

	// Sort by score descending
	sort.Slice(allChunks, func(i, j int) bool {
		return allChunks[i].Score > allChunks[j].Score
	})

	return allChunks, nil
}
