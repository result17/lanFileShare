package concurrency

import (
	"context"
	"errors"
	"sync"
)

var ErrBusy = errors.New("system is busy")

type ConcurrencyGuard struct {
	mu     sync.Mutex
	isBusy bool
}

func NewConcurrencyGuard() *ConcurrencyGuard {
	return &ConcurrencyGuard{}
}

func (g *ConcurrencyGuard) Execute(task func() error) error {
	g.mu.Lock()
	if g.isBusy {
		g.mu.Unlock()
		return ErrBusy
	}
	g.isBusy = true
	g.mu.Unlock()
	defer func() {
		g.mu.Lock()
		g.isBusy = false
		g.mu.Unlock()
	}()
	return task()
}

// ExecuteWithContext executes a task with context support for graceful cancellation
func (g *ConcurrencyGuard) ExecuteWithContext(ctx context.Context, task func(context.Context) error) error {
	g.mu.Lock()
	if g.isBusy {
		g.mu.Unlock()
		return ErrBusy
	}
	g.isBusy = true
	g.mu.Unlock()
	defer func() {
		g.mu.Lock()
		g.isBusy = false
		g.mu.Unlock()
	}()

	// Check if context is already cancelled before starting
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	return task(ctx)
}
