package transfer

import (
	"errors"
	"testing"
)

func TestNewTransferStatusManager(t *testing.T) {
	manager := NewTransferStatusManager()

	if manager == nil {
		t.Fatal("NewTransferStatusManager returned nil")
	}

	if manager.config == nil {
		t.Error("config should be initialized")
	}

	if manager.sessionStatus != nil {
		t.Error("sessionStatus should be nil initially")
	}

	if manager.listeners == nil {
		t.Error("listeners should be initialized")
	}

	// Test that default config is used
	defaultConfig := DefaultTransferConfig()
	if manager.config.ChunkSize != defaultConfig.ChunkSize {
		t.Errorf("Expected ChunkSize %d, got %d", defaultConfig.ChunkSize, manager.config.ChunkSize)
	}
}

func TestNewTransferStatusManagerWithConfig(t *testing.T) {
	customConfig := &TransferConfig{
		ChunkSize:              32 * 1024,
		MinChunkSize:           MinChunkSize,
		MaxChunkSize:           MaxChunkSize,
		MaxConcurrentTransfers: 5,
		MaxConcurrentChunks:    25,
		BufferSize:             4096,
		DefaultRetryPolicy:     DefaultRetryPolicy(),
		EventBufferSize:        50,
	}

	manager := NewTransferStatusManagerWithConfig(customConfig)

	if manager == nil {
		t.Fatal("NewTransferStatusManagerWithConfig returned nil")
	}

	if manager.config.ChunkSize != customConfig.ChunkSize {
		t.Errorf("Expected ChunkSize %d, got %d", customConfig.ChunkSize, manager.config.ChunkSize)
	}

	if manager.config.MaxConcurrentTransfers != customConfig.MaxConcurrentTransfers {
		t.Errorf("Expected MaxConcurrentTransfers %d, got %d",
			customConfig.MaxConcurrentTransfers, manager.config.MaxConcurrentTransfers)
	}
}

func TestNewTransferStatusManagerWithNilConfig(t *testing.T) {
	manager := NewTransferStatusManagerWithConfig(nil)

	if manager == nil {
		t.Fatal("NewTransferStatusManagerWithConfig with nil config returned nil")
	}

	// Should use default config
	defaultConfig := DefaultTransferConfig()
	if manager.config.ChunkSize != defaultConfig.ChunkSize {
		t.Errorf("Expected default ChunkSize %d, got %d", defaultConfig.ChunkSize, manager.config.ChunkSize)
	}
}

func TestTransferStatusManager_InitializeSession(t *testing.T) {
	manager := NewTransferStatusManager()

	tests := []struct {
		name        string
		sessionID   string
		totalFiles  int
		totalBytes  int64
		expectError bool
	}{
		{
			name:        "valid session",
			sessionID:   "test-session-1",
			totalFiles:  3,
			totalBytes:  1024,
			expectError: false,
		},
		{
			name:        "empty session ID",
			sessionID:   "",
			totalFiles:  1,
			totalBytes:  1024,
			expectError: true,
		},
		{
			name:        "negative files",
			sessionID:   "test-session-2",
			totalFiles:  -1,
			totalBytes:  1024,
			expectError: true,
		},
		{
			name:        "negative bytes",
			sessionID:   "test-session-3",
			totalFiles:  1,
			totalBytes:  -1,
			expectError: true,
		},
		{
			name:        "zero files and bytes",
			sessionID:   "test-session-4",
			totalFiles:  0,
			totalBytes:  0,
			expectError: false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// Reset manager for each test
			manager.Clear()

			err := manager.InitializeSession(test.sessionID, test.totalFiles, test.totalBytes)

			if test.expectError {
				if err == nil {
					t.Error("Expected error, but got nil")
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
				}

				// Verify session was created
				status, err := manager.GetSessionStatus()
				if err != nil {
					t.Errorf("GetSessionStatus failed: %v", err)
				} else {
					if status.SessionID != test.sessionID {
						t.Errorf("Expected SessionID %s, got %s", test.sessionID, status.SessionID)
					}
					if status.TotalFiles != test.totalFiles {
						t.Errorf("Expected TotalFiles %d, got %d", test.totalFiles, status.TotalFiles)
					}
					if status.TotalBytes != test.totalBytes {
						t.Errorf("Expected TotalBytes %d, got %d", test.totalBytes, status.TotalBytes)
					}
					if status.State != StatusSessionStateActive {
						t.Errorf("Expected state %s, got %s", StatusSessionStateActive, status.State)
					}
				}
			}
		})
	}
}

