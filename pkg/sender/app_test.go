package sender

import (
	"context"
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

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	// Start the app in a goroutine
	done := make(chan error, 1)
	go func() {
		done <- app.Run(ctx)
	}()

	// Start a transfer process with a short timeout to avoid long waits
	receiver := discovery.ServiceInfo{
		Name: "test-receiver",
		Addr: nil, // This will cause the transfer to fail quickly
		Port: 8080,
	}
	files := []fileInfo.FileNode{}

	// Use a context with short timeout for the transfer
	transferCtx, transferCancel := context.WithTimeout(ctx, 100*time.Millisecond)
	defer transferCancel()

	app.StartSendProcess(transferCtx, receiver, files)

	// Give the transfer goroutine a moment to start and fail
	time.Sleep(200 * time.Millisecond)

	// Cancel the main context to trigger shutdown
	cancel()

	// Wait for the app to shut down gracefully
	select {
	case err := <-done:
		if err != nil && err != context.Canceled && err != context.DeadlineExceeded {
			t.Errorf("Expected context.Canceled, context.DeadlineExceeded or nil, got: %v", err)
		}
	case <-time.After(3 * time.Second):
		t.Error("App did not shut down within 3 seconds")
	}
}

func TestTransferWaitGroup(t *testing.T) {
	app := NewApp(&MockDiscoveryAdapter{})

	// Verify that transferWG is properly initialized (sync.WaitGroup zero value is valid)
	// We can't directly compare sync.WaitGroup, so we'll test its functionality instead
	t.Log("transferWG should be initialized")

	// Test that StartSendProcess properly manages the WaitGroup
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	receiver := discovery.ServiceInfo{
		Name: "test-receiver",
		Addr: nil,
		Port: 8080,
	}
	files := []fileInfo.FileNode{}

	// This should add to the WaitGroup
	app.StartSendProcess(ctx, receiver, files)

	// Give the goroutine a moment to start and fail
	time.Sleep(200 * time.Millisecond)

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
