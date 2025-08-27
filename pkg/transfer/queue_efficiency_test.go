package transfer

import (
	"fmt"
	"testing"
	"time"

	"github.com/rescp17/lanFileSharer/pkg/fileInfo"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestQueueEfficiency tests the efficiency improvements of the new queue implementation
func TestQueueEfficiency(t *testing.T) {
	t.Run("queue_operations_are_O1", func(t *testing.T) {
		manager := NewUnifiedTransferManager("efficiency-test")
		defer manager.Close()

		// Create multiple test files
		numFiles := 1000
		var testFiles []*fileInfo.FileNode
		var cleanupFuncs []func()

		defer func() {
			for _, cleanup := range cleanupFuncs {
				cleanup()
			}
		}()

		// Add many files to test efficiency
		for i := 0; i < numFiles; i++ {
			testContent := []byte(fmt.Sprintf("Test content for file %d", i))
			testFile, cleanup := createFileNodeFromTempFile(t, testContent)
			testFiles = append(testFiles, testFile)
			cleanupFuncs = append(cleanupFuncs, cleanup)

			err := manager.AddFile(testFile)
			require.NoError(t, err)
		}

		// Test that marking files as completed is efficient
		start := time.Now()
		for i := 0; i < numFiles/2; i++ {
			err := manager.MarkFileCompleted(testFiles[i].Path)
			require.NoError(t, err)
		}
		completedTime := time.Since(start)

		// Test that marking files as failed is efficient
		start = time.Now()
		for i := numFiles/2; i < numFiles; i++ {
			err := manager.MarkFileFailed(testFiles[i].Path)
			require.NoError(t, err)
		}
		failedTime := time.Since(start)

		// With O(1) operations, even 1000 files should complete very quickly
		assert.Less(t, completedTime, 100*time.Millisecond, 
			"Marking 500 files as completed should be very fast with O(1) operations")
		assert.Less(t, failedTime, 100*time.Millisecond, 
			"Marking 500 files as failed should be very fast with O(1) operations")

		t.Logf("Completed 500 files in %v", completedTime)
		t.Logf("Failed 500 files in %v", failedTime)
	})

	t.Run("queue_state_consistency", func(t *testing.T) {
		manager := NewUnifiedTransferManager("consistency-test")
		defer manager.Close()

		// Create test files
		testFiles := make([]*fileInfo.FileNode, 3)
		cleanupFuncs := make([]func(), 3)

		defer func() {
			for _, cleanup := range cleanupFuncs {
				cleanup()
			}
		}()

		for i := 0; i < 3; i++ {
			testContent := []byte(fmt.Sprintf("Test content for file %d", i))
			testFile, cleanup := createFileNodeFromTempFile(t, testContent)
			testFiles[i] = testFile
			cleanupFuncs[i] = cleanup

			err := manager.AddFile(testFile)
			require.NoError(t, err)
		}

		// Initially all files should be pending
		manager.queueMu.RLock()
		assert.Equal(t, 3, len(manager.pendingFiles), "Should have 3 pending files")
		assert.Equal(t, 0, len(manager.completedFiles), "Should have 0 completed files")
		assert.Equal(t, 0, len(manager.failedFiles), "Should have 0 failed files")
		
		// Check specific files are in pending state
		for _, testFile := range testFiles {
			assert.True(t, manager.pendingFiles[testFile.Path], 
				"File %s should be in pending state", testFile.Path)
		}
		manager.queueMu.RUnlock()

		// Mark first file as completed
		err := manager.MarkFileCompleted(testFiles[0].Path)
		require.NoError(t, err)

		// Mark second file as failed
		err = manager.MarkFileFailed(testFiles[1].Path)
		require.NoError(t, err)

		// Verify queue states
		manager.queueMu.RLock()
		assert.Equal(t, 1, len(manager.pendingFiles), "Should have 1 pending file")
		assert.Equal(t, 1, len(manager.completedFiles), "Should have 1 completed file")
		assert.Equal(t, 1, len(manager.failedFiles), "Should have 1 failed file")

		// Check specific file states
		assert.True(t, manager.completedFiles[testFiles[0].Path], 
			"File 0 should be in completed state")
		assert.True(t, manager.failedFiles[testFiles[1].Path], 
			"File 1 should be in failed state")
		assert.True(t, manager.pendingFiles[testFiles[2].Path], 
			"File 2 should still be in pending state")

		// Ensure files are not in multiple states
		assert.False(t, manager.pendingFiles[testFiles[0].Path], 
			"File 0 should not be in pending state")
		assert.False(t, manager.pendingFiles[testFiles[1].Path], 
			"File 1 should not be in pending state")
		assert.False(t, manager.completedFiles[testFiles[1].Path], 
			"File 1 should not be in completed state")
		assert.False(t, manager.completedFiles[testFiles[2].Path], 
			"File 2 should not be in completed state")
		assert.False(t, manager.failedFiles[testFiles[0].Path], 
			"File 0 should not be in failed state")
		assert.False(t, manager.failedFiles[testFiles[2].Path], 
			"File 2 should not be in failed state")
		manager.queueMu.RUnlock()
	})

	t.Run("queue_operations_no_duplication", func(t *testing.T) {
		manager := NewUnifiedTransferManager("duplication-test")
		defer manager.Close()

		// Create a test file
		testContent := []byte("Test content for duplication test")
		testFile, cleanup := createFileNodeFromTempFile(t, testContent)
		defer cleanup()

		err := manager.AddFile(testFile)
		require.NoError(t, err)

		// Mark as completed multiple times - should not create duplicates
		err = manager.MarkFileCompleted(testFile.Path)
		require.NoError(t, err)

		err = manager.MarkFileCompleted(testFile.Path)
		require.NoError(t, err) // Should not error, but should not duplicate

		// Verify no duplication
		manager.queueMu.RLock()
		assert.Equal(t, 0, len(manager.pendingFiles), "Should have 0 pending files")
		assert.Equal(t, 1, len(manager.completedFiles), "Should have exactly 1 completed file")
		assert.Equal(t, 0, len(manager.failedFiles), "Should have 0 failed files")
		assert.True(t, manager.completedFiles[testFile.Path], "File should be in completed state")
		manager.queueMu.RUnlock()

		// Try to mark as failed after completion - should move from completed to failed
		err = manager.MarkFileFailed(testFile.Path)
		require.NoError(t, err)

		manager.queueMu.RLock()
		assert.Equal(t, 0, len(manager.pendingFiles), "Should have 0 pending files")
		assert.Equal(t, 0, len(manager.completedFiles), "Should have 0 completed files")
		assert.Equal(t, 1, len(manager.failedFiles), "Should have 1 failed file")
		assert.True(t, manager.failedFiles[testFile.Path], "File should be in failed state")
		assert.False(t, manager.completedFiles[testFile.Path], "File should not be in completed state")
		manager.queueMu.RUnlock()
	})
}

// TestQueueHelperMethods tests the internal queue management helper methods
func TestQueueHelperMethods(t *testing.T) {
	t.Run("moveFileInQueue_operations", func(t *testing.T) {
		manager := NewUnifiedTransferManager("helper-test")
		defer manager.Close()

		testFilePath := "/test/file.txt"

		manager.queueMu.Lock()
		
		// Add file to pending
		manager.addFileToQueue(testFilePath, FileQueueStatePending)
		assert.True(t, manager.isFileInQueue(testFilePath, FileQueueStatePending))
		assert.False(t, manager.isFileInQueue(testFilePath, FileQueueStateCompleted))

		// Move from pending to completed
		moved := manager.moveFileInQueue(testFilePath, FileQueueStatePending, FileQueueStateCompleted)
		assert.True(t, moved, "Should successfully move file")
		assert.False(t, manager.isFileInQueue(testFilePath, FileQueueStatePending))
		assert.True(t, manager.isFileInQueue(testFilePath, FileQueueStateCompleted))

		// Try to move from pending again (should fail since file is not in pending)
		moved = manager.moveFileInQueue(testFilePath, FileQueueStatePending, FileQueueStateFailed)
		assert.False(t, moved, "Should not move file that's not in source state")
		assert.True(t, manager.isFileInQueue(testFilePath, FileQueueStateCompleted))

		// Move from completed to failed
		moved = manager.moveFileInQueue(testFilePath, FileQueueStateCompleted, FileQueueStateFailed)
		assert.True(t, moved, "Should successfully move file")
		assert.False(t, manager.isFileInQueue(testFilePath, FileQueueStateCompleted))
		assert.True(t, manager.isFileInQueue(testFilePath, FileQueueStateFailed))

		// Test queue counts
		pending, completed, failed := manager.getQueueCounts()
		assert.Equal(t, 0, pending)
		assert.Equal(t, 0, completed)
		assert.Equal(t, 1, failed)

		manager.queueMu.Unlock()
	})

	t.Run("getFirstPendingFile_operations", func(t *testing.T) {
		manager := NewUnifiedTransferManager("pending-test")
		defer manager.Close()

		manager.queueMu.Lock()

		// No pending files initially
		filePath, hasPending := manager.getFirstPendingFile()
		assert.False(t, hasPending)
		assert.Equal(t, "", filePath)

		// Add some files
		testFiles := []string{"/test/file1.txt", "/test/file2.txt", "/test/file3.txt"}
		for _, file := range testFiles {
			manager.addFileToQueue(file, FileQueueStatePending)
		}

		// Should return one of the pending files
		filePath, hasPending = manager.getFirstPendingFile()
		assert.True(t, hasPending)
		assert.Contains(t, testFiles, filePath, "Should return one of the pending files")

		manager.queueMu.Unlock()
	})
}