func TestTransferStatusManager_StartFileTransfer(t *testing.T) {
	manager := NewTransferStatusManager()

	// Initialize session first
	err := manager.InitializeSession("test-session", 3, 3072)
	if err != nil {
		t.Fatalf("InitializeSession failed: %v", err)
	}

	tests := []struct {
		name        string
		filePath    string
		fileSize    int64
		expectError bool
	}{
		{
			name:        "valid file transfer",
			filePath:    "/test/file.txt",
			fileSize:    1024,
			expectError: false,
		},
		{
			name:        "empty file path",
			filePath:    "",
			fileSize:    1024,
			expectError: true,
		},
		{
			name:        "negative size",
			filePath:    "/test/file2.txt",
			fileSize:    -1,
			expectError: true,
		},
		{
			name:        "zero size file",
			filePath:    "/test/empty.txt",
			fileSize:    0,
			expectError: false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			status, err := manager.StartFileTransfer(test.filePath, test.fileSize)

			if test.expectError {
				if err == nil {
					t.Error("Expected error, but got nil")
				}
				if status != nil {
					t.Error("Expected nil status on error")
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
				}
				if status == nil {
					t.Error("Expected status, but got nil")
				} else {
					if status.FilePath != test.filePath {
						t.Errorf("Expected FilePath %s, got %s", test.filePath, status.FilePath)
					}
					if status.TotalBytes != test.fileSize {
						t.Errorf("Expected TotalBytes %d, got %d", test.fileSize, status.TotalBytes)
					}
					if status.State != TransferStateActive {
						t.Errorf("Expected state %s, got %s", TransferStateActive, status.State)
					}
				}

				// Complete this transfer before starting next one
				manager.CompleteCurrentFile()
			}
		})
	}
}

func TestTransferStatusManager_StartFileTransfer_NoSession(t *testing.T) {
	manager := NewTransferStatusManager()

	// Try to start file transfer without initializing session
	_, err := manager.StartFileTransfer("/test/file.txt", 1024)
	if err == nil {
		t.Error("Expected error when starting file transfer without session")
	}
	if !errors.Is(err, ErrSessionNotFound) {
		t.Errorf("Expected ErrSessionNotFound, got %v", err)
	}
}

func TestTransferStatusManager_StartFileTransfer_ActiveFile(t *testing.T) {
	manager := NewTransferStatusManager()

	// Initialize session
	err := manager.InitializeSession("test-session", 2, 2048)
	if err != nil {
		t.Fatalf("InitializeSession failed: %v", err)
	}

	// Start first file transfer
	_, err = manager.StartFileTransfer("/test/file1.txt", 1024)
	if err != nil {
		t.Fatalf("First StartFileTransfer failed: %v", err)
	}

	// Try to start second file transfer while first is active
	_, err = manager.StartFileTransfer("/test/file2.txt", 1024)
	if err == nil {
		t.Error("Expected error when starting second file transfer while first is active")
	}
}

func TestTransferStatusManager_GetSessionStatus(t *testing.T) {
	manager := NewTransferStatusManager()

	// Test getting session status when no session exists
	_, err := manager.GetSessionStatus()
	if err == nil {
		t.Error("Expected error for non-existent session")
	}
	if !errors.Is(err, ErrSessionNotFound) {
		t.Errorf("Expected ErrSessionNotFound, got %v", err)
	}

	// Initialize session
	sessionID := "test-session"
	totalFiles := 3
	totalBytes := int64(3072)

	err = manager.InitializeSession(sessionID, totalFiles, totalBytes)
	if err != nil {
		t.Fatalf("InitializeSession failed: %v", err)
	}

	// Get session status
	status, err := manager.GetSessionStatus()
	if err != nil {
		t.Errorf("GetSessionStatus failed: %v", err)
	}

	// Verify the status
	if status.SessionID != sessionID {
		t.Errorf("Expected SessionID %s, got %s", sessionID, status.SessionID)
	}
	if status.TotalFiles != totalFiles {
		t.Errorf("Expected TotalFiles %d, got %d", totalFiles, status.TotalFiles)
	}
	if status.TotalBytes != totalBytes {
		t.Errorf("Expected TotalBytes %d, got %d", totalBytes, status.TotalBytes)
	}
	if status.CompletedFiles != 0 {
		t.Errorf("Expected CompletedFiles 0, got %d", status.CompletedFiles)
	}
	if status.PendingFiles != totalFiles {
		t.Errorf("Expected PendingFiles %d, got %d", totalFiles, status.PendingFiles)
	}
}

