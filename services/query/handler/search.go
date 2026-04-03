package handler

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"testqdrant/internal/llm"
	"testqdrant/internal/rag"
)

// SearchHandler handles non-streaming search requests.
type SearchHandler struct {
	Provider          llm.LLMProvider
	Retriever         *rag.HybridRetriever
	DefaultCollection string
}

// SearchRequest is the JSON body for POST /api/v1/search.
type SearchRequest struct {
	Query       string   `json:"query" binding:"required"`
	Collections []string `json:"collections"`
	TopK        int      `json:"top_k"`
}

// SearchResultItem is a single result in the search response.
type SearchResultItem struct {
	Content  string            `json:"content"`
	Metadata map[string]string `json:"metadata"`
	Score    float32           `json:"score"`
}

// SearchResponse is the JSON response for POST /api/v1/search.
type SearchResponse struct {
	Results     []SearchResultItem `json:"results"`
	QueryTimeMS int64              `json:"query_time_ms"`
}

// Handle processes a search request.
// POST /api/v1/search
func (h *SearchHandler) Handle(c *gin.Context) {
	var req SearchRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	query, err := sanitizeQuery(req.Query)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	ctx := c.Request.Context()
	start := time.Now()

	// Get tenant from context (set by auth middleware)
	tenantID, _ := c.Get("tenant_id")
	tenant, _ := tenantID.(string)
	if tenant == "" {
		tenant = "public"
	}

	topK := req.TopK
	if topK <= 0 {
		topK = 10
	}

	// Embed the query
	embeddings, err := h.Provider.Embed(ctx, []string{query})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "embedding failed: " + err.Error()})
		return
	}

	// Retrieve
	collection := h.DefaultCollection
	if len(req.Collections) > 0 {
		collection = req.Collections[0]
	}
	chunks, err := h.Retriever.Retrieve(ctx, embeddings[0], query, rag.RetrievalOptions{
		Collection:  collection,
		Collections: req.Collections,
		TopK:        topK,
		TenantID:    tenant,
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "retrieval failed: " + err.Error()})
		return
	}

	results := make([]SearchResultItem, len(chunks))
	for i, chunk := range chunks {
		results[i] = SearchResultItem{
			Content:  chunk.Text,
			Metadata: chunk.Metadata,
			Score:    chunk.Score,
		}
	}

	c.JSON(http.StatusOK, SearchResponse{
		Results:     results,
		QueryTimeMS: time.Since(start).Milliseconds(),
	})
}
