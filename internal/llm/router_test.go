package llm_test

import (
	"context"
	"testing"
	"time"

	"testqdrant/internal/llm"
)

// mockProvider is a minimal stub that records calls and returns fixed values.
type mockProvider struct {
	name string
	// calls records which methods were called, for routing assertions.
	completeCalls int
	streamCalls   int
	embedCalls    int
}

func (m *mockProvider) Complete(_ context.Context, _ llm.CompletionRequest) (llm.CompletionResponse, error) {
	m.completeCalls++
	return llm.CompletionResponse{Content: "mock response", ModelUsed: m.name}, nil
}

func (m *mockProvider) Stream(_ context.Context, _ llm.CompletionRequest) (<-chan llm.StreamChunk, error) {
	m.streamCalls++
	ch := make(chan llm.StreamChunk, 1)
	ch <- llm.StreamChunk{Done: true}
	close(ch)
	return ch, nil
}

func (m *mockProvider) Embed(_ context.Context, texts []string) ([][]float32, error) {
	m.embedCalls++
	result := make([][]float32, len(texts))
	for i := range result {
		result[i] = []float32{0.1, 0.2, 0.3}
	}
	return result, nil
}

func (m *mockProvider) Name() string { return m.name }

func (m *mockProvider) HealthCheck(_ context.Context) error { return nil }

// TestRouter_CompileTimeCheck verifies Router satisfies LLMProvider at compile time.
func TestRouter_CompileTimeCheck(t *testing.T) {
	var _ llm.LLMProvider = (*llm.Router)(nil)
}

// TestRouter_Name verifies the router returns "router" as its name.
func TestRouter_Name(t *testing.T) {
	simple := &mockProvider{name: "gemini"}
	complex := &mockProvider{name: "claude"}
	r := llm.NewRouter(simple, complex, 500)
	if r.Name() != "router" {
		t.Errorf("Name() = %q, want %q", r.Name(), "router")
	}
}

// TestRouter_SectionCite verifies section cite queries route to the simple (Gemini) provider.
func TestRouter_SectionCite(t *testing.T) {
	simple := &mockProvider{name: "gemini"}
	complex := &mockProvider{name: "claude"}
	r := llm.NewRouter(simple, complex, 500)

	tests := []struct {
		name  string
		query string
	}{
		{"section number", "What does Section 103 of BNS say?"},
		{"sec abbreviation", "Explain sec. 4 of the Constitution"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			simple.completeCalls = 0
			complex.completeCalls = 0

			_, err := r.Complete(context.Background(), llm.CompletionRequest{Prompt: tc.query})
			if err != nil {
				t.Fatalf("Complete() error: %v", err)
			}
			if simple.completeCalls != 1 {
				t.Errorf("expected simple provider to be called, simple.completeCalls=%d complex.completeCalls=%d",
					simple.completeCalls, complex.completeCalls)
			}
			if complex.completeCalls != 0 {
				t.Errorf("expected complex provider NOT to be called, complex.completeCalls=%d", complex.completeCalls)
			}
		})
	}
}

// TestRouter_CompareQuery verifies compare queries route to the complex (Claude) provider.
func TestRouter_CompareQuery(t *testing.T) {
	simple := &mockProvider{name: "gemini"}
	complex := &mockProvider{name: "claude"}
	r := llm.NewRouter(simple, complex, 500)

	tests := []struct {
		name  string
		query string
	}{
		{"compare keyword", "Compare BNS and IPC on theft"},
		{"vs keyword", "BNS vs IPC: what changed?"},
		{"difference between", "What is the difference between Section 302 IPC and BNS equivalent?"},
		{"versus", "IPC versus BNS on murder"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			simple.completeCalls = 0
			complex.completeCalls = 0

			_, err := r.Complete(context.Background(), llm.CompletionRequest{Prompt: tc.query})
			if err != nil {
				t.Fatalf("Complete() error: %v", err)
			}
			if complex.completeCalls != 1 {
				t.Errorf("expected complex provider to be called, complex.completeCalls=%d simple.completeCalls=%d",
					complex.completeCalls, simple.completeCalls)
			}
		})
	}
}

// TestRouter_DefaultsToComplex verifies that queries with no Gemini signal default to Claude.
func TestRouter_DefaultsToComplex(t *testing.T) {
	simple := &mockProvider{name: "gemini"}
	complex := &mockProvider{name: "claude"}
	r := llm.NewRouter(simple, complex, 500)

	// This query has no Gemini signals (no section cite, no definition prefix)
	// and no complex signals — so defaults to Claude.
	query := "Tell me about fundamental rights"

	_, err := r.Complete(context.Background(), llm.CompletionRequest{Prompt: query})
	if err != nil {
		t.Fatalf("Complete() error: %v", err)
	}
	if complex.completeCalls != 1 {
		t.Errorf("expected complex (Claude) as default, complex.completeCalls=%d simple.completeCalls=%d",
			complex.completeCalls, simple.completeCalls)
	}
}

// TestRouter_MultiPart verifies multi-part queries route to Claude.
func TestRouter_MultiPart(t *testing.T) {
	simple := &mockProvider{name: "gemini"}
	complex := &mockProvider{name: "claude"}
	r := llm.NewRouter(simple, complex, 500)

	query := "What is Section 302 IPC and how does it map to BNS?"

	_, err := r.Complete(context.Background(), llm.CompletionRequest{Prompt: query})
	if err != nil {
		t.Fatalf("Complete() error: %v", err)
	}
	if complex.completeCalls != 1 {
		t.Errorf("expected complex provider for multi-part query, complex.completeCalls=%d", complex.completeCalls)
	}
}