func TestTransferStatusManager_UpdateFileProgress(t *testing.T) {
	manager := NewTransferStatusManager()

	// Initialize session
	err := manager.InitializeSession("test-session", 1, 1024)
	if err != nil {
		t.Fatalf("InitializeSession failed: %v", err)
	}

	// Start file transfer
	filePath := "/test/file.txt"
	fileSize := int64(1024)
	_, err = manager.StartFileTransfer(filePath, fileSize)
	if err != nil {
		t.Fatalf("StartFileTransfer failed: %v", err)
	}

	// Update progress
	bytesSent := int64(512)
	err = manager.UpdateFileProgress(bytesSent)
	if err != nil {
		t.Errorf("UpdateFileProgress failed: %v", err)
	}

	// Verify progress was updated
	currentFile, err := manager.GetCurrentFile()
	if err != nil {
		t.Fatalf("GetCurrentFile failed: %v", err)
	}

	if currentFile.BytesSent != bytesSent {
		t.Errorf("Expected BytesSent %d, got %d", bytesSent, currentFile.BytesSent)
	}

	// Verify session progress was updated
	sessionStatus, err := manager.GetSessionStatus()
	if err != nil {
		t.Fatalf("GetSessionStatus failed: %v", err)
	}

	expectedProgress := float64(bytesSent) / float64(fileSize) * 100.0
	if sessionStatus.OverallProgress != expectedProgress {
		t.Errorf("Expected OverallProgress %.2f, got %.2f", expectedProgress, sessionStatus.OverallProgress)
	}

	// Test invalid updates
	tests := []struct {
		name      string
		bytesSent int64
		expectErr bool
	}{
		{"negative bytes", -1, true},
		{"valid progress", fileSize, false},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := manager.UpdateFileProgress(test.bytesSent)
			if test.expectErr && err == nil {
				t.Error("Expected error, but got nil")
			}
			if !test.expectErr && err != nil {
				t.Errorf("Unexpected error: %v", err)
			}
		})
	}
}
func TestTransferStatusManager_CompleteCurrentFile(t *testing.T) {
	manager := NewTransferStatusManager()

	// Test completing when no session exists
	err := manager.CompleteCurrentFile()
	if err == nil {
		t.Error("Expected error when no session exists")
	}
	if !errors.Is(err, ErrSessionNotFound) {
		t.Errorf("Expected ErrSessionNotFound, got %v", err)
	}

	// Initialize session
	err = manager.InitializeSession("test-session", 2, 2048)
	if err != nil {
		t.Fatalf("InitializeSession failed: %v", err)
	}

	// Test completing when no current file
	err = manager.CompleteCurrentFile()
	if err == nil {
		t.Error("Expected error when no current file")
	}
	if !errors.Is(err, ErrTransferNotFound) {
		t.Errorf("Expected ErrTransferNotFound, got %v", err)
	}

	// Start file transfer
	filePath := "/test/file.txt"
	fileSize := int64(1024)
	_, err = manager.StartFileTransfer(filePath, fileSize)
	if err != nil {
		t.Fatalf("StartFileTransfer failed: %v", err)
	}

	// Complete the transfer
	err = manager.CompleteCurrentFile()
	if err != nil {
		t.Errorf("CompleteCurrentFile failed: %v", err)
	}

	// Verify session status was updated
	sessionStatus, err := manager.GetSessionStatus()
	if err != nil {
		t.Fatalf("GetSessionStatus failed: %v", err)
	}

	if sessionStatus.CompletedFiles != 1 {
		t.Errorf("Expected CompletedFiles 1, got %d", sessionStatus.CompletedFiles)
	}

	if sessionStatus.PendingFiles != 1 {
		t.Errorf("Expected PendingFiles 1, got %d", sessionStatus.PendingFiles)
	}

	if sessionStatus.BytesCompleted != fileSize {
		t.Errorf("Expected BytesCompleted %d, got %d", fileSize, sessionStatus.BytesCompleted)
	}

	if sessionStatus.CurrentFile != nil {
		t.Error("CurrentFile should be nil after completion")
	}
}

