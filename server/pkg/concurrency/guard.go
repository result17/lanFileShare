package concurrency

import (
	"errors"
	"sync"
)

var ErrBusy = errors.New("System is busy!")

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
