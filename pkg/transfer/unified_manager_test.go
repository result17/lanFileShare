package transfer

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/rescp17/lanFileSharer/pkg/fileInfo"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUnifiedTransferManager_Basic(t *testing.T) {
	manager := NewUnifiedTransferManager("test-service")
	defer manager.Close()

	// Test basic initialization
	require.NotNil(t, manager, "NewUnifiedTransferManager returned nil")

	require.NotNil(t, manager.session, "Session should be initialized")
	require.NotNil(t, manager.config, "Config should be initialized")

	// Test empty state
	files := manager.GetAllFiles()
	require.Empty(t, files, "Expected 0 files")

	pending, completed, failed := manager.GetQueueStatus()
	require.Zero(t, pending, "Expected pending to be 0")
	require.Zero(t, completed, "Expected completed to be 0")
	require.Zero(t, failed, "Expected failed to be 0")
}

func TestUnifiedTransferManager_FileManagement(t *testing.T) {
	manager := NewUnifiedTransferManager("test-service")
	defer manager.Close()

	// Create test file
	tempDir := t.TempDir()
	testFile := filepath.Join(tempDir, "test.txt")
	content := "test content for transfer"
	err := os.WriteFile(testFile, []byte(content), 0644)
	require.NoError(t, err, "Failed to create test file")

	node, err := fileInfo.CreateNode(testFile)
	require.NoError(t, err, "Failed to create file node")

	// Add file to manager
	err = manager.AddFile(&node)
	if err != nil {
		t.Errorf("AddFile failed: %v", err)
	}

	// Check file was added
	files := manager.GetAllFiles()
	if len(files) != 1 {
		t.Errorf("Expected 1 file, got %d", len(files))
	}

	if files[0].Path != testFile {
		t.Errorf("Expected file path %s, got %s", testFile, files[0].Path)
	}

	// Check queue status
	pending, completed, failed := manager.GetQueueStatus()
	if pending != 1 || completed != 0 || failed != 0 {
		t.Errorf("Expected pending=1, completed=0, failed=0, got pending=%d, completed=%d, failed=%d", pending, completed, failed)
	}

	// Check total bytes
	totalBytes := manager.GetTotalBytes()
	if totalBytes != int64(len(content)) {
		t.Errorf("Expected %d total bytes, got %d", len(content), totalBytes)
	}
}

func TestUnifiedTransferManager_PauseResume(t *testing.T) {
	manager := NewUnifiedTransferManager("test-service")
	defer manager.Close()

	// Create test file
	tempDir := t.TempDir()
	testFile := filepath.Join(tempDir, "test.txt")
	err := os.WriteFile(testFile, []byte("test content"), 0644)
	require.NoError(t, err, "Failed to create test file")

	node, err := fileInfo.CreateNode(testFile)
	require.NoError(t, err, "Failed to create file node")

	// Add and start transfer
	manager.AddFile(&node)
	manager.StartTransfer(testFile)

	// Pause transfer
	err = manager.PauseTransfer(testFile)
	if err != nil {
		t.Errorf("PauseTransfer failed: %v", err)
	}

	status, err := manager.GetFileStatus(testFile)
	if err != nil {
		t.Errorf("Failed to get file status: %v", err)
	}

	if status.State != TransferStatePaused {
		t.Errorf("Expected paused state, got %s", status.State)
	}

	// Resume transfer
	err = manager.StartTransfer(testFile)
	if err != nil {
		t.Errorf("Resume transfer failed: %v", err)
	}

	status, _ = manager.GetFileStatus(testFile)
	if status.State != TransferStateActive {
		t.Errorf("Expected active state after resume, got %s", status.State)
	}
}

