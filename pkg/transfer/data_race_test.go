package transfer

import (
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestStatusListener is a test implementation of StatusListener
type TestStatusListener struct {
	id                   string
	mu                   sync.Mutex
	fileStatusChanges    []FileStatusChange
	sessionStatusChanges []SessionStatusChange
}

type FileStatusChange struct {
	FilePath  string
	OldStatus *TransferStatus
	NewStatus *TransferStatus
}

type SessionStatusChange struct {
	OldStatus *SessionTransferStatus
	NewStatus *SessionTransferStatus
}

// NewTestStatusListener creates a new test status listener with a unique UUID
func NewTestStatusListener() *TestStatusListener {
	return &TestStatusListener{
		id: uuid.New().String(),
	}
}

// ID returns the unique identifier for this listener
func (tsl *TestStatusListener) ID() string {
	return tsl.id
}

func (tsl *TestStatusListener) OnFileStatusChanged(filePath string, oldStatus, newStatus *TransferStatus) {
	tsl.mu.Lock()
	defer tsl.mu.Unlock()

	// Simulate some processing time to increase chance of race condition
	time.Sleep(1 * time.Millisecond)

	tsl.fileStatusChanges = append(tsl.fileStatusChanges, FileStatusChange{
		FilePath:  filePath,
		OldStatus: oldStatus,
		NewStatus: newStatus,
	})
}

func (tsl *TestStatusListener) OnSessionStatusChanged(oldStatus, newStatus *SessionTransferStatus) {
	tsl.mu.Lock()
	defer tsl.mu.Unlock()

	// Simulate some processing time to increase chance of race condition
	time.Sleep(1 * time.Millisecond)

	tsl.sessionStatusChanges = append(tsl.sessionStatusChanges, SessionStatusChange{
		OldStatus: oldStatus,
		NewStatus: newStatus,
	})
}

func (tsl *TestStatusListener) GetFileStatusChanges() []FileStatusChange {
	tsl.mu.Lock()
	defer tsl.mu.Unlock()
	return append([]FileStatusChange(nil), tsl.fileStatusChanges...)
}

func (tsl *TestStatusListener) GetSessionStatusChanges() []SessionStatusChange {
	tsl.mu.Lock()
	defer tsl.mu.Unlock()
	return append([]SessionStatusChange(nil), tsl.sessionStatusChanges...)
}

// TestDataRaceFixed verifies that the data race in status notifications has been fixed
func TestDataRaceFixed(t *testing.T) {
	// This test should be run with -race flag to detect data races
	tsm := NewTransferStatusManager()

	// Add a test listener
	listener := NewTestStatusListener()
	tsm.AddStatusListener(listener)

	// Initialize session
	err := tsm.InitializeSession("test-session", 3, 3000)
	require.NoError(t, err)

	// Start multiple file transfers rapidly to trigger potential race conditions
	var wg sync.WaitGroup

	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(fileIndex int) {
			defer wg.Done()

			// Start file transfer
			filePath := fmt.Sprintf("/test/file%d.txt", fileIndex)
			_, err := tsm.StartFileTransfer(filePath, 1000)
			if err != nil {
				return // Some operations may fail due to concurrent access, that's OK
			}

			// Update progress multiple times
			for j := 0; j < 5; j++ {
				err := tsm.UpdateFileProgress(int64(j * 200))
				if err != nil {
					return
				}
				time.Sleep(1 * time.Millisecond)
			}

			// Complete the file
			err = tsm.CompleteCurrentFile()
			if err != nil {
				return
			}
		}(i)
	}

	wg.Wait()

	// Give listeners time to process all events
	time.Sleep(100 * time.Millisecond)

	// Verify that we received some status changes without data races
	fileChanges := listener.GetFileStatusChanges()
	sessionChanges := listener.GetSessionStatusChanges()

	// We should have received some notifications
	assert.Greater(t, len(fileChanges), 0, "Should have received file status changes")
	assert.Greater(t, len(sessionChanges), 0, "Should have received session status changes")

	// Verify that the status objects are valid (not corrupted by race conditions)
	for _, change := range fileChanges {
		if change.NewStatus != nil {
			assert.NotEmpty(t, change.FilePath, "File path should not be empty")
			assert.True(t, change.NewStatus.TotalBytes >= 0, "Total bytes should be non-negative")
			assert.True(t, change.NewStatus.BytesSent >= 0, "Bytes sent should be non-negative")
		}
	}

	for _, change := range sessionChanges {
		if change.NewStatus != nil {
			assert.NotEmpty(t, change.NewStatus.SessionID, "Session ID should not be empty")
			assert.True(t, change.NewStatus.TotalFiles >= 0, "Total files should be non-negative")
			assert.True(t, change.NewStatus.TotalBytes >= 0, "Total bytes should be non-negative")
		}
	}
}

// TestConcurrentStatusUpdates tests concurrent status updates without race conditions
func TestConcurrentStatusUpdates(t *testing.T) {
	tsm := NewTransferStatusManager()

	// Initialize session
	err := tsm.InitializeSession("concurrent-test", 1, 1000)
	require.NoError(t, err)

	// Start a file transfer
	_, err = tsm.StartFileTransfer("/test/concurrent.txt", 1000)
	require.NoError(t, err)

	// Add multiple listeners
	listeners := make([]*TestStatusListener, 5)
	for i := range listeners {
		listeners[i] = NewTestStatusListener()
		tsm.AddStatusListener(listeners[i])
	}

	// Perform concurrent updates
	var wg sync.WaitGroup
	numUpdates := 20

	for i := 0; i < numUpdates; i++ {
		wg.Add(1)
		go func(updateIndex int) {
			defer wg.Done()

			// Update progress
			progress := int64(updateIndex * 50)
			if progress <= 1000 {
				err := tsm.UpdateFileProgress(progress)
				if err != nil {
					return
				}
			}
		}(i)
	}

	wg.Wait()

	// Give listeners time to process
	time.Sleep(50 * time.Millisecond)

	// Verify all listeners received notifications without corruption
	for i, listener := range listeners {
		changes := listener.GetSessionStatusChanges()
		assert.Greater(t, len(changes), 0, "Listener %d should have received changes", i)

		// Verify data integrity
		for _, change := range changes {
			if change.NewStatus != nil {
				assert.Equal(t, "concurrent-test", change.NewStatus.SessionID,
					"Session ID should be preserved in listener %d", i)
			}
		}
	}
}
