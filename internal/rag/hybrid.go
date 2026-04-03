package rag

import (
	"context"
	"sort"
	"strings"

	"testqdrant/internal/store"
)

// RetrievalOptions controls retrieval behaviour.
type RetrievalOptions struct {
	Collection  string
	Collections []string // for multi-collection search
	TopK        int
	TenantID    string
}

// HybridRetriever performs dense vector search with keyword boosting.
// Phase 1: Dense search + keyword boost re-scoring.
// ARCH: BM25 sparse + cross-encoder reranking deferred to Phase 2 (requires sparse index).
type HybridRetriever struct {
	store store.VectorStore
}

// NewHybridRetriever creates a HybridRetriever with the given VectorStore.
func NewHybridRetriever(s store.VectorStore) *HybridRetriever {
	return &HybridRetriever{store: s}
}

const keywordBoostFactor = 0.1

// Retrieve performs dense search and applies keyword boost re-scoring.
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

	var chunks []store.Chunk
	var err error

	if len(opts.Collections) > 0 {
		chunks, err = r.store.SearchMulti(ctx, queryVector, opts.Collections, searchOpts)
	} else {
		chunks, err = r.store.Search(ctx, queryVector, searchOpts)
	}
	if err != nil {
		return nil, err
	}

	// Keyword boost: re-score results that contain exact query terms
	queryTerms := extractQueryTerms(queryText)
	for i := range chunks {
		lowerText := strings.ToLower(chunks[i].Text)
		for _, term := range queryTerms {
			if strings.Contains(lowerText, term) {
				chunks[i].Score += keywordBoostFactor
			}
		}
	}

	// Re-sort by adjusted score
	sort.Slice(chunks, func(i, j int) bool {
		return chunks[i].Score > chunks[j].Score
	})

	// Trim to requested topK
	if len(chunks) > topK {
		chunks = chunks[:topK]
	}

	return chunks, nil
}

// extractQueryTerms splits query into lowercase terms, filtering out stop words.
func extractQueryTerms(query string) []string {
	stopWords := map[string]bool{
		"the": true, "a": true, "an": true, "is": true, "are": true,
		"was": true, "were": true, "be": true, "been": true, "being": true,
		"have": true, "has": true, "had": true, "do": true, "does": true,
		"did": true, "will": true, "would": true, "could": true, "should": true,
		"may": true, "might": true, "shall": true, "can": true,
		"of": true, "in": true, "to": true, "for": true, "with": true,
		"on": true, "at": true, "from": true, "by": true, "about": true,
		"as": true, "into": true, "through": true, "during": true,
		"and": true, "or": true, "but": true, "not": true, "what": true,
		"which": true, "who": true, "whom": true, "this": true, "that": true,
		"under": true, "how": true, "when": true, "where": true, "why": true,
	}

	words := strings.Fields(strings.ToLower(query))
	terms := make([]string, 0, len(words))
	for _, w := range words {
		w = strings.Trim(w, ".,;:!?\"'()[]{}") // strip punctuation
		if len(w) > 1 && !stopWords[w] {
			terms = append(terms, w)
		}
	}
	return terms
}
