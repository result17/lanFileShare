package sender

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/rescp17/lanFileSharer/pkg/discovery"
	"github.com/rescp17/lanFileSharer/pkg/fileInfo"
)

// MockDiscoveryAdapter for testing
type MockDiscoveryAdapter struct{}

func (m *MockDiscoveryAdapter) Announce(ctx context.Context, service discovery.ServiceInfo) error {
	return nil // Not used in sender tests
}

func (m *MockDiscoveryAdapter) Discover(ctx context.Context, service string) (<-chan []discovery.ServiceInfo, error) {
	ch := make(chan []discovery.ServiceInfo)
	go func() {
		defer close(ch)
		<-ctx.Done() // Wait for context cancellation
	}()
	return ch, nil
}

func TestGracefulShutdown(t *testing.T) {
	app := NewApp(&MockDiscoveryAdapter{})

	ctx, cancel := context.WithCancel(context.Background())

	// Start the app in a goroutine
	done := make(chan error, 1)
	go func() {
		done <- app.Run(ctx)
	}()

	// Start a transfer process
	receiver := discovery.ServiceInfo{
		Name: "test-receiver",
		Addr: nil, // This will cause the transfer to fail, but that's ok for testing
		Port: 8080,
	}
	files := []fileInfo.FileNode{}

	app.StartSendProcess(ctx, receiver, files)

	// Give the transfer goroutine a moment to start
	time.Sleep(10 * time.Millisecond)

	// Cancel the context to trigger shutdown
	cancel()

	// Wait for the app to shut down gracefully
	select {
	case err := <-done:
		if err != nil && err != context.Canceled {
			t.Errorf("Expected context.Canceled or nil, got: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Error("App did not shut down within 5 seconds")
	}
}

func TestTransferWaitGroup(t *testing.T) {
	app := NewApp(&MockDiscoveryAdapter{})

	// Verify that transferWG is properly initialized
	if app.transferWG == (sync.WaitGroup{}) {
		t.Error("transferWG should be initialized")
	}

	// Test that StartSendProcess properly manages the WaitGroup
	ctx := context.Background()
	receiver := discovery.ServiceInfo{
		Name: "test-receiver",
		Addr: nil,
		Port: 8080,
	}
	files := []fileInfo.FileNode{}

	// This should add to the WaitGroup
	app.StartSendProcess(ctx, receiver, files)

	// Give the goroutine a moment to start and fail
	time.Sleep(100 * time.Millisecond)

	// The WaitGroup should be back to 0 after the goroutine completes
	// We can't directly test this, but we can ensure Wait() doesn't block
	done := make(chan struct{})
	go func() {
		app.transferWG.Wait()
		close(done)
	}()

	select {
	case <-done:
		// Good, Wait() didn't block
	case <-time.After(1 * time.Second):
		t.Error("transferWG.Wait() blocked, indicating goroutine didn't complete properly")
	}
}
