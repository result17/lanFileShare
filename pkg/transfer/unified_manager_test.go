package transfer

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/rescp17/lanFileSharer/pkg/fileInfo"
)

func TestUnifiedTransferManager_Basic(t *testing.T) {
	manager := NewUnifiedTransferManager("test-service")
	defer manager.Close()
	
	// Test basic initialization
	if manager == nil {
		t.Fatal("NewUnifiedTransferManager returned nil")
	}
	
	if manager.session == nil {
		t.Error("Session should be initialized")
	}
	
	if manager.config == nil {
		t.Error("Config should be initialized")
	}
	
	// Test empty state
	files := manager.GetAllFiles()
	if len(files) != 0 {
		t.Errorf("Expected 0 files, got %d", len(files))
	}
	
	pending, completed, failed := manager.GetQueueStatus()
	if pending != 0 || completed != 0 || failed != 0 {
		t.Errorf("Expected empty queue, got pending=%d, completed=%d, failed=%d", pending, completed, failed)
	}
}

func TestUnifiedTransferManager_FileManagement(t *testing.T) {
	manager := NewUnifiedTransferManager("test-service")
	defer manager.Close()
	
	// Create test file
	tempDir := t.TempDir()
	testFile := filepath.Join(tempDir, "test.txt")
	content := "test content for transfer"
	err := os.WriteFile(testFile, []byte(content), 0644)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}
	
	node, err := fileInfo.CreateNode(testFile)
	if err != nil {
		t.Fatalf("Failed to create file node: %v", err)
	}
	
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
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}
	
	node, err := fileInfo.CreateNode(testFile)
	if err != nil {
		t.Fatalf("Failed to create file node: %v", err)
	}
	
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
		if err != nil {
			t.Fatalf("Failed to create test file %s: %v", fileName, err)
		}
		
		node, err := fileInfo.CreateNode(filePath)
		if err != nil {
			t.Fatalf("Failed to create file node for %s: %v", fileName, err)
		}
		
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
	fileEvents    []string  // Records file status change events as strings
	sessionEvents []string  // Records session status change events as strings
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
	tsl.sessionEvents = append(tsl.sessionEvents, fmt.Sprintf("session: %s -> %s", oldState, newState))
}

func TestUnifiedTransferManager_StatusListener(t *testing.T) {
	manager := NewUnifiedTransferManager("test-service")
	defer manager.Close()
	
	listener := &testStatusListener{}
	manager.AddStatusListener(listener)
	
	// Create test file
	tempDir := t.TempDir()
	testFile := filepath.Join(tempDir, "test.txt")
	err := os.WriteFile(testFile, []byte("test content"), 0644)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}
	
	node, err := fileInfo.CreateNode(testFile)
	if err != nil {
		t.Fatalf("Failed to create file node: %v", err)
	}
	
	// Add file and start transfer
	manager.AddFile(&node)
	err = manager.StartTransfer(testFile)
	if err != nil {
		t.Errorf("StartTransfer failed: %v", err)
	}
	
	// Give time for async events
	time.Sleep(10 * time.Millisecond)
	
	// Check that events were received
	if len(listener.fileEvents) == 0 {
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
	
	if len(listener.sessionEvents) == 0 {
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
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}
	
	node, err := fileInfo.CreateNode(testFile)
	if err != nil {
		t.Fatalf("Failed to create file node: %v", err)
	}
	
	manager.AddFile(&node)
	
	// Test GetChunker (compatibility method)
	chunker, exists := manager.GetChunker(testFile)
	if !exists {
		t.Error("Chunker should exist for added file")
	}
	
	if chunker == nil {
		t.Error("Chunker should not be nil")
	}
	
	// Test non-existent file
	_, exists = manager.GetChunker("/non/existent/file")
	if exists {
		t.Error("Chunker should not exist for non-existent file")
	}
}