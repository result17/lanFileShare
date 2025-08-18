package transfer

import (
	"fmt"
	"testing"
	"time"

	"github.com/rescp17/lanFileSharer/pkg/fileInfo"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestGetFileStatusClarity tests that GetFileStatus returns complete and accurate information
func TestGetFileStatusClarity(t *testing.T) {
	t.Run("completed_file_has_complete_status", func(t *testing.T) {
		manager := NewUnifiedTransferManager("status-clarity-test")
		defer manager.Close()

		// Create a test file with known size
		testContent := []byte("This is test content with known size for status testing")
		expectedSize := int64(len(testContent))
		testFile, cleanup := createFileNodeFromTempFile(t, testContent)
		defer cleanup()

		err := manager.AddFile(testFile)
		require.NoError(t, err)

		// Mark file as completed
		err = manager.MarkFileCompleted(testFile.Path)
		require.NoError(t, err)

		// Get file status
		status, err := manager.GetFileStatus(testFile.Path)
		require.NoError(t, err)
		require.NotNil(t, status)

		// Verify complete and accurate status information
		assert.Equal(t, testFile.Path, status.FilePath, "FilePath should match")
		assert.Equal(t, TransferStateCompleted, status.State, "State should be completed")
		assert.Equal(t, expectedSize, status.TotalBytes, "TotalBytes should match actual file size")
		assert.Equal(t, expectedSize, status.BytesSent, "BytesSent should equal TotalBytes for completed files")
		assert.NotEmpty(t, status.SessionID, "SessionID should be set")
		assert.False(t, status.LastUpdateTime.IsZero(), "LastUpdateTime should be set")

		// For completed files, transfer rate should be calculated if possible
		if status.TransferRate > 0 {
			assert.Greater(t, status.TransferRate, 0.0, "TransferRate should be positive if calculated")
		}

		t.Logf("Completed file status: Size=%d, BytesSent=%d, Rate=%.2f",
			status.TotalBytes, status.BytesSent, status.TransferRate)
	})

	t.Run("failed_file_has_complete_status", func(t *testing.T) {
		manager := NewUnifiedTransferManager("status-clarity-test-2")
		defer manager.Close()

		// Create a test file
		testContent := []byte("This is test content for failed file status testing")
		expectedSize := int64(len(testContent))
		testFile, cleanup := createFileNodeFromTempFile(t, testContent)
		defer cleanup()

		err := manager.AddFile(testFile)
		require.NoError(t, err)

		// Mark file as failed
		err = manager.MarkFileFailed(testFile.Path)
		require.NoError(t, err)

		// Get file status
		status, err := manager.GetFileStatus(testFile.Path)
		require.NoError(t, err)
		require.NotNil(t, status)

		// Verify complete and accurate status information
		assert.Equal(t, testFile.Path, status.FilePath, "FilePath should match")
		assert.Equal(t, TransferStateFailed, status.State, "State should be failed")
		assert.Equal(t, expectedSize, status.TotalBytes, "TotalBytes should match actual file size")
		assert.Equal(t, int64(0), status.BytesSent, "BytesSent should be 0 for failed files")
		assert.NotEmpty(t, status.SessionID, "SessionID should be set")
		assert.False(t, status.LastUpdateTime.IsZero(), "LastUpdateTime should be set")

		t.Logf("Failed file status: Size=%d, BytesSent=%d",
			status.TotalBytes, status.BytesSent)
	})

	t.Run("pending_file_has_complete_status", func(t *testing.T) {
		manager := NewUnifiedTransferManager("status-clarity-test-3")
		defer manager.Close()

		// Create a test file
		testContent := []byte("This is test content for pending file status testing")
		expectedSize := int64(len(testContent))
		testFile, cleanup := createFileNodeFromTempFile(t, testContent)
		defer cleanup()

		err := manager.AddFile(testFile)
		require.NoError(t, err)

		// File should be pending by default
		// Get file status
		status, err := manager.GetFileStatus(testFile.Path)
		require.NoError(t, err)
		require.NotNil(t, status)

		// Verify complete and accurate status information
		assert.Equal(t, testFile.Path, status.FilePath, "FilePath should match")
		assert.Equal(t, TransferStatePending, status.State, "State should be pending")
		assert.Equal(t, expectedSize, status.TotalBytes, "TotalBytes should match actual file size")
		assert.Equal(t, int64(0), status.BytesSent, "BytesSent should be 0 for pending files")
		assert.NotEmpty(t, status.SessionID, "SessionID should be set")
		assert.False(t, status.LastUpdateTime.IsZero(), "LastUpdateTime should be set")

		// For pending files, start time should be set if session has started
		if !status.StartTime.IsZero() {
			assert.True(t, status.StartTime.Before(time.Now()) || status.StartTime.Equal(time.Now()),
				"StartTime should be in the past or now")
		}

		t.Logf("Pending file status: Size=%d, BytesSent=%d",
			status.TotalBytes, status.BytesSent)
	})

	t.Run("active_file_returns_current_status", func(t *testing.T) {
		manager := NewUnifiedTransferManager("status-clarity-test-4")
		defer manager.Close()

		// Create a test file
		testContent := []byte("This is test content for active file status testing")
		testFile, cleanup := createFileNodeFromTempFile(t, testContent)
		defer cleanup()

		err := manager.AddFile(testFile)
		require.NoError(t, err)

		// Start transfer to make it active
		err = manager.StartTransfer(testFile.Path)
		require.NoError(t, err)

		// Get file status
		status, err := manager.GetFileStatus(testFile.Path)
		require.NoError(t, err)
		require.NotNil(t, status)

		// For active files, should return the current transfer status
		assert.Equal(t, testFile.Path, status.FilePath, "FilePath should match")
		assert.Equal(t, TransferStateActive, status.State, "State should be active")
		assert.Greater(t, status.TotalBytes, int64(0), "TotalBytes should be positive")
		assert.NotEmpty(t, status.SessionID, "SessionID should be set")

		t.Logf("Active file status: Size=%d, BytesSent=%d, State=%v",
			status.TotalBytes, status.BytesSent, status.State)
	})

	t.Run("nonexistent_file_returns_error", func(t *testing.T) {
		manager := NewUnifiedTransferManager("status-clarity-test-5")
		defer manager.Close()

		// Try to get status of non-existent file
		status, err := manager.GetFileStatus("/nonexistent/file.txt")
		assert.Error(t, err, "Should return error for non-existent file")
		assert.Equal(t, ErrTransferNotFound, err, "Should return ErrTransferNotFound")
		assert.Nil(t, status, "Status should be nil for non-existent file")
	})
}

// TestGetFileStatusConsistency tests that GetFileStatus is consistent across state changes
func TestGetFileStatusConsistency(t *testing.T) {
	t.Run("status_consistency_through_lifecycle", func(t *testing.T) {
		manager := NewUnifiedTransferManager("consistency-test")
		defer manager.Close()

		// Create a test file
		testContent := []byte("This is test content for lifecycle consistency testing")
		expectedSize := int64(len(testContent))
		testFile, cleanup := createFileNodeFromTempFile(t, testContent)
		defer cleanup()

		err := manager.AddFile(testFile)
		require.NoError(t, err)

		// 1. Check pending status
		status, err := manager.GetFileStatus(testFile.Path)
		require.NoError(t, err)
		assert.Equal(t, TransferStatePending, status.State)
		assert.Equal(t, expectedSize, status.TotalBytes)
		assert.Equal(t, int64(0), status.BytesSent)

		// 2. Start transfer and check active status
		err = manager.StartTransfer(testFile.Path)
		require.NoError(t, err)

		status, err = manager.GetFileStatus(testFile.Path)
		require.NoError(t, err)
		assert.Equal(t, TransferStateActive, status.State)
		assert.Equal(t, expectedSize, status.TotalBytes)

		// 3. Complete transfer and check completed status
		err = manager.CompleteTransfer(testFile.Path)
		require.NoError(t, err)

		status, err = manager.GetFileStatus(testFile.Path)
		require.NoError(t, err)
		assert.Equal(t, TransferStateCompleted, status.State)
		assert.Equal(t, expectedSize, status.TotalBytes)
		assert.Equal(t, expectedSize, status.BytesSent, "Completed file should have all bytes sent")

		// Throughout the lifecycle, TotalBytes should remain consistent
		t.Logf("File lifecycle completed: TotalBytes=%d consistently maintained", expectedSize)
	})

	t.Run("multiple_files_independent_status", func(t *testing.T) {
		manager := NewUnifiedTransferManager("independence-test")
		defer manager.Close()

		// Create multiple test files with different sizes
		files := make([]*fileInfo.FileNode, 3)
		cleanups := make([]func(), 3)
		expectedSizes := make([]int64, 3)

		defer func() {
			for _, cleanup := range cleanups {
				cleanup()
			}
		}()

		for i := 0; i < 3; i++ {
			content := []byte(fmt.Sprintf("Test content for file %d with different size", i))
			expectedSizes[i] = int64(len(content))
			file, cleanup := createFileNodeFromTempFile(t, content)
			files[i] = file
			cleanups[i] = cleanup

			err := manager.AddFile(file)
			require.NoError(t, err)
		}

		// Set different states for each file
		err := manager.MarkFileCompleted(files[0].Path)
		require.NoError(t, err)

		err = manager.MarkFileFailed(files[1].Path)
		require.NoError(t, err)

		// files[2] remains pending

		// Verify each file has correct independent status
		for i, file := range files {
			status, err := manager.GetFileStatus(file.Path)
			require.NoError(t, err, "Should get status for file %d", i)

			assert.Equal(t, expectedSizes[i], status.TotalBytes,
				"File %d should have correct TotalBytes", i)
			assert.Equal(t, file.Path, status.FilePath,
				"File %d should have correct FilePath", i)

			switch i {
			case 0: // Completed
				assert.Equal(t, TransferStateCompleted, status.State)
				assert.Equal(t, expectedSizes[i], status.BytesSent)
			case 1: // Failed
				assert.Equal(t, TransferStateFailed, status.State)
				assert.Equal(t, int64(0), status.BytesSent)
			case 2: // Pending
				assert.Equal(t, TransferStatePending, status.State)
				assert.Equal(t, int64(0), status.BytesSent)
			}

			t.Logf("File %d: State=%v, Size=%d, BytesSent=%d",
				i, status.State, status.TotalBytes, status.BytesSent)
		}
	})
}
