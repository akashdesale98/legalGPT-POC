package rag

import (
	"context"
	"fmt"
	"log"
	"path/filepath"

	"testqdrant/internal/store"
)

// RetrievalOptions controls retrieval behaviour.
type RetrievalOptions struct {
	Collection  string
	Collections []string // for multi-collection search
	TopK        int
	TenantID    string
}

// HybridStore is the interface needed by HybridRetriever.
// QdrantStore satisfies this; the extra SearchHybrid method is optional
// and accessed via type assertion for graceful degradation.
type HybridStore interface {
	store.VectorStore
}

// HybridRetriever performs dense + BM25 sparse hybrid search with RRF fusion.
// Falls back to dense-only if the tenant vocab is not found.
// Optionally applies cross-encoder reranking when a Reranker is configured.
type HybridRetriever struct {
	store    HybridStore
	vocabDir string   // directory containing <tenant_id>.json vocab files
	reranker Reranker // optional; nil means no reranking
}

// NewHybridRetriever creates a HybridRetriever.
// vocabDir defaults to "data/vocab" if empty.
func NewHybridRetriever(s HybridStore, vocabDir ...string) *HybridRetriever {
	dir := "data/vocab"
	if len(vocabDir) > 0 && vocabDir[0] != "" {
		dir = vocabDir[0]
	}
	return &HybridRetriever{store: s, vocabDir: dir}
}

// SetReranker configures an optional cross-encoder reranker.
// When set, Retrieve() applies reranking after initial retrieval.
func (r *HybridRetriever) SetReranker(rr Reranker) {
	r.reranker = rr
}

// hybridSearcher is satisfied by *store.QdrantStore.
type hybridSearcher interface {
	SearchHybrid(ctx context.Context, denseVec []float32, sparseIndices []uint32, sparseValues []float32, opts store.SearchOptions) ([]store.Chunk, error)
}

// Retrieve performs hybrid (dense + BM25) search for a single collection,
// or multi-collection dense search when opts.Collections is set.
func (r *HybridRetriever) Retrieve(ctx context.Context, queryVector []float32, queryText string, opts RetrievalOptions) ([]store.Chunk, error) {
	topK := opts.TopK
	if topK <= 0 {
		topK = 20
	}

	searchOpts := store.SearchOptions{
		Collection: opts.Collection,
		TopK:       topK,
		TenantID:   opts.TenantID,
	}

	// Multi-collection path: dense-only fan-out (sparse not yet supported across collections)
	if len(opts.Collections) > 0 {
		chunks, err := r.store.SearchMulti(ctx, queryVector, opts.Collections, searchOpts)
		if err != nil {
			return nil, err
		}
		return r.applyReranking(ctx, queryText, chunks, topK)
	}

	// Single-collection: attempt hybrid if QdrantStore supports it and vocab exists
	hs, ok := r.store.(hybridSearcher)
	if !ok {
		chunks, err := r.store.Search(ctx, queryVector, searchOpts)
		if err != nil {
			return nil, err
		}
		return r.applyReranking(ctx, queryText, chunks, topK)
	}

	vocabPath := filepath.Join(r.vocabDir, opts.TenantID+".json")
	vocab, err := loadVocab(vocabPath)
	if err != nil {
		log.Printf("RETRIEVAL: mode=dense_only reason=vocab_missing tenant=%s", opts.TenantID)
		chunks, searchErr := r.store.Search(ctx, queryVector, searchOpts)
		if searchErr != nil {
			return nil, searchErr
		}
		return r.applyReranking(ctx, queryText, chunks, topK)
	}

	sparseIndices, sparseValues := vocab.encodeQuery(queryText)
	if len(sparseIndices) == 0 {
		log.Printf("RETRIEVAL: mode=dense_only reason=empty_sparse tenant=%s", opts.TenantID)
		chunks, searchErr := r.store.Search(ctx, queryVector, searchOpts)
		if searchErr != nil {
			return nil, searchErr
		}
		return r.applyReranking(ctx, queryText, chunks, topK)
	}

	chunks, err := hs.SearchHybrid(ctx, queryVector, sparseIndices, sparseValues, searchOpts)
	if err != nil {
		return nil, fmt.Errorf("hybrid retrieval: %w", err)
	}

	log.Printf("RETRIEVAL: mode=hybrid dense_sparse_hits=%d tenant=%s", len(chunks), opts.TenantID)
	return r.applyReranking(ctx, queryText, chunks, opts.TopK)
}

// applyReranking runs the configured reranker if available.
// Falls back to returning chunks as-is on error or if no reranker is set.
func (r *HybridRetriever) applyReranking(ctx context.Context, query string, chunks []store.Chunk, topN int) ([]store.Chunk, error) {
	if r.reranker == nil || len(chunks) == 0 {
		return chunks, nil
	}

	reranked, err := r.reranker.Rerank(ctx, query, chunks, topN)
	if err != nil {
		log.Printf("RERANK: fallback=original reason=%v", err)
		return chunks, nil
	}
	return reranked, nil
}
