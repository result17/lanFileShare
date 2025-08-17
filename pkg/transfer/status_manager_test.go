package transfer

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewTransferStatusManager(t *testing.T) {
	manager := NewTransferStatusManager()

	require.NotNil(t, manager, "NewTransferStatusManager returned nil")

	require.NotNil(t, manager.config, "config should be initialized")
	require.Nil(t, manager.sessionStatus, "sessionStatus should be nil initially")
	require.NotNil(t, manager.listeners, "listeners should be initialized")

	// Test that default config is used
	defaultConfig := DefaultTransferConfig()
	require.Equal(t, defaultConfig.ChunkSize, manager.config.ChunkSize, "Should use default ChunkSize")
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

	require.NotNil(t, manager, "NewTransferStatusManagerWithConfig returned nil")

	require.Equal(t, customConfig.ChunkSize, manager.config.ChunkSize, "Should use custom ChunkSize")
	require.Equal(t, customConfig.MaxConcurrentTransfers, manager.config.MaxConcurrentTransfers, "Should use custom MaxConcurrentTransfers")
}

func TestNewTransferStatusManagerWithNilConfig(t *testing.T) {
	manager := NewTransferStatusManagerWithConfig(nil)

	require.NotNil(t, manager, "NewTransferStatusManagerWithConfig with nil config returned nil")

	// Should use default config
	defaultConfig := DefaultTransferConfig()
	require.Equal(t, defaultConfig.ChunkSize, manager.config.ChunkSize, "Should use default ChunkSize when config is nil")
}

func TestTransferStatusManager_InitializeSession(t *testing.T) {
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
			// Create a fresh manager instance for each sub-test to ensure complete isolation
			manager := NewTransferStatusManager()

			err := manager.InitializeSession(test.sessionID, test.totalFiles, test.totalBytes)

			if test.expectError {
				require.Error(t, err, "Expected error, but got nil")
			} else {
				require.NoError(t, err, "Unexpected error")

				// Verify session was created
				status, err := manager.GetSessionStatus()
				require.NoError(t, err, "GetSessionStatus should succeed")

				assert.Equal(t, test.sessionID, status.SessionID, "SessionID should match")
				assert.Equal(t, test.totalFiles, status.TotalFiles, "TotalFiles should match")
				assert.Equal(t, test.totalBytes, status.TotalBytes, "TotalBytes should match")
				assert.Equal(t, StatusSessionStateActive, status.State, "State should be active")
			}
		})
	}
}

func TestTransferStatusManager_StartFileTransfer(t *testing.T) {
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
			// Create a fresh manager instance for each sub-test to ensure isolation
			manager := NewTransferStatusManager()

			// Initialize session for this specific test
			err := manager.InitializeSession("test-session", 3, 3072)
			require.NoError(t, err, "InitializeSession failed")

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
			}
		})
	}
}

func TestTransferStatusManager_StartFileTransfer_NoSession(t *testing.T) {
	manager := NewTransferStatusManager()

	// Try to start file transfer without initializing session
	_, err := manager.StartFileTransfer("/test/file.txt", 1024)
	require.Error(t, err, "Expected error when starting file transfer without session")
	assert.ErrorIs(t, err, ErrSessionNotFound, "Should return ErrSessionNotFound")
}