func TestTransferStatusManager_FailCurrentFile(t *testing.T) {
	manager := NewTransferStatusManager()
	testError := errors.New("test error")

	// Test failing when no session exists
	err := manager.FailCurrentFile(testError)
	if err == nil {
		t.Error("Expected error when no session exists")
	}
	if !errors.Is(err, ErrSessionNotFound) {
		t.Errorf("Expected ErrSessionNotFound, got %v", err)
	}

	// Initialize session
	err = manager.InitializeSession("test-session", 2, 2048)
	if err != nil {
		t.Fatalf("InitializeSession failed: %v", err)
	}

	// Test failing when no current file
	err = manager.FailCurrentFile(testError)
	if err == nil {
		t.Error("Expected error when no current file")
	}
	if !errors.Is(err, ErrTransferNotFound) {
		t.Errorf("Expected ErrTransferNotFound, got %v", err)
	}

	// Start file transfer
	filePath := "/test/file.txt"
	fileSize := int64(1024)
	_, err = manager.StartFileTransfer(filePath, fileSize)
	if err != nil {
		t.Fatalf("StartFileTransfer failed: %v", err)
	}

	// Fail the transfer
	err = manager.FailCurrentFile(testError)
	if err != nil {
		t.Errorf("FailCurrentFile failed: %v", err)
	}

	// Verify session status was updated
	sessionStatus, err := manager.GetSessionStatus()
	if err != nil {
		t.Fatalf("GetSessionStatus failed: %v", err)
	}

	if sessionStatus.FailedFiles != 1 {
		t.Errorf("Expected FailedFiles 1, got %d", sessionStatus.FailedFiles)
	}

	if sessionStatus.PendingFiles != 1 {
		t.Errorf("Expected PendingFiles 1, got %d", sessionStatus.PendingFiles)
	}

	if sessionStatus.CurrentFile != nil {
		t.Error("CurrentFile should be nil after failure")
	}
}

func TestTransferStatusManager_PauseResumeCurrentFile(t *testing.T) {
	manager := NewTransferStatusManager()

	// Initialize session
	err := manager.InitializeSession("test-session", 1, 1024)
	if err != nil {
		t.Fatalf("InitializeSession failed: %v", err)
	}

	// Start file transfer
	filePath := "/test/file.txt"
	fileSize := int64(1024)
	_, err = manager.StartFileTransfer(filePath, fileSize)
	if err != nil {
		t.Fatalf("StartFileTransfer failed: %v", err)
	}

	// Pause the transfer
	err = manager.PauseCurrentFile()
	if err != nil {
		t.Errorf("PauseCurrentFile failed: %v", err)
	}

	// Verify the transfer is paused
	currentFile, err := manager.GetCurrentFile()
	if err != nil {
		t.Fatalf("GetCurrentFile failed: %v", err)
	}

	if currentFile.State != TransferStatePaused {
		t.Errorf("Expected state %s, got %s", TransferStatePaused, currentFile.State)
	}

	// Resume the transfer
	err = manager.ResumeCurrentFile()
	if err != nil {
		t.Errorf("ResumeCurrentFile failed: %v", err)
	}

	// Verify the transfer is active again
	currentFile, err = manager.GetCurrentFile()
	if err != nil {
		t.Fatalf("GetCurrentFile failed: %v", err)
	}

	if currentFile.State != TransferStateActive {
		t.Errorf("Expected state %s, got %s", TransferStateActive, currentFile.State)
	}
}

