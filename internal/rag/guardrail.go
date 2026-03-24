package rag

import (
	"crypto/sha256"
	"fmt"
	"log"
	"regexp"
	"strings"

	"testqdrant/internal/store"
)

// AbstentionThreshold is the minimum top chunk score for a confident answer.
const AbstentionThreshold float32 = 0.72

// GuardrailResult holds the outcome of a guardrail check.
type GuardrailResult struct {
	ShouldAbstain   bool
	Reason          string  // "low_confidence" | "off_corpus" | "injection_detected"
	ConfidenceScore float32
}

// Guardrail checks whether the system should abstain from answering.
type Guardrail struct {
	injectionPatterns []*regexp.Regexp
}

// NewGuardrail creates a Guardrail with standard injection detection patterns.
func NewGuardrail() *Guardrail {
	patterns := []string{
		`(?i)ignore\s+(all\s+)?previous\s+instructions`,
		`(?i)ignore\s+(all\s+)?above\s+instructions`,
		`(?i)you\s+are\s+now\s+`,
		`(?i)disregard\s+(all\s+)?prior`,
		`(?i)system\s*prompt`,
		`(?i)forget\s+everything`,
		`(?i)new\s+instructions?:`,
		`(?i)override\s+(the\s+)?system`,
		`(?i)act\s+as\s+(a|an)\s+`,
		`(?i)pretend\s+(you\s+are|to\s+be)`,
		`(?i)jailbreak`,
		`(?i)\bDAN\b`,
	}

	compiled := make([]*regexp.Regexp, 0, len(patterns))
	for _, p := range patterns {
		compiled = append(compiled, regexp.MustCompile(p))
	}

	return &Guardrail{injectionPatterns: compiled}
}

// Check evaluates whether to abstain based on retrieval results and query.
func (g *Guardrail) Check(chunks []store.Chunk, query string) GuardrailResult {
	// 1. No chunks returned — off corpus
	if len(chunks) == 0 {
		logAbstention("off_corpus", query)
		return GuardrailResult{
			ShouldAbstain:   true,
			Reason:          "off_corpus",
			ConfidenceScore: 0,
		}
	}

	// 2. Prompt injection detection
	for _, pattern := range g.injectionPatterns {
		if pattern.MatchString(query) {
			logAbstention("injection_detected", query)
			return GuardrailResult{
				ShouldAbstain:   true,
				Reason:          "injection_detected",
				ConfidenceScore: 0,
			}
		}
	}

	// 3. Low confidence — top chunk below threshold
	topScore := chunks[0].Score
	if topScore < AbstentionThreshold {
		logAbstention("low_confidence", query)
		return GuardrailResult{
			ShouldAbstain:   true,
			Reason:          "low_confidence",
			ConfidenceScore: topScore,
		}
	}

	return GuardrailResult{
		ShouldAbstain:   false,
		ConfidenceScore: topScore,
	}
}

// logAbstention logs an abstention event with the query hash (never the raw query for privacy).
func logAbstention(reason, query string) {
	// Hash the query for privacy — never log raw user queries
	h := sha256.Sum256([]byte(strings.TrimSpace(query)))
	queryHash := fmt.Sprintf("%x", h[:16])
	log.Printf("ABSTENTION: reason=%s query_hash=%s", reason, queryHash)
}
