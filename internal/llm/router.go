package llm

import (
	"context"
	"fmt"
	"log"
	"regexp"
	"strings"
)

// Compile-time check that Router implements LLMProvider.
var _ LLMProvider = (*Router)(nil)

var sectionCiteRE = regexp.MustCompile(`(?i)section\s+\d+|sec\.\s*\d+`)

// Router implements LLMProvider, routing to simple (Gemini) or complex (Claude)
// based on query signals. Transparent substitution — callers unchanged.
type Router struct {
	simple    LLMProvider // Gemini 2.5 Flash — for simple queries
	complex   LLMProvider // Claude Sonnet — for complex queries
	threshold int         // token estimate threshold
}

// NewRouter creates a Router.
func NewRouter(simple, complex LLMProvider, threshold int) *Router {
	return &Router{
		simple:    simple,
		complex:   complex,
		threshold: threshold,
	}
}

// route returns the provider and reason string.
// routing_reason format: "gemini:simple:section_cite:tokens=234"
//
//	or: "claude:complex:multi_part:tokens=891"
func (r *Router) route(req CompletionRequest) (LLMProvider, string) {
	tokens := estimateTokens(req)
	prompt := req.Prompt

	// Complex signals — any one triggers Claude.
	promptLower := strings.ToLower(prompt)

	isMultiPart := strings.Contains(promptLower, " and ") ||
		strings.Contains(promptLower, "compare") ||
		strings.Contains(promptLower, "difference between") ||
		strings.Contains(promptLower, " vs ") ||
		strings.Contains(promptLower, "versus")

	isProcedural := strings.Contains(promptLower, "steps to") ||
		strings.Contains(promptLower, "procedure for") ||
		strings.Contains(promptLower, "how to file") ||
		strings.Contains(promptLower, "process of")

	isNegation := strings.Contains(promptLower, "except") ||
		strings.Contains(promptLower, "unless") ||
		strings.Contains(promptLower, "not apply") ||
		strings.Contains(promptLower, "exempted")

	if isMultiPart {
		return r.complex, fmt.Sprintf("claude:complex:multi_part:tokens=%d", tokens)
	}
	if isProcedural {
		return r.complex, fmt.Sprintf("claude:complex:procedural:tokens=%d", tokens)
	}
	if isNegation {
		return r.complex, fmt.Sprintf("claude:complex:negation:tokens=%d", tokens)
	}
	if tokens >= r.threshold {
		return r.complex, fmt.Sprintf("claude:complex:token_threshold:tokens=%d", tokens)
	}

	// Simple signals — Gemini only when tokens < threshold AND one of these matches.
	isSectionCite := sectionCiteRE.MatchString(prompt)

	isDefinition := strings.HasPrefix(promptLower, "what is") ||
		strings.HasPrefix(promptLower, "define") ||
		strings.HasPrefix(promptLower, "meaning of") ||
		strings.HasPrefix(promptLower, "explain")

	if isSectionCite {
		return r.simple, fmt.Sprintf("gemini:simple:section_cite:tokens=%d", tokens)
	}
	if isDefinition {
		return r.simple, fmt.Sprintf("gemini:simple:definition:tokens=%d", tokens)
	}

	// Default: no Gemini signal matched → Claude.
	return r.complex, fmt.Sprintf("claude:complex:default:tokens=%d", tokens)
}

// estimateTokens estimates token count from request content.
// Heuristic: ~4 chars per token.
func estimateTokens(req CompletionRequest) int {
	total := len(req.Prompt) + len(req.SystemPrompt)
	for _, m := range req.Messages {
		total += len(m.Content)
	}
	return total / 4
}

// Name returns the provider identifier.
func (r *Router) Name() string { return "router" }

// Complete routes the request to the chosen provider and delegates.
func (r *Router) Complete(ctx context.Context, req CompletionRequest) (CompletionResponse, error) {
	provider, reason := r.route(req)
	log.Printf("ROUTER: routing_reason=%s provider=%s", reason, provider.Name())
	return provider.Complete(ctx, req)
}

// Stream routes the request to the chosen provider, logs the routing reason, and streams.
func (r *Router) Stream(ctx context.Context, req CompletionRequest) (<-chan StreamChunk, error) {
	provider, reason := r.route(req)
	log.Printf("ROUTER: routing_reason=%s provider=%s", reason, provider.Name())
	return provider.Stream(ctx, req)
}

// Embed always uses the simple provider — embedding doesn't need complex routing.
func (r *Router) Embed(ctx context.Context, texts []string) ([][]float32, error) {
	return r.simple.Embed(ctx, texts)
}

// HealthCheck verifies both providers are reachable. Returns first error encountered.
func (r *Router) HealthCheck(ctx context.Context) error {
	if err := r.simple.HealthCheck(ctx); err != nil {
		return fmt.Errorf("router: simple provider (%s) health check: %w", r.simple.Name(), err)
	}
	if err := r.complex.HealthCheck(ctx); err != nil {
		return fmt.Errorf("router: complex provider (%s) health check: %w", r.complex.Name(), err)
	}
	return nil
}
