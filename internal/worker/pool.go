package worker

import "context"

// Pool is a semaphore-based bounded worker pool.
// It limits the number of concurrent operations to prevent resource exhaustion.
type Pool struct {
	sem  chan struct{}
	size int
}

// NewPool creates a worker pool with the given maximum concurrency.
func NewPool(size int) *Pool {
	sem := make(chan struct{}, size)
	for i := 0; i < size; i++ {
		sem <- struct{}{}
	}
	return &Pool{sem: sem, size: size}
}

// Acquire blocks until a slot is available or ctx is cancelled.
// Returns ctx.Err() if the context is cancelled before a slot is acquired.
func (p *Pool) Acquire(ctx context.Context) error {
	select {
	case <-p.sem:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// Release returns a slot to the pool. Must be called after each successful Acquire.
func (p *Pool) Release() {
	p.sem <- struct{}{}
}

// ActiveWorkers returns the number of slots currently in use.
func (p *Pool) ActiveWorkers() int {
	return p.size - len(p.sem)
}

// Capacity returns the maximum number of concurrent workers.
func (p *Pool) Capacity() int {
	return p.size
}
