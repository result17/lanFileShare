package transfer

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestVerifyTestIsolation verifies that our test isolation fixes work correctly
func TestVerifyTestIsolation(t *testing.T) {
	t.Run("isolated_session_initialization", func(t *testing.T) {
		// Test that each sub-test gets a fresh manager instance
		sessionIDs := []string{"session-1", "session-2", "session-3"}
		
		for _, sessionID := range sessionIDs {
			sessionID := sessionID // Capture range variable
			t.Run("session_"+sessionID, func(t *testing.T) {
				t.Parallel() // This should work safely with proper isolation
				
				manager := NewTransferStatusManager()
				
				// Each manager should be able to initialize a session
				err := manager.InitializeSession(sessionID, 1, 1000)
				require.NoError(t, err, "Should be able to initialize session in isolated test")
				
				status, err := manager.GetSessionStatus()
				require.NoError(t, err)
				assert.Equal(t, sessionID, status.SessionID)
				assert.Equal(t, StatusSessionStateActive, status.State)
			})
		}
	})
	
	t.Run("isolated_file_transfers", func(t *testing.T) {
		// Test that file transfer state doesn't leak between sub-tests
		filePaths := []string{"/file1.txt", "/file2.txt", "/file3.txt"}
		
		for i, filePath := range filePaths {
			filePath := filePath // Capture range variable
			i := i
			t.Run("file_"+string(rune('A'+i)), func(t *testing.T) {
				manager := NewTransferStatusManager()
				
				// Initialize session
				err := manager.InitializeSession("test-session", 1, 1000)
				require.NoError(t, err)
				
				// Start file transfer - should work for each isolated test
				status, err := manager.StartFileTransfer(filePath, 1000)
				require.NoError(t, err, "Should be able to start file transfer in isolated test")
				
				assert.Equal(t, filePath, status.FilePath)
				assert.Equal(t, TransferStateActive, status.State)
				assert.Equal(t, int64(1000), status.TotalBytes)
				assert.Equal(t, int64(0), status.BytesSent)
			})
		}
	})
	
	t.Run("isolated_progress_updates", func(t *testing.T) {
		// Test that progress updates don't interfere between sub-tests
		progressValues := []int64{100, 500, 1000}
		
		for _, progress := range progressValues {
			progress := progress // Capture range variable
			t.Run("progress_"+string(rune('0'+progress/100)), func(t *testing.T) {
				manager := NewTransferStatusManager()
				
				// Initialize session and start file transfer
				err := manager.InitializeSession("test-session", 1, 1000)
				require.NoError(t, err)
				
				_, err = manager.StartFileTransfer("/test.txt", 1000)
				require.NoError(t, err)
				
				// Update progress
				err = manager.UpdateFileProgress(progress)
				require.NoError(t, err, "Should be able to update progress in isolated test")
				
				// Verify progress
				currentFile, err := manager.GetCurrentFile()
				require.NoError(t, err)
				assert.Equal(t, progress, currentFile.BytesSent)
				
				expectedPercentage := float64(progress) / 1000.0 * 100.0
				assert.Equal(t, expectedPercentage, currentFile.GetProgressPercentage())
			})
		}
	})
}

// TestOrderIndependence verifies that tests can run in any order
func TestOrderIndependence(t *testing.T) {
	// These tests are designed to run in any order and still pass
	// They test the same functionality but with different data
	
	t.Run("z_test_last_alphabetically", func(t *testing.T) {
		manager := NewTransferStatusManager()
		err := manager.InitializeSession("last-session", 3, 3000)
		require.NoError(t, err)
		
		status, err := manager.GetSessionStatus()
		require.NoError(t, err)
		assert.Equal(t, "last-session", status.SessionID)
		assert.Equal(t, 3, status.TotalFiles)
	})
	
	t.Run("a_test_first_alphabetically", func(t *testing.T) {
		manager := NewTransferStatusManager()
		err := manager.InitializeSession("first-session", 1, 1000)
		require.NoError(t, err)
		
		status, err := manager.GetSessionStatus()
		require.NoError(t, err)
		assert.Equal(t, "first-session", status.SessionID)
		assert.Equal(t, 1, status.TotalFiles)
	})
	
	t.Run("m_test_middle_alphabetically", func(t *testing.T) {
		manager := NewTransferStatusManager()
		err := manager.InitializeSession("middle-session", 2, 2000)
		require.NoError(t, err)
		
		status, err := manager.GetSessionStatus()
		require.NoError(t, err)
		assert.Equal(t, "middle-session", status.SessionID)
		assert.Equal(t, 2, status.TotalFiles)
	})
}

