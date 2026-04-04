package llm

import (
	"context"
	"log"
	"time"

	"github.com/sony/gobreaker"
)

// BreakerProvider wraps any LLMProvider with a circuit breaker.
// Settings: 5 consecutive failures → open; 30s half-open timeout; 2 successes → close.
type BreakerProvider struct {
	inner   LLMProvider
	breaker *gobreaker.CircuitBreaker
}

// Compile-time check: BreakerProvider must implement LLMProvider.
var _ LLMProvider = (*BreakerProvider)(nil)

// NewBreakerProvider wraps an LLMProvider with a circuit breaker.
func NewBreakerProvider(inner LLMProvider) *BreakerProvider {
	settings := gobreaker.Settings{
		Name:        inner.Name(),
		MaxRequests: 2,                // half-open: 2 successes → close
		Interval:    0,                // no periodic reset of counts in closed state
		Timeout:     30 * time.Second, // half-open timeout
		ReadyToTrip: func(counts gobreaker.Counts) bool {
			return counts.ConsecutiveFailures >= 5
		},
		OnStateChange: func(name string, from, to gobreaker.State) {
			log.Printf("CIRCUIT: provider=%s state=%s", name, to.String())
		},
	}
	return &BreakerProvider{
		inner:   inner,
		breaker: gobreaker.NewCircuitBreaker(settings),
	}
}

// Complete calls the inner provider's Complete method through the circuit breaker.
func (b *BreakerProvider) Complete(ctx context.Context, req CompletionRequest) (CompletionResponse, error) {
	result, err := b.breaker.Execute(func() (interface{}, error) {
		return b.inner.Complete(ctx, req)
	})
	if err != nil {
		return CompletionResponse{}, err
	}
	return result.(CompletionResponse), nil
}

// Stream calls the inner provider's Stream method through the circuit breaker.
func (b *BreakerProvider) Stream(ctx context.Context, req CompletionRequest) (<-chan StreamChunk, error) {
	result, err := b.breaker.Execute(func() (interface{}, error) {
		return b.inner.Stream(ctx, req)
	})
	if err != nil {
		return nil, err
	}
	return result.(<-chan StreamChunk), nil
}

// Embed calls the inner provider's Embed method through the circuit breaker.
func (b *BreakerProvider) Embed(ctx context.Context, texts []string) ([][]float32, error) {
	result, err := b.breaker.Execute(func() (interface{}, error) {
		return b.inner.Embed(ctx, texts)
	})
	if err != nil {
		return nil, err
	}
	return result.([][]float32), nil
}

// Name returns the inner provider's name.
func (b *BreakerProvider) Name() string {
	return b.inner.Name()
}

// HealthCheck calls the inner provider's HealthCheck through the circuit breaker.
func (b *BreakerProvider) HealthCheck(ctx context.Context) error {
	_, err := b.breaker.Execute(func() (interface{}, error) {
		return nil, b.inner.HealthCheck(ctx)
	})
	return err
}