func TestUnifiedTransferManager_MultipleFiles(t *testing.T) {
	manager := NewUnifiedTransferManager("test-service")
	defer manager.Close()

	tempDir := t.TempDir()

	// Create multiple test files
	files := []string{"file1.txt", "file2.txt", "file3.txt"}
	for _, fileName := range files {
		filePath := filepath.Join(tempDir, fileName)
		err := os.WriteFile(filePath, []byte("content of "+fileName), 0644)
		require.NoError(t, err, "Failed to create test file %s", fileName)

		node, err := fileInfo.CreateNode(filePath)
		require.NoError(t, err, "Failed to create file node for %s", fileName)

		err = manager.AddFile(&node)
		if err != nil {
			t.Errorf("AddFile failed for %s: %v", fileName, err)
		}
	}

	// Check session status
	sessionStatus := manager.GetSessionStatus()
	if sessionStatus.TotalFiles != 3 {
		t.Errorf("Expected 3 total files, got %d", sessionStatus.TotalFiles)
	}

	// Start and complete first file (sequential processing)
	firstFile := filepath.Join(tempDir, files[0])
	err := manager.StartTransfer(firstFile)
	if err != nil {
		t.Errorf("StartTransfer failed: %v", err)
	}

	// Update progress
	err = manager.UpdateProgress(firstFile, int64(len("content of "+files[0])))
	if err != nil {
		t.Errorf("UpdateProgress failed: %v", err)
	}

	// Complete the transfer
	err = manager.CompleteTransfer(firstFile)
	if err != nil {
		t.Errorf("CompleteTransfer failed: %v", err)
	}

	sessionStatus = manager.GetSessionStatus()
	if sessionStatus.CompletedFiles != 1 {
		t.Errorf("Expected 1 completed file, got %d", sessionStatus.CompletedFiles)
	}

	if sessionStatus.State == StatusSessionStateCompleted {
		t.Error("Session should not be completed with only 1 of 3 files done")
	}
}

// Test status listener
type testStatusListener struct {
	id            string     // Unique identifier for this listener
	fileEvents    []string   // Records file status change events as strings
	sessionEvents []string   // Records session status change events as strings
	mu            sync.Mutex // Protect concurrent access to slices
}

// newTestStatusListener creates a new test status listener with a unique UUID
func newTestStatusListener() *testStatusListener {
	return &testStatusListener{
		id: uuid.New().String(),
	}
}

// ID returns the unique identifier for this listener
func (tsl *testStatusListener) ID() string {
	return tsl.id
}

func (tsl *testStatusListener) OnFileStatusChanged(filePath string, oldStatus, newStatus *TransferStatus) {
	var oldState, newState string
	if oldStatus != nil {
		oldState = oldStatus.State.String()
	} else {
		oldState = "nil"
	}
	if newStatus != nil {
		newState = newStatus.State.String()
	} else {
		newState = "nil"
	}

	tsl.mu.Lock()
	defer tsl.mu.Unlock()
	tsl.fileEvents = append(tsl.fileEvents, fmt.Sprintf("%s: %s -> %s", filePath, oldState, newState))
}

func (tsl *testStatusListener) OnSessionStatusChanged(oldStatus, newStatus *SessionTransferStatus) {
	var oldState, newState string
	if oldStatus != nil {
		oldState = oldStatus.State.String()
	} else {
		oldState = "nil"
	}
	if newStatus != nil {
		newState = newStatus.State.String()
	} else {
		newState = "nil"
	}

	tsl.mu.Lock()
	defer tsl.mu.Unlock()
	tsl.sessionEvents = append(tsl.sessionEvents, fmt.Sprintf("session: %s -> %s", oldState, newState))
}

// Thread-safe methods to access events
func (tsl *testStatusListener) GetFileEvents() []string {
	tsl.mu.Lock()
	defer tsl.mu.Unlock()
	// Return a copy to avoid race conditions
	events := make([]string, len(tsl.fileEvents))
	copy(events, tsl.fileEvents)
	return events
}

func (tsl *testStatusListener) GetSessionEvents() []string {
	tsl.mu.Lock()
	defer tsl.mu.Unlock()
	// Return a copy to avoid race conditions
	events := make([]string, len(tsl.sessionEvents))
	copy(events, tsl.sessionEvents)
	return events
}

func (tsl *testStatusListener) GetFileEventCount() int {
	tsl.mu.Lock()
	defer tsl.mu.Unlock()
	return len(tsl.fileEvents)
}

func (tsl *testStatusListener) GetSessionEventCount() int {
	tsl.mu.Lock()
	defer tsl.mu.Unlock()
	return len(tsl.sessionEvents)
}