func TestTransferStatusManager_StartFileTransfer_ActiveFile(t *testing.T) {
	manager := NewTransferStatusManager()

	// Initialize session
	err := manager.InitializeSession("test-session", 2, 2048)
	require.NoError(t, err, "InitializeSession failed")

	// Start first file transfer
	_, err = manager.StartFileTransfer("/test/file1.txt", 1024)
	require.NoError(t, err, "First StartFileTransfer failed")

	// Try to start second file transfer while first is active
	_, err = manager.StartFileTransfer("/test/file2.txt", 1024)
	require.Error(t, err, "Expected error when starting second file transfer while first is active")
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
	require.NoError(t, err, "InitializeSession failed")

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

// TestTransferStatusManager_UpdateFileProgress tests file progress updates with various scenarios
func TestTransferStatusManager_UpdateFileProgress(t *testing.T) {
	tests := []struct {
		name                 string
		fileSize             int64
		progressUpdates      []int64 // Sequence of progress updates to apply
		expectError          []bool  // Whether each update should cause an error
		expectedFinalBytes   int64   // Expected final bytes sent
		expectedFinalPercent float64 // Expected final progress percentage
		description          string  // Description of what this test verifies
	}{
		{
			name:                 "valid_incremental_progress",
			fileSize:             1000,
			progressUpdates:      []int64{100, 300, 500, 800},
			expectError:          []bool{false, false, false, false},
			expectedFinalBytes:   800,
			expectedFinalPercent: 80.0,
			description:          "Normal incremental progress updates",
		},
		{
			name:                 "progress_to_completion",
			fileSize:             1024,
			progressUpdates:      []int64{512, 1024},
			expectError:          []bool{false, false},
			expectedFinalBytes:   1024,
			expectedFinalPercent: 100.0,
			description:          "Progress updates leading to completion",
		},
		{
			name:                 "zero_progress_update",
			fileSize:             500,
			progressUpdates:      []int64{0},
			expectError:          []bool{false},
			expectedFinalBytes:   0,
			expectedFinalPercent: 0.0,
			description:          "Zero progress update should be valid",
		},
		{
			name:                 "negative_progress_rejected",
			fileSize:             1000,
			progressUpdates:      []int64{100, -50},
			expectError:          []bool{false, true},
			expectedFinalBytes:   100, // Should remain at previous valid value
			expectedFinalPercent: 10.0,
			description:          "Negative progress should be rejected",
		},
		{
			name:                 "progress_exceeding_file_size_allowed",
			fileSize:             1000,
			progressUpdates:      []int64{500, 1500},
			expectError:          []bool{false, false}, // Current implementation allows this
			expectedFinalBytes:   1500,                 // Implementation allows exceeding file size
			expectedFinalPercent: 150.0,
			description:          "Progress exceeding file size is currently allowed",
		},
		{
			name:                 "non_monotonic_progress_allowed",
			fileSize:             1000,
			progressUpdates:      []int64{800, 300},
			expectError:          []bool{false, false}, // Current implementation allows this
			expectedFinalBytes:   300,                  // Implementation allows non-monotonic updates
			expectedFinalPercent: 30.0,
			description:          "Non-monotonic progress is currently allowed",
		},
		{
			name:                 "single_large_update",
			fileSize:             2048,
			progressUpdates:      []int64{2048},
			expectError:          []bool{false},
			expectedFinalBytes:   2048,
			expectedFinalPercent: 100.0,
			description:          "Single update to complete file",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// Create isolated manager for this test case
			manager := NewTransferStatusManager()

			// Initialize session
			err := manager.InitializeSession("test-session", 1, test.fileSize)
			require.NoError(t, err, "InitializeSession should succeed")

			// Start file transfer
			filePath := "/test/file.txt"
			_, err = manager.StartFileTransfer(filePath, test.fileSize)
			require.NoError(t, err, "StartFileTransfer should succeed")

			// Apply progress updates sequentially
			for i, progressBytes := range test.progressUpdates {
				err = manager.UpdateFileProgress(progressBytes)

				if test.expectError[i] {
					assert.Error(t, err, "Progress update %d should fail: %s", i+1, test.description)
				} else {
					assert.NoError(t, err, "Progress update %d should succeed: %s", i+1, test.description)
				}
			}

			// Verify final state
			currentFile, err := manager.GetCurrentFile()
			require.NoError(t, err, "GetCurrentFile should succeed")

			assert.Equal(t, test.expectedFinalBytes, currentFile.BytesSent,
				"Final bytes sent should match expected: %s", test.description)
			assert.Equal(t, test.expectedFinalPercent, currentFile.GetProgressPercentage(),
				"Final progress percentage should match expected: %s", test.description)

			// Verify session progress is consistent
			sessionStatus, err := manager.GetSessionStatus()
			require.NoError(t, err, "GetSessionStatus should succeed")

			expectedSessionProgress := float64(test.expectedFinalBytes) / float64(test.fileSize) * 100.0
			assert.Equal(t, expectedSessionProgress, sessionStatus.OverallProgress,
				"Session overall progress should match file progress: %s", test.description)
		})
	}
}

