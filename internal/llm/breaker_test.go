package llm_test

import (
	"context"
	"errors"
	"testing"

	"github.com/sony/gobreaker"

	"testqdrant/internal/llm"
)

// Compile-time check: BreakerProvider implements LLMProvider.
var _ llm.LLMProvider = (*llm.BreakerProvider)(nil)

// errProvider is a mock LLMProvider that always returns an error.
type errProvider struct{}

func (e *errProvider) Complete(_ context.Context, _ llm.CompletionRequest) (llm.CompletionResponse, error) {
	return llm.CompletionResponse{}, errors.New("provider down")
}

func (e *errProvider) Stream(_ context.Context, _ llm.CompletionRequest) (<-chan llm.StreamChunk, error) {
	return nil, errors.New("provider down")
}

func (e *errProvider) Embed(_ context.Context, _ []string) ([][]float32, error) {
	return nil, errors.New("provider down")
}

func (e *errProvider) Name() string { return "mock-error" }

func (e *errProvider) HealthCheck(_ context.Context) error {
	return errors.New("provider down")
}

// TestBreakerProvider_OpensAfterFiveFailures verifies that after 5 consecutive
// failures the circuit breaker opens and the 6th call returns ErrOpenState.
func TestBreakerProvider_OpensAfterFiveFailures(t *testing.T) {
	bp := llm.NewBreakerProvider(&errProvider{})
	ctx := context.Background()

	// First 5 calls: expect underlying errors
	for i := 0; i < 5; i++ {
		_, err := bp.Complete(ctx, llm.CompletionRequest{Prompt: "test"})
		if err == nil {
			t.Fatalf("call %d: expected error, got nil", i+1)
		}
		if errors.Is(err, gobreaker.ErrOpenState) {
			t.Fatalf("call %d: circuit opened too early", i+1)
		}
	}

	// 6th call: circuit should be open
	_, err := bp.Complete(ctx, llm.CompletionRequest{Prompt: "test"})
	if err == nil {
		t.Fatal("6th call: expected ErrOpenState, got nil")
	}
	if !errors.Is(err, gobreaker.ErrOpenState) {
		t.Fatalf("6th call: expected ErrOpenState, got %v", err)
	}
}
