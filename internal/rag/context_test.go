package rag_test

import (
	"strings"
	"testing"

	"testqdrant/internal/rag"
	"testqdrant/internal/store"
)

func TestContextAssembler_TokenBudget(t *testing.T) {
	assembler := rag.NewContextAssembler(100) // Very small budget

	// Each chunk is ~100 chars = ~25 tokens
	chunks := []store.Chunk{
		{Text: strings.Repeat("a", 100), Score: 0.9, Metadata: map[string]string{"source": "test"}},
		{Text: strings.Repeat("b", 100), Score: 0.8, Metadata: map[string]string{"source": "test"}},
		{Text: strings.Repeat("c", 100), Score: 0.7, Metadata: map[string]string{"source": "test"}},
		{Text: strings.Repeat("d", 100), Score: 0.6, Metadata: map[string]string{"source": "test"}},
		{Text: strings.Repeat("e", 100), Score: 0.5, Metadata: map[string]string{"source": "test"}},
	}

	result := assembler.BuildContext(chunks)

	// Should not include all 5 chunks due to token budget
	if len(result.Chunks) >= 5 {
		t.Errorf("expected fewer than 5 chunks due to budget, got %d", len(result.Chunks))
	}
	if result.TokenEstimate > 100 {
		t.Errorf("token estimate %d exceeds budget 100", result.TokenEstimate)
	}
}

func TestContextAssembler_Deduplication(t *testing.T) {
	assembler := rag.NewContextAssembler(6000)

	chunks := []store.Chunk{
		{Text: "identical chunk text", Score: 0.9, ContentHash: "abc123", Metadata: map[string]string{}},
		{Text: "identical chunk text", Score: 0.8, ContentHash: "abc123", Metadata: map[string]string{}},
		{Text: "different chunk text", Score: 0.7, ContentHash: "def456", Metadata: map[string]string{}},
	}

	result := assembler.BuildContext(chunks)

	if len(result.Chunks) != 2 {
		t.Errorf("expected 2 unique chunks after dedup, got %d", len(result.Chunks))
	}
}

func TestContextAssembler_SupersessionNotice(t *testing.T) {
	assembler := rag.NewContextAssembler(6000)

	chunks := []store.Chunk{
		{
			Text:         "IPC Section 302 defines murder.",
			Score:        0.9,
			SupersededBy: "BNS Section 103",
			Metadata:     map[string]string{"article_number": "302", "article_title": "Murder"},
		},
	}

	result := assembler.BuildContext(chunks)

	if !strings.Contains(result.Text, "superseded by BNS Section 103") {
		t.Error("expected supersession notice in context text")
	}
	if len(result.StaleChunks) != 1 {
		t.Errorf("expected 1 stale chunk, got %d", len(result.StaleChunks))
	}
}

func TestContextAssembler_SourcePrefix(t *testing.T) {
	assembler := rag.NewContextAssembler(6000)

	chunks := []store.Chunk{
		{
			Text:  "Murder is punishable under this section.",
			Score: 0.9,
			Metadata: map[string]string{
				"article_number": "103",
				"article_title":  "Punishment for murder",
			},
		},
	}

	result := assembler.BuildContext(chunks)

	if !strings.Contains(result.Text, "[SOURCE: Article 103") {
		t.Errorf("expected source prefix, got: %s", result.Text[:100])
	}
}
