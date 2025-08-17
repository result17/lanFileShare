package transfer

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/rescp17/lanFileSharer/pkg/fileInfo"
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

	status, _ := manager.GetFileStatus(testFile)
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
	manager := NewUnifiedTransferManager("test-service")
	defer manager.Close()

	listener := newTestStatusListener()
	manager.AddStatusListener(listener)

	// Create test file
	tempDir := t.TempDir()
	testFile := filepath.Join(tempDir, "test.txt")
	err := os.WriteFile(testFile, []byte("test content"), 0644)
	require.NoError(t, err, "Failed to create test file")

	node, err := fileInfo.CreateNode(testFile)
	require.NoError(t, err, "Failed to create file node")

	// Add file and start transfer
	manager.AddFile(&node)
	err = manager.StartTransfer(testFile)
	require.NoError(t, err, "StartTransfer failed")

	// Give time for async events
	time.Sleep(10 * time.Millisecond)

	// Check that events were received
	if listener.GetFileEventCount() == 0 {
		t.Error("Expected file status events")
	}

	// Update progress
	err = manager.UpdateProgress(testFile, int64(len("test content")))
	if err != nil {
		t.Errorf("UpdateProgress failed: %v", err)
	}

	// Complete transfer
	err = manager.CompleteTransfer(testFile)
	if err != nil {
		t.Errorf("CompleteTransfer failed: %v", err)
	}

	// Give time for async events
	time.Sleep(10 * time.Millisecond)

	if listener.GetSessionEventCount() == 0 {
		t.Error("Expected session status events")
	}
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