// TestTransferStatusManager_UpdateFileProgress_ErrorCases tests specific error scenarios
func TestTransferStatusManager_UpdateFileProgress_ErrorCases(t *testing.T) {
	t.Run("no_session_initialized", func(t *testing.T) {
		manager := NewTransferStatusManager()

		err := manager.UpdateFileProgress(100)
		assert.Error(t, err, "Should fail when no session is initialized")
		assert.ErrorIs(t, err, ErrSessionNotFound, "Should return ErrSessionNotFound")
	})

	t.Run("no_active_file", func(t *testing.T) {
		manager := NewTransferStatusManager()

		// Initialize session but don't start any file transfer
		err := manager.InitializeSession("test-session", 1, 1000)
		require.NoError(t, err)

		err = manager.UpdateFileProgress(100)
		assert.Error(t, err, "Should fail when no file transfer is active")
	})

	t.Run("file_already_completed", func(t *testing.T) {
		manager := NewTransferStatusManager()

		// Initialize session and start file transfer
		err := manager.InitializeSession("test-session", 1, 1000)
		require.NoError(t, err)

		_, err = manager.StartFileTransfer("/test.txt", 1000)
		require.NoError(t, err)

		// Complete the file
		err = manager.UpdateFileProgress(1000)
		require.NoError(t, err)

		err = manager.CompleteCurrentFile()
		require.NoError(t, err)

		// Try to update progress on completed file
		err = manager.UpdateFileProgress(500)
		assert.Error(t, err, "Should fail when trying to update completed file")
	})
}

// TestTransferStatusManager_UpdateFileProgress_SequentialUpdates tests realistic progress sequences
func TestTransferStatusManager_UpdateFileProgress_SequentialUpdates(t *testing.T) {
	t.Run("realistic_download_simulation", func(t *testing.T) {
		manager := NewTransferStatusManager()

		// Simulate downloading a 10MB file in chunks
		fileSize := int64(10 * 1024 * 1024) // 10MB
		chunkSize := int64(1024 * 1024)     // 1MB chunks

		err := manager.InitializeSession("download-session", 1, fileSize)
		require.NoError(t, err)

		_, err = manager.StartFileTransfer("/large-file.bin", fileSize)
		require.NoError(t, err)

		// Simulate progressive download
		var totalSent int64
		for i := 0; i < 10; i++ {
			totalSent += chunkSize
			err = manager.UpdateFileProgress(totalSent)
			require.NoError(t, err, "Chunk %d should update successfully", i+1)

			// Verify progress
			currentFile, err := manager.GetCurrentFile()
			require.NoError(t, err)
			assert.Equal(t, totalSent, currentFile.BytesSent)

			expectedPercent := float64(totalSent) / float64(fileSize) * 100.0
			assert.Equal(t, expectedPercent, currentFile.GetProgressPercentage())
		}

		// Verify final state
		assert.Equal(t, fileSize, totalSent)

		currentFile, err := manager.GetCurrentFile()
		require.NoError(t, err)
		assert.Equal(t, 100.0, currentFile.GetProgressPercentage())
	})

	t.Run("non_monotonic_updates_allowed", func(t *testing.T) {
		manager := NewTransferStatusManager()

		err := manager.InitializeSession("test-session", 1, 1000)
		require.NoError(t, err)

		_, err = manager.StartFileTransfer("/test.txt", 1000)
		require.NoError(t, err)

		// Update to 500 bytes
		err = manager.UpdateFileProgress(500)
		require.NoError(t, err)

		// Update to a lower value (current implementation allows this)
		err = manager.UpdateFileProgress(300)
		assert.NoError(t, err, "Current implementation allows non-monotonic progress update")

		// Verify progress was updated to 300
		currentFile, err := manager.GetCurrentFile()
		require.NoError(t, err)
		assert.Equal(t, int64(300), currentFile.BytesSent)
	})
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
	require.NoError(t, err, "StartFileTransfer failed")

	// Complete the transfer
	err = manager.CompleteCurrentFile()
	require.NoError(t, err, "CompleteCurrentFile failed")

	// Verify session status was updated
	sessionStatus, err := manager.GetSessionStatus()
	require.NoError(t, err, "GetSessionStatus failed")

	assert.Equal(t, 1, sessionStatus.CompletedFiles, "Expected CompletedFiles 1")
	assert.Equal(t, 1, sessionStatus.PendingFiles, "Expected PendingFiles 1")

	assert.Equal(t, fileSize, sessionStatus.BytesCompleted, "BytesCompleted should match file size")

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

func TestTransferStatusManager_SessionCompletionLifecycle(t *testing.T) {
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
	manager.ResetSession()

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