// TestRouter_TokenThreshold verifies that a large prompt (>= threshold tokens) routes to Claude.
func TestRouter_TokenThreshold(t *testing.T) {
	simple := &mockProvider{name: "gemini"}
	complex := &mockProvider{name: "claude"}
	threshold := 50 // low threshold so we can test with a moderate-length prompt

	r := llm.NewRouter(simple, complex, threshold)

	// Build a prompt that results in > threshold tokens (~4 chars/token, so > 200 chars).
	// This is a definition query BUT the token count overrides the simple signal.
	longPrompt := "explain " + repeatStr("what is the meaning of fundamental rights in India ", 5)

	_, err := r.Complete(context.Background(), llm.CompletionRequest{Prompt: longPrompt})
	if err != nil {
		t.Fatalf("Complete() error: %v", err)
	}
	if complex.completeCalls != 1 {
		t.Errorf("expected complex provider for token threshold breach, complex.completeCalls=%d simple.completeCalls=%d",
			complex.completeCalls, simple.completeCalls)
	}
}

// TestRouter_DefinitionQuery verifies definition queries route to simple (Gemini) provider.
func TestRouter_DefinitionQuery(t *testing.T) {
	simple := &mockProvider{name: "gemini"}
	complex := &mockProvider{name: "claude"}
	r := llm.NewRouter(simple, complex, 500)

	tests := []struct {
		name  string
		query string
	}{
		{"what is", "what is fundamental right?"},
		{"define", "define habeas corpus"},
		{"meaning of", "meaning of cognizable offence"},
		{"explain", "explain the concept of bail"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			simple.completeCalls = 0
			complex.completeCalls = 0

			_, err := r.Complete(context.Background(), llm.CompletionRequest{Prompt: tc.query})
			if err != nil {
				t.Fatalf("Complete() error: %v", err)
			}
			if simple.completeCalls != 1 {
				t.Errorf("expected simple (Gemini) for definition query %q, simple.completeCalls=%d complex.completeCalls=%d",
					tc.query, simple.completeCalls, complex.completeCalls)
			}
		})
	}
}

// TestRouter_EmbedUsesSimple verifies Embed always delegates to the simple provider.
func TestRouter_EmbedUsesSimple(t *testing.T) {
	simple := &mockProvider{name: "gemini"}
	complex := &mockProvider{name: "claude"}
	r := llm.NewRouter(simple, complex, 500)

	_, err := r.Embed(context.Background(), []string{"some text"})
	if err != nil {
		t.Fatalf("Embed() error: %v", err)
	}
	if simple.embedCalls != 1 {
		t.Errorf("expected simple provider for Embed, simple.embedCalls=%d", simple.embedCalls)
	}
	if complex.embedCalls != 0 {
		t.Errorf("complex provider should not be called for Embed, complex.embedCalls=%d", complex.embedCalls)
	}
}

// TestRouter_StreamLogsReason verifies Stream delegates without error.
func TestRouter_StreamLogsReason(t *testing.T) {
	simple := &mockProvider{name: "gemini"}
	complex := &mockProvider{name: "claude"}
	r := llm.NewRouter(simple, complex, 500)

	// Section cite → Gemini
	ch, err := r.Stream(context.Background(), llm.CompletionRequest{Prompt: "What is Section 10?"})
	if err != nil {
		t.Fatalf("Stream() error: %v", err)
	}
	// Drain channel
	for range ch {
	}
	if simple.streamCalls != 1 {
		t.Errorf("expected simple provider for section cite stream, simple.streamCalls=%d", simple.streamCalls)
	}
}

// TestRouter_HealthCheck verifies both providers are checked.
func TestRouter_HealthCheck(t *testing.T) {
	simple := &mockProvider{name: "gemini"}
	complex := &mockProvider{name: "claude"}
	r := llm.NewRouter(simple, complex, 500)

	if err := r.HealthCheck(context.Background()); err != nil {
		t.Errorf("HealthCheck() error: %v", err)
	}
}

// BenchmarkRouter_route benchmarks the routing decision (target < 5ms).
func BenchmarkRouter_route(b *testing.B) {
	simple := &mockProvider{name: "gemini"}
	complex := &mockProvider{name: "claude"}
	r := llm.NewRouter(simple, complex, 500)

	req := llm.CompletionRequest{
		Prompt:       "What does Section 302 BNS say about murder?",
		SystemPrompt: "You are a legal assistant.",
	}

	b.ResetTimer()
	start := time.Now()
	for i := 0; i < b.N; i++ {
		_, err := r.Complete(context.Background(), req)
		if err != nil {
			b.Fatalf("Complete() error: %v", err)
		}
	}
	elapsed := time.Since(start)
	if b.N > 0 {
		perOp := elapsed / time.Duration(b.N)
		b.Logf("BenchmarkRouter_route: %d ns/op (%.3f ms/op)", perOp.Nanoseconds(), float64(perOp.Nanoseconds())/1e6)
	}
}

// repeatStr is a test helper to repeat a string n times.
func repeatStr(s string, n int) string {
	result := ""
	for i := 0; i < n; i++ {
		result += s
	}
	return result
}
