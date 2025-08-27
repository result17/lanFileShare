package transfer

import (
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/rescp17/lanFileSharer/pkg/fileInfo"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestListener implements StatusListener for testing
type TestListener struct {
	id                     string
	fileStatusCallCount    int64
	sessionStatusCallCount int64
	blockDuration          time.Duration
	panicOnCall            bool
	mu                     sync.Mutex
	lastFileEvent          *FileStatusEvent
	lastSessionEvent       *SessionStatusEvent
}

type FileStatusEvent struct {
	FilePath  string
	OldStatus *TransferStatus
	NewStatus *TransferStatus
	Timestamp time.Time
}

type SessionStatusEvent struct {
	OldStatus *SessionTransferStatus
	NewStatus *SessionTransferStatus
	Timestamp time.Time
}

func NewTestListener(id string) *TestListener {
	return &TestListener{
		id: id,
	}
}

func (tl *TestListener) ID() string {
	return tl.id
}

func (tl *TestListener) SetBlockDuration(duration time.Duration) {
	tl.blockDuration = duration
}

func (tl *TestListener) SetPanicOnCall(panic bool) {
	tl.panicOnCall = panic
}

func (tl *TestListener) OnFileStatusChanged(filePath string, oldStatus, newStatus *TransferStatus) {
	if tl.panicOnCall {
		panic(fmt.Sprintf("Test panic in listener %s", tl.id))
	}

	if tl.blockDuration > 0 {
		time.Sleep(tl.blockDuration)
	}

	atomic.AddInt64(&tl.fileStatusCallCount, 1)

	tl.mu.Lock()
	tl.lastFileEvent = &FileStatusEvent{
		FilePath:  filePath,
		OldStatus: oldStatus,
		NewStatus: newStatus,
		Timestamp: time.Now(),
	}
	tl.mu.Unlock()
}

func (tl *TestListener) OnSessionStatusChanged(oldStatus, newStatus *SessionTransferStatus) {
	if tl.panicOnCall {
		panic(fmt.Sprintf("Test panic in listener %s", tl.id))
	}

	if tl.blockDuration > 0 {
		time.Sleep(tl.blockDuration)
	}

	atomic.AddInt64(&tl.sessionStatusCallCount, 1)

	tl.mu.Lock()
	tl.lastSessionEvent = &SessionStatusEvent{
		OldStatus: oldStatus,
		NewStatus: newStatus,
		Timestamp: time.Now(),
	}
	tl.mu.Unlock()
}

func (tl *TestListener) GetFileStatusCallCount() int64 {
	return atomic.LoadInt64(&tl.fileStatusCallCount)
}

func (tl *TestListener) GetSessionStatusCallCount() int64 {
	return atomic.LoadInt64(&tl.sessionStatusCallCount)
}

func (tl *TestListener) GetLastFileEvent() *FileStatusEvent {
	tl.mu.Lock()
	defer tl.mu.Unlock()
	return tl.lastFileEvent
}

func (tl *TestListener) GetLastSessionEvent() *SessionStatusEvent {
	tl.mu.Lock()
	defer tl.mu.Unlock()
	return tl.lastSessionEvent
}

// TestListenerConcurrency tests that listeners are called concurrently and don't block each other
func TestListenerConcurrency(t *testing.T) {
	t.Run("listeners_run_concurrently", func(t *testing.T) {
		manager := NewUnifiedTransferManager("concurrency-test")
		defer manager.Close()

		// Create test listeners with different blocking durations
		fastListener := NewTestListener("fast")
		slowListener := NewTestListener("slow")
		slowListener.SetBlockDuration(500 * time.Millisecond) // Slow listener

		// Add listeners
		manager.AddStatusListener(fastListener)
		manager.AddStatusListener(slowListener)

		// Create and add a test file
		testContent := []byte("Test content for listener concurrency")
		testFile, cleanup := createFileNodeFromTempFile(t, testContent)
		defer cleanup()

		err := manager.AddFile(testFile)
		require.NoError(t, err)

		// Start transfer to trigger notifications
		start := time.Now()
		err = manager.StartTransfer(testFile.Path)
		require.NoError(t, err)

		// The notification should return quickly even though slow listener blocks
		elapsed := time.Since(start)
		assert.Less(t, elapsed, 100*time.Millisecond,
			"StartTransfer should not be blocked by slow listeners")

		// Wait for both listeners to be called
		assert.Eventually(t, func() bool {
			return fastListener.GetFileStatusCallCount() > 0 &&
				slowListener.GetFileStatusCallCount() > 0
		}, 1*time.Second, 10*time.Millisecond, "Both listeners should be called")

		// Fast listener should complete quickly
		assert.Eventually(t, func() bool {
			return fastListener.GetFileStatusCallCount() > 0
		}, 50*time.Millisecond, 5*time.Millisecond, "Fast listener should complete quickly")
	})

	t.Run("panic_in_listener_does_not_crash_system", func(t *testing.T) {
		manager := NewUnifiedTransferManager("panic-test")
		defer manager.Close()

		// Create listeners: one that panics, one normal
		panicListener := NewTestListener("panic")
		panicListener.SetPanicOnCall(true)
		normalListener := NewTestListener("normal")

		manager.AddStatusListener(panicListener)
		manager.AddStatusListener(normalListener)

		// Create and add a test file
		testContent := []byte("Test content for panic handling")
		testFile, cleanup := createFileNodeFromTempFile(t, testContent)
		defer cleanup()

		err := manager.AddFile(testFile)
		require.NoError(t, err)

		// Start transfer - should not crash despite panic in listener
		err = manager.StartTransfer(testFile.Path)
		require.NoError(t, err)

		// Normal listener should still be called despite panic in other listener
		assert.Eventually(t, func() bool {
			return normalListener.GetFileStatusCallCount() > 0
		}, 1*time.Second, 10*time.Millisecond, "Normal listener should be called despite panic in other listener")
	})

	t.Run("multiple_rapid_notifications", func(t *testing.T) {
		manager := NewUnifiedTransferManager("rapid-test")
		defer manager.Close()

		// Create multiple listeners
		listeners := make([]*TestListener, 5)
		for i := 0; i < 5; i++ {
			listeners[i] = NewTestListener(fmt.Sprintf("listener-%d", i))
			manager.AddStatusListener(listeners[i])
		}

		// Create and add multiple test files
		numFiles := 3
		var testFiles []*fileInfo.FileNode
		var cleanupFuncs []func()

		defer func() {
			for _, cleanup := range cleanupFuncs {
				cleanup()
			}
		}()

		for i := 0; i < numFiles; i++ {
			testContent := []byte(fmt.Sprintf("Test content for file %d", i))
			testFile, cleanup := createFileNodeFromTempFile(t, testContent)
			testFiles = append(testFiles, testFile)
			cleanupFuncs = append(cleanupFuncs, cleanup)

			err := manager.AddFile(testFile)
			require.NoError(t, err)
		}

		// Rapidly start and complete transfers
		for _, testFile := range testFiles {
			err := manager.StartTransfer(testFile.Path)
			require.NoError(t, err)

			err = manager.CompleteTransfer(testFile.Path)
			require.NoError(t, err)
		}

		// All listeners should receive all notifications
		expectedCalls := int64(numFiles * 2) // Start + Complete for each file
		for i, listener := range listeners {
			assert.Eventually(t, func() bool {
				return listener.GetFileStatusCallCount() >= expectedCalls
			}, 2*time.Second, 10*time.Millisecond,
				"Listener %d should receive all notifications", i)
		}
	})
}

// TestListenerLockMinimization tests that lock holding time is minimized
func TestListenerLockMinimization(t *testing.T) {
	t.Run("lock_holding_time_is_minimal", func(t *testing.T) {
		manager := NewUnifiedTransferManager("lock-test")
		defer manager.Close()

		// Create a slow listener
		slowListener := NewTestListener("slow")
		slowListener.SetBlockDuration(200 * time.Millisecond)
		manager.AddStatusListener(slowListener)

		// Create test file
		testContent := []byte("Test content for lock timing")
		testFile, cleanup := createFileNodeFromTempFile(t, testContent)
		defer cleanup()

		err := manager.AddFile(testFile)
		require.NoError(t, err)

		// Measure time to add another listener while notification is happening
		err = manager.StartTransfer(testFile.Path)
		require.NoError(t, err)

		// Immediately try to add another listener - this should not block
		start := time.Now()
		fastListener := NewTestListener("fast")
		manager.AddStatusListener(fastListener)
		elapsed := time.Since(start)

		// Adding listener should be fast even if notification is ongoing
		assert.Less(t, elapsed, 50*time.Millisecond,
			"Adding listener should not be blocked by ongoing notifications")
	})
}