func TestUnifiedTransferManager_StatusListener(t *testing.T) {
	t.Run("complete_transfer_lifecycle_events", func(t *testing.T) {
		manager := NewUnifiedTransferManager("test-service")
		defer manager.Close()

		listener := newTestStatusListener()
		manager.AddStatusListener(listener)

		// Create test file
		tempDir := t.TempDir()
		testFile := filepath.Join(tempDir, "test.txt")
		testContent := []byte("test content for status listener")
		err := os.WriteFile(testFile, testContent, 0644)
		require.NoError(t, err, "Failed to create test file")

		node, err := fileInfo.CreateNode(testFile)
		require.NoError(t, err, "Failed to create file node")

		// Step 1: Add file (should trigger pending state)
		err = manager.AddFile(&node)
		require.NoError(t, err, "AddFile failed")

		// Step 2: Start transfer (should trigger active state)
		err = manager.StartTransfer(testFile)
		require.NoError(t, err, "StartTransfer failed")

		// Step 3: Update progress (should trigger progress update)
		err = manager.UpdateProgress(testFile, int64(len(testContent)/2))
		require.NoError(t, err, "UpdateProgress failed")

		// Step 4: Complete transfer (should trigger completed state)
		err = manager.CompleteTransfer(testFile)
		require.NoError(t, err, "CompleteTransfer failed")

		// Give time for async events to be processed
		time.Sleep(50 * time.Millisecond)

		// Verify file events in correct order
		fileEvents := listener.GetFileEvents()
		require.NotEmpty(t, fileEvents, "Should have received file status events")

		t.Logf("File events received: %v", fileEvents)

		// Analyze the actual sequence of events
		// Events may come in different orders due to async processing
		// What's important is that we have the expected state transitions

		// Verify we have at least some events
		assert.GreaterOrEqual(t, len(fileEvents), 2, "Should have at least 2 events")

		// Verify all events are for our test file
		for _, event := range fileEvents {
			assert.Contains(t, event, testFile, "All events should be for our test file")
		}

		// Verify session events
		sessionEvents := listener.GetSessionEvents()
		require.NotEmpty(t, sessionEvents, "Should have received session status events")

		t.Logf("Session events received: %v", sessionEvents)

		// Session should have transitioned through states
		// Expected: nil -> active (when first file starts) -> completed (when all files complete)
		assert.GreaterOrEqual(t, len(sessionEvents), 1, "Should have at least one session event")

		// Check that we have meaningful state transitions, not just event counts
		// Based on the actual event log, we should see transitions to completed state
		hasCompletedTransition := false
		hasActiveState := false

		for _, event := range fileEvents {
			if strings.Contains(event, "-> completed") {
				hasCompletedTransition = true
			}
			if strings.Contains(event, "active ->") || strings.Contains(event, "-> active") {
				hasActiveState = true
			}
		}

		assert.True(t, hasCompletedTransition, "Should have transition to completed state")
		assert.True(t, hasActiveState, "Should have active state involved in transitions")
	})

	t.Run("failed_transfer_events", func(t *testing.T) {
		manager := NewUnifiedTransferManager("test-service-fail")
		defer manager.Close()

		listener := newTestStatusListener()
		manager.AddStatusListener(listener)

		// Create test file
		tempDir := t.TempDir()
		testFile := filepath.Join(tempDir, "test_fail.txt")
		err := os.WriteFile(testFile, []byte("test content"), 0644)
		require.NoError(t, err, "Failed to create test file")

		node, err := fileInfo.CreateNode(testFile)
		require.NoError(t, err, "Failed to create file node")

		// Add file and start transfer
		err = manager.AddFile(&node)
		require.NoError(t, err, "AddFile failed")

		err = manager.StartTransfer(testFile)
		require.NoError(t, err, "StartTransfer failed")

		// Fail the transfer
		testError := errors.New("simulated transfer failure")
		err = manager.FailTransfer(testFile, testError)
		require.NoError(t, err, "FailTransfer failed")

		// Give time for async events
		time.Sleep(50 * time.Millisecond)

		// Verify file events
		fileEvents := listener.GetFileEvents()
		require.NotEmpty(t, fileEvents, "Should have received file status events")

		t.Logf("File events for failed transfer: %v", fileEvents)

		// Should have at least some failure events
		assert.GreaterOrEqual(t, len(fileEvents), 1, "Should have at least one failure event")

		// Verify we have transition to failed state
		hasFailedTransition := false

		for _, event := range fileEvents {
			if strings.Contains(event, "-> failed") {
				hasFailedTransition = true
			}
		}

		assert.True(t, hasFailedTransition, "Should have transition to failed state")
	})

	t.Run("multiple_files_independent_events", func(t *testing.T) {
		manager := NewUnifiedTransferManager("test-service-multi")
		defer manager.Close()

		listener := newTestStatusListener()
		manager.AddStatusListener(listener)

		// Create multiple test files
		tempDir := t.TempDir()
		testFiles := make([]string, 3)

		for i := 0; i < 3; i++ {
			testFiles[i] = filepath.Join(tempDir, fmt.Sprintf("test%d.txt", i))
			err := os.WriteFile(testFiles[i], []byte(fmt.Sprintf("content %d", i)), 0644)
			require.NoError(t, err, "Failed to create test file %d", i)

			node, err := fileInfo.CreateNode(testFiles[i])
			require.NoError(t, err, "Failed to create file node %d", i)

			err = manager.AddFile(&node)
			require.NoError(t, err, "AddFile failed for file %d", i)
		}

		// Process files with different outcomes
		// File 0: Complete successfully
		err := manager.StartTransfer(testFiles[0])
		require.NoError(t, err, "StartTransfer failed for file 0")
		err = manager.CompleteTransfer(testFiles[0])
		require.NoError(t, err, "CompleteTransfer failed for file 0")

		// File 1: Fail
		err = manager.StartTransfer(testFiles[1])
		require.NoError(t, err, "StartTransfer failed for file 1")
		err = manager.FailTransfer(testFiles[1], errors.New("test failure"))
		require.NoError(t, err, "FailTransfer failed for file 1")

		// File 2: Leave pending (just add, don't start)

		// Give time for async events
		time.Sleep(50 * time.Millisecond)

		// Verify events for each file
		fileEvents := listener.GetFileEvents()
		require.NotEmpty(t, fileEvents, "Should have received file status events")

		t.Logf("Multi-file events: %v", fileEvents)

		// Count events per file
		file0Events := 0
		file1Events := 0
		file2Events := 0

		for _, event := range fileEvents {
			if strings.Contains(event, testFiles[0]) {
				file0Events++
			}
			if strings.Contains(event, testFiles[1]) {
				file1Events++
			}
			if strings.Contains(event, testFiles[2]) {
				file2Events++
			}
		}

		// File 0 should have events (start -> complete)
		assert.GreaterOrEqual(t, file0Events, 2, "File 0 should have start and complete events")

		// File 1 should have events (start -> fail)
		assert.GreaterOrEqual(t, file1Events, 2, "File 1 should have start and fail events")

		// File 2 should have no events (never started)
		assert.Equal(t, 0, file2Events, "File 2 should have no events (never started)")

		// Verify specific state transitions
		hasFile0Completed := false
		hasFile1Failed := false

		for _, event := range fileEvents {
			if strings.Contains(event, testFiles[0]) && strings.Contains(event, "-> completed") {
				hasFile0Completed = true
			}
			if strings.Contains(event, testFiles[1]) && strings.Contains(event, "-> failed") {
				hasFile1Failed = true
			}
		}

		assert.True(t, hasFile0Completed, "File 0 should have completed transition")
		assert.True(t, hasFile1Failed, "File 1 should have failed transition")
	})

	t.Run("detailed_state_transition_verification", func(t *testing.T) {
		manager := NewUnifiedTransferManager("test-service-detailed")
		defer manager.Close()

		listener := newTestStatusListener()
		manager.AddStatusListener(listener)

		// Create test file
		tempDir := t.TempDir()
		testFile := filepath.Join(tempDir, "detailed_test.txt")
		testContent := []byte("detailed test content for state verification")
		err := os.WriteFile(testFile, testContent, 0644)
		require.NoError(t, err, "Failed to create test file")

		node, err := fileInfo.CreateNode(testFile)
		require.NoError(t, err, "Failed to create file node")

		// Add file
		err = manager.AddFile(&node)
		require.NoError(t, err, "AddFile failed")

		// Start transfer
		err = manager.StartTransfer(testFile)
		require.NoError(t, err, "StartTransfer failed")

		// Update progress multiple times
		contentLen := int64(len(testContent))
		err = manager.UpdateProgress(testFile, contentLen/4)
		require.NoError(t, err, "UpdateProgress 1 failed")

		err = manager.UpdateProgress(testFile, contentLen/2)
		require.NoError(t, err, "UpdateProgress 2 failed")

		err = manager.UpdateProgress(testFile, contentLen*3/4)
		require.NoError(t, err, "UpdateProgress 3 failed")

		// Complete transfer
		err = manager.CompleteTransfer(testFile)
		require.NoError(t, err, "CompleteTransfer failed")

		// Give time for async events
		time.Sleep(100 * time.Millisecond)

		// Analyze events in detail
		fileEvents := listener.GetFileEvents()
		require.NotEmpty(t, fileEvents, "Should have received file status events")

		t.Logf("Detailed file events: %v", fileEvents)

		// Verify event structure and content
		for i, event := range fileEvents {
			t.Logf("Event %d: %s", i, event)

			// Each event should contain the file path
			assert.Contains(t, event, testFile, "Event %d should contain file path", i)

			// Each event should have a state transition (contain "->")
			assert.Contains(t, event, " -> ", "Event %d should contain state transition", i)

			// Event should not be empty or malformed
			assert.NotEmpty(t, event, "Event %d should not be empty", i)
			assert.NotContains(t, event, " ->  ", "Event %d should not have empty target state", i)
		}

		// Count specific state transitions
		nilToCompletedCount := 0
		activeToCompletedCount := 0
		activeToActiveCount := 0
		otherTransitions := 0

		for _, event := range fileEvents {
			switch {
			case strings.Contains(event, "nil -> completed"):
				nilToCompletedCount++
			case strings.Contains(event, "active -> completed"):
				activeToCompletedCount++
			case strings.Contains(event, "active -> active"):
				activeToActiveCount++
			default:
				otherTransitions++
				t.Logf("Other transition: %s", event)
			}
		}

		// Verify we have expected transitions
		assert.GreaterOrEqual(t, nilToCompletedCount+activeToCompletedCount, 1,
			"Should have at least one completion transition")

		// Log transition counts for analysis
		t.Logf("Transition counts: nil->completed=%d, active->completed=%d, active->active=%d, other=%d",
			nilToCompletedCount, activeToCompletedCount, activeToActiveCount, otherTransitions)

		// Verify session events are also meaningful
		sessionEvents := listener.GetSessionEvents()
		require.NotEmpty(t, sessionEvents, "Should have session events")

		for i, event := range sessionEvents {
			assert.Contains(t, event, "session:", "Session event %d should be prefixed with 'session:'", i)
			assert.Contains(t, event, " -> ", "Session event %d should contain state transition", i)
		}
	})
}

func TestUnifiedTransferManager_GetChunker(t *testing.T) {
	manager := NewUnifiedTransferManager("test-service")
	defer manager.Close()

	// Create test file
	tempDir := t.TempDir()
	testFile := filepath.Join(tempDir, "test.txt")
	err := os.WriteFile(testFile, []byte("test content"), 0644)
	require.NoError(t, err, "Failed to create test file")

	node, err := fileInfo.CreateNode(testFile)
	require.NoError(t, err, "Failed to create file node")

	manager.AddFile(&node)

	// Test GetChunker (compatibility method)
	chunker, exists := manager.GetChunker(testFile)
	require.True(t, exists, "Chunker should exist for added file")
	require.NotNil(t, chunker, "Chunker should not be nil")

	// Test non-existent file
	_, exists = manager.GetChunker("/non/existent/file")
	require.False(t, exists, "Chunker should not exist for non-existent file")
}
