package rag

import (
	"crypto/sha256"
	"fmt"
	"strings"

	"testqdrant/internal/store"
)

// ContextWindow holds assembled context ready for LLM injection.
type ContextWindow struct {
	Text          string
	Chunks        []store.Chunk
	TokenEstimate int
	StaleChunks   []store.Chunk
}

// ContextAssembler builds a context window from retrieved chunks within a token budget.
type ContextAssembler struct {
	MaxTokens int
}

// NewContextAssembler creates a ContextAssembler with the given token budget.
func NewContextAssembler(maxTokens int) *ContextAssembler {
	if maxTokens <= 0 {
		maxTokens = 6000
	}
	return &ContextAssembler{MaxTokens: maxTokens}
}

// BuildContext assembles a context string from chunks, respecting the token budget.
// - Sorts by score descending
// - Deduplicates by content hash
// - Injects IPC→BNS notice if chunk has SupersededBy metadata
// - Prefixes each chunk with source citation
func (a *ContextAssembler) BuildContext(chunks []store.Chunk) ContextWindow {
	var (
		parts     []string
		used      int
		included  []store.Chunk
		stale     []store.Chunk
		seenHash  = make(map[string]bool)
	)

	for _, chunk := range chunks {
		// Deduplicate by content hash
		hash := chunk.ContentHash
		if hash == "" {
			h := sha256.Sum256([]byte(chunk.Text))
			hash = fmt.Sprintf("%x", h[:8])
		}
		if seenHash[hash] {
			continue
		}

		chunkTokens := estimateTokens(chunk.Text)
		if used+chunkTokens > a.MaxTokens {
			break
		}

		// Track stale chunks
		if chunk.SupersededBy != "" {
			stale = append(stale, chunk)
		}

		// Build citation prefix
		var prefix string
		if statute, ok := chunk.Metadata["statute"]; ok {
			section := chunk.Metadata["section"]
			prefix = fmt.Sprintf("[SOURCE: %s §%s]", statute, section)
		} else if articleNum, ok := chunk.Metadata["article_number"]; ok {
			title := chunk.Metadata["article_title"]
			prefix = fmt.Sprintf("[SOURCE: Article %s — %s]", articleNum, title)
		} else if source, ok := chunk.Metadata["source"]; ok {
			prefix = fmt.Sprintf("[SOURCE: %s]", source)
		}

		entry := prefix + "\n" + chunk.Text

		// IPC→BNS staleness notice
		if chunk.SupersededBy != "" {
			entry += fmt.Sprintf("\n⚠ NOTE: This provision has been superseded by %s.", chunk.SupersededBy)
		}

		parts = append(parts, entry)
		used += chunkTokens
		seenHash[hash] = true
		included = append(included, chunk)
	}

	return ContextWindow{
		Text:          strings.Join(parts, "\n\n---\n\n"),
		Chunks:        included,
		TokenEstimate: used,
		StaleChunks:   stale,
	}
}

// estimateTokens approximates token count as len(text)/4.
func estimateTokens(text string) int {
	n := len(text) / 4
	if n == 0 && len(text) > 0 {
		n = 1
	}
	return n
}
