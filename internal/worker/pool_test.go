package worker

import (
	"context"
	"testing"
	"time"
)

// TestPool_AtCapacity_CancelledCtx verifies that Acquire returns immediately
// when the context is cancelled and the pool is at capacity.
func TestPool_AtCapacity_CancelledCtx(t *testing.T) {
	pool := NewPool(2)

	// Fill pool to capacity
	ctx := context.Background()
	if err := pool.Acquire(ctx); err != nil {
		t.Fatalf("first Acquire failed: %v", err)
	}
	if err := pool.Acquire(ctx); err != nil {
		t.Fatalf("second Acquire failed: %v", err)
	}

	if pool.ActiveWorkers() != 2 {
		t.Fatalf("expected 2 active workers, got %d", pool.ActiveWorkers())
	}

	// Cancelled context should return immediately
	cancelledCtx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately

	start := time.Now()
	err := pool.Acquire(cancelledCtx)
	elapsed := time.Since(start)

	if err == nil {
		t.Fatal("expected error from cancelled context, got nil")
	}
	if err != context.Canceled {
		t.Fatalf("expected context.Canceled, got %v", err)
	}
	if elapsed > 100*time.Millisecond {
		t.Fatalf("Acquire took too long on cancelled context: %v", elapsed)
	}
}

// TestPool_ReleaseRestoresCapacity verifies that releasing a slot allows
// a subsequent Acquire to succeed.
func TestPool_ReleaseRestoresCapacity(t *testing.T) {
	pool := NewPool(1)
	ctx := context.Background()

	// First acquire should succeed
	if err := pool.Acquire(ctx); err != nil {
		t.Fatalf("first Acquire failed: %v", err)
	}
	if pool.ActiveWorkers() != 1 {
		t.Fatalf("expected 1 active worker, got %d", pool.ActiveWorkers())
	}

	// Release
	pool.Release()
	if pool.ActiveWorkers() != 0 {
		t.Fatalf("expected 0 active workers after release, got %d", pool.ActiveWorkers())
	}

	// Second acquire should succeed immediately
	if err := pool.Acquire(ctx); err != nil {
		t.Fatalf("second Acquire after release failed: %v", err)
	}
	pool.Release()
}
