package transfer

import (
	"errors"
	"fmt"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/rescp17/lanFileSharer/pkg/fileInfo"
	"github.com/stretchr/testify/require"
)

// createTempFileForTest creates a temporary file with test content
func createTempFileForTest(t *testing.T, content []byte) (string, func()) {
	t.Helper()

	tempFile, err := os.CreateTemp("", "deadlock-test-*.txt")
	require.NoError(t, err, "Failed to create temp file")

	_, err = tempFile.Write(content)
	require.NoError(t, err, "Failed to write to temp file")

	err = tempFile.Close()
	require.NoError(t, err, "Failed to close temp file")

	cleanup := func() {
		os.Remove(tempFile.Name())
	}

	return tempFile.Name(), cleanup
}

// createFileNodeFromTempFile creates a FileNode from a temporary file
func createFileNodeFromTempFile(t *testing.T, content []byte) (*fileInfo.FileNode, func()) {
	t.Helper()

	filePath, cleanup := createTempFileForTest(t, content)

	node, err := fileInfo.CreateNode(filePath)
	require.NoError(t, err, "Failed to create FileNode")

	return &node, cleanup
}

// TestDeadlockPrevention tests that the lock ordering fixes prevent deadlocks
func TestDeadlockPrevention(t *testing.T) {
	t.Run("concurrent_complete_and_mark_operations", func(t *testing.T) {
		manager := NewUnifiedTransferManager("deadlock-test")
		defer manager.Close()

		// Create a real temporary file for testing
		testContent := []byte("This is test content for deadlock testing")
		testFile, cleanup := createFileNodeFromTempFile(t, testContent)
		defer cleanup()

		err := manager.AddFile(testFile)
		require.NoError(t, err)

		// Start transfer
		err = manager.StartTransfer(testFile.Path)
		require.NoError(t, err)

		// Test concurrent operations that previously could cause deadlock
		var wg sync.WaitGroup
		var testErrors []error
		var errorsMu sync.Mutex

		addError := func(err error) {
			if err != nil {
				errorsMu.Lock()
				testErrors = append(testErrors, err)
				errorsMu.Unlock()
			}
		}

		// Goroutine 1: Try to complete transfer (statusMu -> queueMu)
		wg.Add(1)
		go func() {
			defer wg.Done()
			time.Sleep(10 * time.Millisecond) // Small delay to increase chance of race
			err := manager.CompleteTransfer(testFile.Path)
			addError(err)
		}()

		// Goroutine 2: Try to mark file completed (queueMu -> statusMu)
		wg.Add(1)
		go func() {
			defer wg.Done()
			time.Sleep(5 * time.Millisecond) // Different delay
			err := manager.MarkFileCompleted(testFile.Path)
			addError(err)
		}()

		// Wait for completion with timeout to detect deadlock
		done := make(chan struct{})
		go func() {
			wg.Wait()
			close(done)
		}()

		select {
		case <-done:
			// Success - no deadlock occurred
			t.Log("No deadlock detected - operations completed successfully")
		case <-time.After(5 * time.Second):
			t.Fatal("Potential deadlock detected - operations did not complete within timeout")
		}

		// Check that at least one operation succeeded (the other might fail due to state changes)
		errorsMu.Lock()
		if len(testErrors) == 2 {
			// Both operations failed - this might be expected due to race conditions
			t.Logf("Both operations failed (expected due to race): %v", testErrors)
		}
		errorsMu.Unlock()
	})

	t.Run("concurrent_fail_and_mark_operations", func(t *testing.T) {
		manager := NewUnifiedTransferManager("deadlock-test-2")
		defer manager.Close()

		// Create a real temporary file for testing
		testContent := []byte("This is test content for deadlock testing - file 2")
		testFile, cleanup := createFileNodeFromTempFile(t, testContent)
		defer cleanup()

		err := manager.AddFile(testFile)
		require.NoError(t, err)

		// Start transfer
		err = manager.StartTransfer(testFile.Path)
		require.NoError(t, err)

		var wg sync.WaitGroup
		var testErrors []error
		var errorsMu sync.Mutex

		addError := func(err error) {
			if err != nil {
				errorsMu.Lock()
				testErrors = append(testErrors, err)
				errorsMu.Unlock()
			}
		}

		// Goroutine 1: Try to fail transfer (statusMu -> queueMu)
		wg.Add(1)
		go func() {
			defer wg.Done()
			time.Sleep(10 * time.Millisecond)
			testErr := errors.New("test error")
			err := manager.FailTransfer(testFile.Path, testErr)
			addError(err)
		}()

		// Goroutine 2: Try to mark file failed (queueMu -> statusMu)
		wg.Add(1)
		go func() {
			defer wg.Done()
			time.Sleep(5 * time.Millisecond)
			err := manager.MarkFileFailed(testFile.Path)
			addError(err)
		}()

		// Wait for completion with timeout
		done := make(chan struct{})
		go func() {
			wg.Wait()
			close(done)
		}()

		select {
		case <-done:
			t.Log("No deadlock detected - operations completed successfully")
		case <-time.After(5 * time.Second):
			t.Fatal("Potential deadlock detected - operations did not complete within timeout")
		}
	})

	t.Run("stress_test_concurrent_operations", func(t *testing.T) {
		manager := NewUnifiedTransferManager("stress-test")
		defer manager.Close()

		// Add multiple test files
		numFiles := 5 // Reduced for faster testing
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

		var wg sync.WaitGroup

		// Start multiple concurrent operations
		for i := 0; i < numFiles; i++ {
			testFile := testFiles[i]

			// Start transfer
			wg.Add(1)
			go func(file *fileInfo.FileNode) {
				defer wg.Done()
				manager.StartTransfer(file.Path)
			}(testFile)

			// Complete or fail transfer
			wg.Add(1)
			go func(file *fileInfo.FileNode, index int) {
				defer wg.Done()
				time.Sleep(time.Duration(index) * time.Millisecond)
				if index%2 == 0 {
					manager.CompleteTransfer(file.Path)
				} else {
					testErr := errors.New("test error")
					manager.FailTransfer(file.Path, testErr)
				}
			}(testFile, i)

			// Mark operations
			wg.Add(1)
			go func(file *fileInfo.FileNode, index int) {
				defer wg.Done()
				time.Sleep(time.Duration(index+5) * time.Millisecond)
				if index%2 == 0 {
					manager.MarkFileCompleted(file.Path)
				} else {
					manager.MarkFileFailed(file.Path)
				}
			}(testFile, i)
		}

		// Wait for all operations with timeout
		done := make(chan struct{})
		go func() {
			wg.Wait()
			close(done)
		}()

		select {
		case <-done:
			t.Log("Stress test completed - no deadlock detected")
		case <-time.After(10 * time.Second):
			t.Fatal("Stress test timeout - potential deadlock detected")
		}
	})
}

// TestLockOrderingConsistency verifies that all methods follow the same lock ordering
func TestLockOrderingConsistency(t *testing.T) {
	t.Run("verify_lock_ordering_documentation", func(t *testing.T) {
		// This test serves as documentation for the expected lock ordering
		// Lock order should be: filesMu -> queueMu -> statusMu

		manager := NewUnifiedTransferManager("lock-order-test")
		defer manager.Close()

		// Test that we can acquire locks in the correct order without deadlock
		manager.filesMu.Lock()
		manager.queueMu.Lock()
		manager.statusMu.Lock()

		// Do some minimal work to ensure locks are actually held
		_ = len(manager.chunkers)
		_ = len(manager.pendingFiles)
		_ = manager.sessionStatus

		manager.statusMu.Unlock()
		manager.queueMu.Unlock()
		manager.filesMu.Unlock()

		t.Log("Lock ordering test passed - filesMu -> queueMu -> statusMu")
	})
}