func TestTransferStatusManager_GetCurrentFile(t *testing.T) {
	manager := NewTransferStatusManager()

	// Test getting current file when no session exists
	_, err := manager.GetCurrentFile()
	if err == nil {
		t.Error("Expected error when no session exists")
	}
	if !errors.Is(err, ErrSessionNotFound) {
		t.Errorf("Expected ErrSessionNotFound, got %v", err)
	}

	// Initialize session
	err = manager.InitializeSession("test-session", 1, 1024)
	if err != nil {
		t.Fatalf("InitializeSession failed: %v", err)
	}

	// Test getting current file when no file is active
	_, err = manager.GetCurrentFile()
	if err == nil {
		t.Error("Expected error when no current file")
	}
	if !errors.Is(err, ErrTransferNotFound) {
		t.Errorf("Expected ErrTransferNotFound, got %v", err)
	}

	// Start file transfer
	filePath := "/test/file.txt"
	fileSize := int64(1024)
	originalStatus, err := manager.StartFileTransfer(filePath, fileSize)
	if err != nil {
		t.Fatalf("StartFileTransfer failed: %v", err)
	}

	// Get current file
	currentFile, err := manager.GetCurrentFile()
	if err != nil {
		t.Errorf("GetCurrentFile failed: %v", err)
	}

	// Verify the file matches
	if currentFile.FilePath != originalStatus.FilePath {
		t.Errorf("Expected FilePath %s, got %s", originalStatus.FilePath, currentFile.FilePath)
	}
	if currentFile.TotalBytes != originalStatus.TotalBytes {
		t.Errorf("Expected TotalBytes %d, got %d", originalStatus.TotalBytes, currentFile.TotalBytes)
	}
}

func TestTransferStatusManager_IsSessionActive(t *testing.T) {
	manager := NewTransferStatusManager()

	// Initially should be false
	if manager.IsSessionActive() {
		t.Error("Expected IsSessionActive to be false initially")
	}

	// Initialize session
	err := manager.InitializeSession("test-session", 1, 1024)
	if err != nil {
		t.Fatalf("InitializeSession failed: %v", err)
	}

	// Should be true now
	if !manager.IsSessionActive() {
		t.Error("Expected IsSessionActive to be true after initialization")
	}

	// Start and complete a file transfer
	_, err = manager.StartFileTransfer("/test/file.txt", 1024)
	if err != nil {
		t.Fatalf("StartFileTransfer failed: %v", err)
	}

	err = manager.CompleteCurrentFile()
	if err != nil {
		t.Fatalf("CompleteCurrentFile failed: %v", err)
	}

	// Should be false now (session completed)
	if manager.IsSessionActive() {
		t.Error("Expected IsSessionActive to be false after session completion")
	}
}
func TestTransferStatusManager_ResetSession(t *testing.T) {
	manager := NewTransferStatusManager()

	// Initialize session
	err := manager.InitializeSession("test-session", 1, 1024)
	if err != nil {
		t.Fatalf("InitializeSession failed: %v", err)
	}

	// Verify session exists
	if !manager.IsSessionActive() {
		t.Error("Expected session to be active")
	}

	// Reset session
	manager.ResetSession()

	// Verify session is cleared
	if manager.IsSessionActive() {
		t.Error("Expected session to be inactive after reset")
	}

	_, err = manager.GetSessionStatus()
	if err == nil {
		t.Error("Expected error getting session status after reset")
	}
	if !errors.Is(err, ErrSessionNotFound) {
		t.Errorf("Expected ErrSessionNotFound, got %v", err)
	}
}

func TestTransferStatusManager_Clear(t *testing.T) {
	manager := NewTransferStatusManager()

	// Initialize session
	err := manager.InitializeSession("test-session", 1, 1024)
	if err != nil {
		t.Fatalf("InitializeSession failed: %v", err)
	}

	// Verify session exists
	if !manager.IsSessionActive() {
		t.Error("Expected session to be active")
	}

	// Clear manager
	manager.Clear()

	// Verify session is cleared
	if manager.IsSessionActive() {
		t.Error("Expected session to be inactive after clear")
	}

	_, err = manager.GetSessionStatus()
	if err == nil {
		t.Error("Expected error getting session status after clear")
	}
	if !errors.Is(err, ErrSessionNotFound) {
		t.Errorf("Expected ErrSessionNotFound, got %v", err)
	}
}