// TestParallelExecution verifies that tests can run in parallel safely
func TestParallelExecution(t *testing.T) {
	// Test data for parallel execution
	testCases := []struct {
		sessionID  string
		totalFiles int
		totalBytes int64
	}{
		{"parallel-1", 1, 1000},
		{"parallel-2", 2, 2000},
		{"parallel-3", 3, 3000},
		{"parallel-4", 4, 4000},
		{"parallel-5", 5, 5000},
	}
	
	for _, tc := range testCases {
		tc := tc // Capture range variable
		t.Run("parallel_"+tc.sessionID, func(t *testing.T) {
			t.Parallel() // Enable parallel execution
			
			manager := NewTransferStatusManager()
			
			err := manager.InitializeSession(tc.sessionID, tc.totalFiles, tc.totalBytes)
			require.NoError(t, err, "Should initialize session in parallel test")
			
			status, err := manager.GetSessionStatus()
			require.NoError(t, err)
			
			assert.Equal(t, tc.sessionID, status.SessionID)
			assert.Equal(t, tc.totalFiles, status.TotalFiles)
			assert.Equal(t, tc.totalBytes, status.TotalBytes)
			assert.Equal(t, StatusSessionStateActive, status.State)
			
			// Simulate some work to increase chance of race conditions if they exist
			for i := 0; i < 10; i++ {
				_, err := manager.GetSessionStatus()
				require.NoError(t, err)
			}
		})
	}
}

// TestStateIsolationVerification explicitly tests that state doesn't leak
func TestStateIsolationVerification(t *testing.T) {
	t.Run("session_state_isolation", func(t *testing.T) {
		// First sub-test creates a session with specific state
		t.Run("create_session_with_files", func(t *testing.T) {
			manager := NewTransferStatusManager()
			
			err := manager.InitializeSession("test-session", 5, 5000)
			require.NoError(t, err)
			
			// Start and complete a file transfer
			_, err = manager.StartFileTransfer("/file1.txt", 1000)
			require.NoError(t, err)
			
			err = manager.UpdateFileProgress(1000)
			require.NoError(t, err)
			
			err = manager.CompleteCurrentFile()
			require.NoError(t, err)
			
			// Verify state
			status, err := manager.GetSessionStatus()
			require.NoError(t, err)
			assert.Equal(t, 1, status.CompletedFiles)
			assert.Equal(t, 4, status.PendingFiles)
		})
		
		// Second sub-test should start with clean state
		t.Run("verify_clean_state", func(t *testing.T) {
			manager := NewTransferStatusManager()
			
			// This should work without any issues from the previous test
			err := manager.InitializeSession("fresh-session", 2, 2000)
			require.NoError(t, err)
			
			status, err := manager.GetSessionStatus()
			require.NoError(t, err)
			
			// Should have clean state, not affected by previous test
			assert.Equal(t, "fresh-session", status.SessionID)
			assert.Equal(t, 2, status.TotalFiles)
			assert.Equal(t, int64(2000), status.TotalBytes)
			assert.Equal(t, 0, status.CompletedFiles)
			assert.Equal(t, 2, status.PendingFiles)
			assert.Equal(t, StatusSessionStateActive, status.State)
		})
	})
}
