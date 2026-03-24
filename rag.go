package main

import (
	"context"
	"fmt"
	"strings"

	"github.com/qdrant/go-client/qdrant"
)

// ---------------------------------------------------------------------------
// Domain types
// ---------------------------------------------------------------------------

// SearchResult is a single retrieved chunk with its metadata and relevance score.
type SearchResult struct {
	Text          string  `json:"text"`
	ArticleNumber string  `json:"article_number,omitempty"`
	ArticleTitle  string  `json:"article_title,omitempty"`
	Part          string  `json:"part,omitempty"`
	Source        string  `json:"source,omitempty"`
	DocumentType  string  `json:"document_type,omitempty"`
	Score         float32 `json:"score"`
}

// ---------------------------------------------------------------------------
// Vector search
// ---------------------------------------------------------------------------

// SearchQdrant performs a nearest-neighbour query and returns ranked results.
func SearchQdrant(
	client *qdrant.Client,
	collection string,
	vector []float32,
	topK int,
) ([]SearchResult, error) {
	ctx := context.Background()

	points, err := client.Query(ctx, &qdrant.QueryPoints{
		CollectionName: collection,
		Query:          qdrant.NewQuery(vector...),
		WithPayload:    qdrant.NewWithPayload(true),
		Limit:          qdrant.PtrOf(uint64(topK)),
	})
	if err != nil {
		return nil, fmt.Errorf("qdrant query: %w", err)
	}

	results := make([]SearchResult, 0, len(points))
	for _, p := range points {
		sr := SearchResult{Score: p.Score}
		if v, ok := p.Payload["text"]; ok {
			sr.Text = v.GetStringValue()
		}
		if v, ok := p.Payload["article_number"]; ok {
			sr.ArticleNumber = v.GetStringValue()
		}
		if v, ok := p.Payload["article_title"]; ok {
			sr.ArticleTitle = v.GetStringValue()
		}
		if v, ok := p.Payload["part"]; ok {
			sr.Part = v.GetStringValue()
		}
		if v, ok := p.Payload["source"]; ok {
			sr.Source = v.GetStringValue()
		}
		if v, ok := p.Payload["document_type"]; ok {
			sr.DocumentType = v.GetStringValue()
		}
		results = append(results, sr)
	}
	return results, nil
}

// ---------------------------------------------------------------------------
// Prompt builder
// ---------------------------------------------------------------------------

// BuildRAGPrompt assembles system + user messages for the LLM.
// The prompt is tuned for Indian legal contexts but the structure is
// generic enough to serve any legal domain.
func BuildRAGPrompt(query string, results []SearchResult) []ChatMessage {
	var ctx strings.Builder
	for i, r := range results {
		citation := fmt.Sprintf("Article %s", r.ArticleNumber)
		if r.ArticleTitle != "" {
			citation += " — " + r.ArticleTitle
		}
		if r.Part != "" {
			citation += " (" + r.Part + ")"
		}
		fmt.Fprintf(&ctx, "[%d] %s\n%s\n\n", i+1, citation, r.Text)
	}

	system := `You are an expert on Indian Constitutional Law. Your audience includes lawyers, law students (LLB/LLM), UPSC/MPSC aspirants, and the general public.

Rules:
1. Answer only from the provided constitutional provisions.
2. Always cite the Article number (e.g., "Article 21").
3. If multiple articles are relevant, mention each one clearly.
4. If the answer is NOT in the provided context, say: "This is not covered in the retrieved provisions. Please consult a qualified lawyer."
5. Keep language clear — accessible to both legal professionals and laypersons.`

	user := fmt.Sprintf(
		"Constitutional provisions:\n\n%s\nQuestion: %s\n\nAnswer (with Article citations):",
		ctx.String(),
		query,
	)

	return []ChatMessage{
		{Role: "system", Content: system},
		{Role: "user", Content: user},
	}
}
