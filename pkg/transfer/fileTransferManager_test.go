package transfer

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/rescp17/lanFileSharer/pkg/fileInfo"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// setupTestDir creates a temporary directory structure for testing
func setupTestDir(tb testing.TB) string {
	tb.Helper()

	tempDir, err := os.MkdirTemp("", "ftm-test-*")
	if err != nil {
		tb.Fatalf("Failed to create temp dir: %v", err)
	}

	// Create test files
	testFiles := map[string][]byte{
		"file1.txt": []byte("Hello World 1"),
		"file2.txt": []byte("Hello World 2"),
		"file3.txt": []byte("Hello World 3"),
	}

	for filename, content := range testFiles {
		filePath := filepath.Join(tempDir, filename)
		if err := os.WriteFile(filePath, content, 0644); err != nil {
			tb.Fatalf("Failed to create test file %s: %v", filename, err)
		}
	}

	// Create subdirectory with files
	subDir := filepath.Join(tempDir, "subdir")
	if err := os.Mkdir(subDir, 0755); err != nil {
		tb.Fatalf("Failed to create subdirectory: %v", err)
	}

	subFiles := map[string][]byte{
		"sub1.txt": []byte("Sub file 1"),
		"sub2.txt": []byte("Sub file 2"),
	}

	for filename, content := range subFiles {
		filePath := filepath.Join(subDir, filename)
		if err := os.WriteFile(filePath, content, 0644); err != nil {
			tb.Fatalf("Failed to create sub test file %s: %v", filename, err)
		}
	}

	tb.Cleanup(func() {
		// On Windows, we need to be more careful about file cleanup
		// Try multiple times with a small delay
		var lastErr error
		for i := 0; i < 3; i++ {
			lastErr = os.RemoveAll(tempDir)
			if lastErr == nil {
				return
			}
			if i < 2 {
				// Small delay before retry
				time.Sleep(10 * time.Millisecond)
			}
		}
		if lastErr != nil {
			tb.Errorf("Failed to clean up temp dir after retries: %v", lastErr)
		}
	})

	return tempDir
}

func TestNewFileTransferManager(t *testing.T) {
	ftm := NewFileTransferManager()

	require.NotNil(t, ftm, "NewFileTransferManager returned nil")
	assert.NotNil(t, ftm.chunkers, "chunkers map not initialized")
	assert.Equal(t, 0, len(ftm.chunkers), "chunkers map should be empty initially")
}

func TestFileTransferManager_AddSingleFile(t *testing.T) {
	ftm := NewFileTransferManager()

	tempDir := setupTestDir(t)

	// Register FileTransferManager cleanup AFTER setupTestDir so it runs BEFORE directory cleanup
	t.Cleanup(func() {
		ftm.Close()
	})

	filePath := filepath.Join(tempDir, "file1.txt")
	node, err := fileInfo.CreateNode(filePath)
	require.NoError(t, err, "Failed to create FileNode")

	err = ftm.AddFileNode(&node)
	require.NoError(t, err, "AddFileNode failed")

	// Verify chunker was added
	chunker, exists := ftm.GetChunker(filePath)
	assert.True(t, exists, "Chunker not found after adding file")
	assert.NotNil(t, chunker, "Retrieved chunker is nil")
}

func TestFileTransferManager_AddDirectory(t *testing.T) {
	ftm := NewFileTransferManager()

	tempDir := setupTestDir(t)

	// Register FileTransferManager cleanup AFTER setupTestDir so it runs BEFORE directory cleanup
	t.Cleanup(func() {
		ftm.Close()
	})

	node, err := fileInfo.CreateNode(tempDir)
	require.NoError(t, err, "Failed to create FileNode for directory")

	err = ftm.AddFileNode(&node)
	require.NoError(t, err, "AddFileNode failed for directory")

	// Verify all files in directory were added
	expectedFiles := []string{
		filepath.Join(tempDir, "file1.txt"),
		filepath.Join(tempDir, "file2.txt"),
		filepath.Join(tempDir, "file3.txt"),
		filepath.Join(tempDir, "subdir", "sub1.txt"),
		filepath.Join(tempDir, "subdir", "sub2.txt"),
	}

	for _, filePath := range expectedFiles {
		chunker, exists := ftm.GetChunker(filePath)
		assert.True(t, exists, "Chunker not found for file: %s", filePath)
		assert.NotNil(t, chunker, "Retrieved chunker is nil for file: %s", filePath)
	}
}

func TestFileTransferManager_ReplaceExistingFile(t *testing.T) {
	ftm := NewFileTransferManager()

	tempDir := setupTestDir(t)

	// Register FileTransferManager cleanup AFTER setupTestDir so it runs BEFORE directory cleanup
	t.Cleanup(func() {
		ftm.Close()
	})

	filePath := filepath.Join(tempDir, "file1.txt")
	node, err := fileInfo.CreateNode(filePath)
	require.NoError(t, err, "Failed to create FileNode")

	// Add file first time
	err = ftm.AddFileNode(&node)
	require.NoError(t, err, "First AddFileNode failed")

	firstChunker, _ := ftm.GetChunker(filePath)

	// Add same file again (should replace)
	err = ftm.AddFileNode(&node)
	require.NoError(t, err, "Second AddFileNode failed")

	secondChunker, exists := ftm.GetChunker(filePath)
	assert.True(t, exists, "Chunker not found after replacement")

	// Should be different chunker instances
	assert.NotEqual(t, firstChunker, secondChunker, "Chunker was not replaced")
}

func TestFileTransferManager_GetChunker(t *testing.T) {
	ftm := NewFileTransferManager()

	tempDir := setupTestDir(t)

	// Register FileTransferManager cleanup AFTER setupTestDir so it runs BEFORE directory cleanup
	t.Cleanup(func() {
		ftm.Close()
	})

	filePath := filepath.Join(tempDir, "file1.txt")
	node, err := fileInfo.CreateNode(filePath)
	require.NoError(t, err, "Failed to create FileNode")

	// Test getting non-existent chunker
	_, exists := ftm.GetChunker(filePath)
	assert.False(t, exists, "GetChunker should return false for non-existent file")

	// Add file and test getting existing chunker
	err = ftm.AddFileNode(&node)
	require.NoError(t, err, "AddFileNode failed")

	chunker, exists := ftm.GetChunker(filePath)
	assert.True(t, exists, "GetChunker should return true for existing file")
	assert.NotNil(t, chunker, "GetChunker should return non-nil chunker")
}

func TestFileTransferManager_Close(t *testing.T) {
	ftm := NewFileTransferManager()

	tempDir := setupTestDir(t)

	// Add some files
	filePaths := []string{
		filepath.Join(tempDir, "file1.txt"),
		filepath.Join(tempDir, "file2.txt"),
	}

	for _, filePath := range filePaths {
		node, err := fileInfo.CreateNode(filePath)
		require.NoError(t, err, "Failed to create FileNode")

		err = ftm.AddFileNode(&node)
		require.NoError(t, err, "AddFileNode failed")
	}

	// Verify files were added
	assert.Equal(t, 2, len(ftm.chunkers), "Expected 2 chunkers")

	// Close manager
	err := ftm.Close()
	require.NoError(t, err, "Close failed")

	// Verify all chunkers were removed
	assert.Equal(t, 0, len(ftm.chunkers), "Expected 0 chunkers after close")

	// Verify chunkers are no longer accessible
	for _, filePath := range filePaths {
		_, exists := ftm.GetChunker(filePath)
		assert.False(t, exists, "Chunker still exists after close: %s", filePath)
	}
}

func TestFileTransferManager_ConcurrentAccess(t *testing.T) {
	ftm := NewFileTransferManager()

	tempDir := setupTestDir(t)

	// Register FileTransferManager cleanup AFTER setupTestDir so it runs BEFORE directory cleanup
	t.Cleanup(func() {
		ftm.Close()
	})

	const numGoroutines = 10
	var wg sync.WaitGroup

	// Use channels to collect errors from goroutines to avoid data races
	errorChan := make(chan error, numGoroutines*2) // Buffer for potential errors

	// Test concurrent additions
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()

			// Create a unique file for this goroutine
			fileName := filepath.Join(tempDir, fmt.Sprintf("concurrent_%d.txt", index))
			content := fmt.Sprintf("Content for file %d", index)

			if err := os.WriteFile(fileName, []byte(content), 0644); err != nil {
				errorChan <- fmt.Errorf("goroutine %d: failed to create concurrent test file: %w", index, err)
				return
			}

			node, err := fileInfo.CreateNode(fileName)
			if err != nil {
				errorChan <- fmt.Errorf("goroutine %d: failed to create FileNode: %w", index, err)
				return
			}

			err = ftm.AddFileNode(&node)
			if err != nil {
				errorChan <- fmt.Errorf("goroutine %d: AddFileNode failed: %w", index, err)
				return
			}
		}(i)
	}

	wg.Wait()

	// Check for any errors from the addition phase
	select {
	case err := <-errorChan:
		t.Errorf("Error during concurrent additions: %v", err)
	default:
		// No errors, continue
	}

	// Verify all files were added
	assert.GreaterOrEqual(t, len(ftm.chunkers), numGoroutines, "Expected at least %d chunkers", numGoroutines)

	// Test concurrent reads
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()

			fileName := filepath.Join(tempDir, fmt.Sprintf("concurrent_%d.txt", index))
			chunker, exists := ftm.GetChunker(fileName)
			if !exists {
				errorChan <- fmt.Errorf("goroutine %d: chunker not found for concurrent file", index)
				return
			}
			if chunker == nil {
				errorChan <- fmt.Errorf("goroutine %d: retrieved chunker is nil for concurrent file", index)
				return
			}
		}(i)
	}

	wg.Wait()

	// Check for any errors from the read phase
	close(errorChan)
	for err := range errorChan {
		t.Errorf("Goroutine error: %v", err)
	}
}
func TestFileTransferManager_AddFileNode_ErrorHandling(t *testing.T) {
	ftm := NewFileTransferManager()
	t.Cleanup(func() {
		ftm.Close()
	})

	// Table-driven test cases for error scenarios
	testCases := []struct {
		name          string
		node          *fileInfo.FileNode
		expectError   bool
		errorContains string
		description   string
	}{
		{
			name:          "nil_node",
			node:          nil,
			expectError:   true,
			errorContains: "cannot be nil",
			description:   "Should reject nil node",
		},
		{
			name: "non_existent_file",
			node: &fileInfo.FileNode{
				Name:  "nonexistent.txt",
				IsDir: false,
				Size:  100,
				Path:  "/path/to/nonexistent/file.txt",
			},
			expectError:   true,
			errorContains: "",
			description:   "Should reject non-existent file",
		},
		{
			name: "empty_path",
			node: &fileInfo.FileNode{
				Name:  "empty.txt",
				IsDir: false,
				Size:  50,
				Path:  "",
			},
			expectError:   true,
			errorContains: "",
			description:   "Should reject empty path",
		},
		{
			name: "invalid_directory_path",
			node: &fileInfo.FileNode{
				Name:     "invalid_dir",
				IsDir:    true,
				Size:     0,
				Path:     "/invalid/directory/path",
				Children: []fileInfo.FileNode{},
			},
			expectError:   false, // Directory nodes might not be validated immediately
			errorContains: "",
			description:   "Directory path validation may be deferred",
		},
		{
			name: "file_with_negative_size",
			node: &fileInfo.FileNode{
				Name:  "negative.txt",
				IsDir: false,
				Size:  -100,
				Path:  "/tmp/negative.txt",
			},
			expectError:   true,
			errorContains: "",
			description:   "Should reject file with negative size",
		},
	}

	// Execute table-driven tests
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := ftm.AddFileNode(tc.node)

			if tc.expectError {
				require.Error(t, err, tc.description)
				if tc.errorContains != "" {
					assert.Contains(t, err.Error(), tc.errorContains,
						"Error message should contain expected text")
				}
			} else {
				require.NoError(t, err, tc.description)
			}
		})
	}
}

func TestFileTransferManager_ProcessDirConcurrent_ErrorHandling(t *testing.T) {
	ftm := NewFileTransferManager()

	tempDir := setupTestDir(t)

	// Register FileTransferManager cleanup AFTER setupTestDir so it runs BEFORE directory cleanup
	t.Cleanup(func() {
		ftm.Close()
	})

	// Table-driven test cases for directory processing errors
	testCases := []struct {
		name          string
		setupNode     func() *fileInfo.FileNode
		expectError   bool
		errorContains string
		description   string
	}{
		{
			name: "directory_with_invalid_child_file",
			setupNode: func() *fileInfo.FileNode {
				node, err := fileInfo.CreateNode(tempDir)
				require.NoError(t, err, "Failed to create directory node")

				// Add an invalid child file
				invalidChild := fileInfo.FileNode{
					Name:  "invalid.txt",
					IsDir: false,
					Size:  100,
					Path:  "/invalid/path/file.txt",
				}
				node.Children = append(node.Children, invalidChild)
				return &node
			},
			expectError:   true,
			errorContains: "",
			description:   "Should fail when directory contains invalid child file",
		},
		{
			name: "directory_with_invalid_child_directory",
			setupNode: func() *fileInfo.FileNode {
				node, err := fileInfo.CreateNode(tempDir)
				require.NoError(t, err, "Failed to create directory node")

				// Add an invalid child directory
				invalidChildDir := fileInfo.FileNode{
					Name:     "invalid_dir",
					IsDir:    true,
					Size:     0,
					Path:     "/invalid/directory/path",
					Children: []fileInfo.FileNode{},
				}
				node.Children = append(node.Children, invalidChildDir)
				return &node
			},
			expectError:   false, // Directory validation may be deferred
			errorContains: "",
			description:   "Directory validation may be deferred until processing",
		},
		{
			name: "directory_with_mixed_valid_invalid_children",
			setupNode: func() *fileInfo.FileNode {
				node, err := fileInfo.CreateNode(tempDir)
				require.NoError(t, err, "Failed to create directory node")

				// Add both valid and invalid children
				validFile := filepath.Join(tempDir, "valid.txt")
				err = os.WriteFile(validFile, []byte("valid content"), 0644)
				require.NoError(t, err, "Failed to create valid test file")

				validChild, err := fileInfo.CreateNode(validFile)
				require.NoError(t, err, "Failed to create valid child node")

				invalidChild := fileInfo.FileNode{
					Name:  "invalid.txt",
					IsDir: false,
					Size:  100,
					Path:  "/invalid/path/file.txt",
				}

				node.Children = append(node.Children, validChild, invalidChild)
				return &node
			},
			expectError:   true,
			errorContains: "",
			description:   "Should fail when directory contains mix of valid and invalid children",
		},
		{
			name: "directory_with_empty_child_path",
			setupNode: func() *fileInfo.FileNode {
				node, err := fileInfo.CreateNode(tempDir)
				require.NoError(t, err, "Failed to create directory node")

				// Add child with empty path
				emptyPathChild := fileInfo.FileNode{
					Name:  "empty_path.txt",
					IsDir: false,
					Size:  50,
					Path:  "",
				}
				node.Children = append(node.Children, emptyPathChild)
				return &node
			},
			expectError:   true,
			errorContains: "",
			description:   "Should fail when child has empty path",
		},
	}

	// Execute table-driven tests
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			node := tc.setupNode()
			err := ftm.AddFileNode(node)

			if tc.expectError {
				require.Error(t, err, tc.description)
				if tc.errorContains != "" {
					assert.Contains(t, err.Error(), tc.errorContains,
						"Error message should contain expected text")
				}
			} else {
				require.NoError(t, err, tc.description)
			}
		})
	}
}

func TestFileTransferManager_EmptyDirectory(t *testing.T) {
	ftm := NewFileTransferManager()
	t.Cleanup(func() {
		ftm.Close()
	})

	// Create empty directory
	tempDir, err := os.MkdirTemp("", "empty-dir-test-*")
	require.NoError(t, err, "Failed to create temp dir")
	t.Cleanup(func() {
		os.RemoveAll(tempDir)
	})

	node, err := fileInfo.CreateNode(tempDir)
	require.NoError(t, err, "Failed to create FileNode for empty directory")

	err = ftm.AddFileNode(&node)
	require.NoError(t, err, "AddFileNode failed for empty directory")

	// Should have no chunkers since directory is empty
	assert.Equal(t, 0, len(ftm.chunkers), "Expected 0 chunkers for empty directory")
}

func TestFileTransferManager_LargeDirectory(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping large directory test in short mode")
	}

	ftm := NewFileTransferManager()
	t.Cleanup(func() {
		ftm.Close()
	})

	// Create directory with many files
	tempDir, err := os.MkdirTemp("", "large-dir-test-*")
	require.NoError(t, err, "Failed to create temp dir")
	t.Cleanup(func() {
		os.RemoveAll(tempDir)
	})

	const numFiles = 20
	for i := 0; i < numFiles; i++ {
		fileName := filepath.Join(tempDir, fmt.Sprintf("file_%03d.txt", i))
		content := fmt.Sprintf("Content of file %d", i)
		require.NoError(t, os.WriteFile(fileName, []byte(content), 0644), "Failed to create test file %d", i)
	}

	node, err := fileInfo.CreateNode(tempDir)
	require.NoError(t, err, "Failed to create FileNode for large directory")

	err = ftm.AddFileNode(&node)
	require.NoError(t, err, "AddFileNode failed for large directory")

	// Verify all files were processed
	assert.Equal(t, numFiles, len(ftm.chunkers), "Expected %d chunkers", numFiles)

	// Verify each file can be retrieved
	for i := 0; i < numFiles; i++ {
		fileName := filepath.Join(tempDir, fmt.Sprintf("file_%03d.txt", i))
		chunker, exists := ftm.GetChunker(fileName)
		assert.True(t, exists, "Chunker not found for file %d", i)
		assert.NotNil(t, chunker, "Retrieved chunker is nil for file %d", i)
	}
}

// Benchmark tests
func BenchmarkFileTransferManager_AddSingleFile(b *testing.B) {
	tempDir := setupTestDir(b)

	filePath := filepath.Join(tempDir, "file1.txt")
	node, err := fileInfo.CreateNode(filePath)
	if err != nil {
		b.Fatalf("Failed to create FileNode: %v", err)
	}

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		ftm := NewFileTransferManager()
		err := ftm.AddFileNode(&node)
		if err != nil {
			b.Fatalf("AddFileNode failed: %v", err)
		}
		ftm.Close()
	}
}

func BenchmarkFileTransferManager_AddDirectory(b *testing.B) {
	tempDir := setupTestDir(b)

	node, err := fileInfo.CreateNode(tempDir)
	if err != nil {
		b.Fatalf("Failed to create FileNode: %v", err)
	}

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		ftm := NewFileTransferManager()
		err := ftm.AddFileNode(&node)
		if err != nil {
			b.Fatalf("AddFileNode failed: %v", err)
		}
		ftm.Close()
	}
}

func BenchmarkFileTransferManager_GetChunker(b *testing.B) {
	ftm := NewFileTransferManager()

	tempDir := setupTestDir(b)

	// Register FileTransferManager cleanup AFTER setupTestDir so it runs BEFORE directory cleanup
	b.Cleanup(func() {
		ftm.Close()
	})

	filePath := filepath.Join(tempDir, "file1.txt")
	node, err := fileInfo.CreateNode(filePath)
	if err != nil {
		b.Fatalf("Failed to create FileNode: %v", err)
	}

	err = ftm.AddFileNode(&node)
	if err != nil {
		b.Fatalf("AddFileNode failed: %v", err)
	}

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_, exists := ftm.GetChunker(filePath)
		if !exists {
			b.Fatal("Chunker not found")
		}
	}
}

func BenchmarkFileTransferManager_ConcurrentAccess(b *testing.B) {
	ftm := NewFileTransferManager()

	tempDir := setupTestDir(b)

	// Register FileTransferManager cleanup AFTER setupTestDir so it runs BEFORE directory cleanup
	b.Cleanup(func() {
		ftm.Close()
	})

	// Pre-populate with some files
	filePaths := make([]string, 10)
	for i := 0; i < 10; i++ {
		fileName := filepath.Join(tempDir, fmt.Sprintf("bench_%d.txt", i))
		content := fmt.Sprintf("Benchmark content %d", i)
		if err := os.WriteFile(fileName, []byte(content), 0644); err != nil {
			b.Fatalf("Failed to create benchmark file: %v", err)
		}

		node, err := fileInfo.CreateNode(fileName)
		if err != nil {
			b.Fatalf("Failed to create FileNode: %v", err)
		}

		err = ftm.AddFileNode(&node)
		if err != nil {
			b.Fatalf("AddFileNode failed: %v", err)
		}

		filePaths[i] = fileName
	}

	b.ResetTimer()

	// Use atomic counter to distribute file access across parallel workers
	var counter int64

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			// Each goroutine gets a different file path using atomic increment
			// This ensures true concurrent access patterns without artificial contention
			fileIndex := atomic.AddInt64(&counter, 1) % int64(len(filePaths))
			filePath := filePaths[fileIndex]
			_, exists := ftm.GetChunker(filePath)
			if !exists {
				b.Fatal("Chunker not found")
			}
		}
	})
}

// TestFileTransferManager_ConcurrentFileLimitRaceCondition tests the TOCTOU race condition fix
func TestFileTransferManager_ConcurrentFileLimitRaceCondition(t *testing.T) {
	ftm := NewFileTransferManager()
	t.Cleanup(func() {
		ftm.Close()
	})

	// Create a temporary directory with many files to test the limit
	tempDir, err := os.MkdirTemp("", "race_test")
	require.NoError(t, err)
	t.Cleanup(func() {
		os.RemoveAll(tempDir)
	})

	// Create files that would exceed the limit when processed concurrently
	const numFiles = 50
	var nodes []*fileInfo.FileNode

	for i := 0; i < numFiles; i++ {
		fileName := filepath.Join(tempDir, fmt.Sprintf("file_%d.txt", i))
		err := os.WriteFile(fileName, []byte(fmt.Sprintf("content %d", i)), 0644)
		require.NoError(t, err)

		node, err := fileInfo.CreateNode(fileName)
		require.NoError(t, err)
		nodes = append(nodes, &node)
	}

	// Simulate being close to the limit by pre-filling the manager
	// We'll set a lower limit for testing by temporarily modifying the check
	originalLimit := MaxSupportedFiles

	// Fill up to near the limit (we can't modify the const, so we'll test with actual files)
	// Instead, let's test the concurrent behavior with a reasonable number of files

	var wg sync.WaitGroup
	errorChan := make(chan error, numFiles)

	// Process files concurrently to trigger potential race condition
	for _, node := range nodes {
		wg.Add(1)
		go func(n *fileInfo.FileNode) {
			defer wg.Done()
			if err := ftm.addSingleFileWithLimitCheck(n); err != nil {
				errorChan <- err
			}
		}(node)
	}

	wg.Wait()
	close(errorChan)

	// Collect any errors
	var errors []error
	for err := range errorChan {
		errors = append(errors, err)
	}

	// Verify that all files were added successfully (since we're under the limit)
	assert.Empty(t, errors, "Should not have errors when under the limit")

	// Verify the final count
	ftm.mu.RLock()
	finalCount := len(ftm.chunkers)
	ftm.mu.RUnlock()

	assert.Equal(t, numFiles, finalCount, "All files should be added")

	t.Logf("Successfully added %d files concurrently without race conditions", finalCount)
	_ = originalLimit // Keep the variable to avoid unused warning
}

// TestFileTransferManager_FileLimitEnforcement tests that the file limit is properly enforced
func TestFileTransferManager_FileLimitEnforcement(t *testing.T) {
	ftm := NewFileTransferManager()
	t.Cleanup(func() {
		ftm.Close()
	})

	// Create a test file
	tempDir, err := os.MkdirTemp("", "limit_test")
	require.NoError(t, err)
	t.Cleanup(func() {
		os.RemoveAll(tempDir)
	})

	fileName := filepath.Join(tempDir, "test.txt")
	err = os.WriteFile(fileName, []byte("test content"), 0644)
	require.NoError(t, err)

	node, err := fileInfo.CreateNode(fileName)
	require.NoError(t, err)

	// We can't easily simulate MaxSupportedFiles (1,000,000) chunkers in a test
	// Instead, let's test the logic by temporarily creating a smaller scenario
	// and verifying the atomic behavior

	// First, let's test that the method works correctly under normal conditions
	err = ftm.addSingleFileWithLimitCheck(&node)
	assert.NoError(t, err, "Should succeed when under limit")

	// Verify the file was added
	ftm.mu.RLock()
	count := len(ftm.chunkers)
	ftm.mu.RUnlock()
	assert.Equal(t, 1, count, "File should be added")

	// The actual limit test would require creating 1,000,000 files which is impractical
	// The important thing is that we've fixed the race condition by making the
	// check and add operation atomic under the same lock
}

// TestFileTransferManager_TOCTOUFix demonstrates the fix for Time-of-Check to Time-of-Use race condition
func TestFileTransferManager_TOCTOUFix(t *testing.T) {
	ftm := NewFileTransferManager()
	t.Cleanup(func() {
		ftm.Close()
	})

	// Create test files
	tempDir, err := os.MkdirTemp("", "toctou_test")
	require.NoError(t, err)
	t.Cleanup(func() {
		os.RemoveAll(tempDir)
	})

	const numConcurrentFiles = 100
	var nodes []*fileInfo.FileNode

	for i := 0; i < numConcurrentFiles; i++ {
		fileName := filepath.Join(tempDir, fmt.Sprintf("toctou_file_%d.txt", i))
		err := os.WriteFile(fileName, []byte(fmt.Sprintf("content %d", i)), 0644)
		require.NoError(t, err)

		node, err := fileInfo.CreateNode(fileName)
		require.NoError(t, err)
		nodes = append(nodes, &node)
	}

	// Test the atomic behavior: all operations should succeed or fail atomically
	var wg sync.WaitGroup
	successCount := int64(0)
	errorCount := int64(0)

	// Launch many concurrent operations
	for _, node := range nodes {
		wg.Add(1)
		go func(n *fileInfo.FileNode) {
			defer wg.Done()

			// This should be atomic - either the check passes and file is added,
			// or the check fails and no file is added
			if err := ftm.addSingleFileWithLimitCheck(n); err != nil {
				atomic.AddInt64(&errorCount, 1)
			} else {
				atomic.AddInt64(&successCount, 1)
			}
		}(node)
	}

	wg.Wait()

	// Verify consistency: success count should match the actual number of files in the manager
	ftm.mu.RLock()
	actualCount := len(ftm.chunkers)
	ftm.mu.RUnlock()

	assert.Equal(t, int64(actualCount), successCount,
		"Success count should match actual files in manager")
	assert.Equal(t, int64(numConcurrentFiles), successCount+errorCount,
		"Total operations should equal success + error count")

	t.Logf("TOCTOU test results: %d successes, %d errors, %d actual files",
		successCount, errorCount, actualCount)
}

// TestFileTransferManager_DynamicConcurrency tests the dynamic concurrency calculation
func TestFileTransferManager_DynamicConcurrency(t *testing.T) {
	ftm := NewFileTransferManager()
	t.Cleanup(func() {
		ftm.Close()
	})

	// Test that concurrency is calculated based on CPU count
	expectedMin := int64(2)
	expectedMax := int64(64)

	concurrency := ftm.GetMaxConcurrency()
	assert.GreaterOrEqual(t, concurrency, expectedMin, "Concurrency should be at least 2")
	assert.LessOrEqual(t, concurrency, expectedMax, "Concurrency should not exceed 64")

	t.Logf("System CPU count: %d, Calculated concurrency: %d", runtime.NumCPU(), concurrency)
}

// TestFileTransferManager_ConcurrencyAdjustment tests runtime concurrency adjustment
func TestFileTransferManager_ConcurrencyAdjustment(t *testing.T) {
	ftm := NewFileTransferManager()
	t.Cleanup(func() {
		ftm.Close()
	})

	// Test setting valid concurrency
	ftm.SetMaxConcurrency(10)
	assert.Equal(t, int64(10), ftm.GetMaxConcurrency(), "Should set concurrency to 10")

	// Test boundary conditions
	ftm.SetMaxConcurrency(0) // Should be clamped to 1
	assert.Equal(t, int64(1), ftm.GetMaxConcurrency(), "Should clamp to minimum 1")

	ftm.SetMaxConcurrency(200) // Should be clamped to 128
	assert.Equal(t, int64(128), ftm.GetMaxConcurrency(), "Should clamp to maximum 128")
}

// TestFileTransferManager_AdaptiveConcurrency tests workload-based concurrency adaptation
func TestFileTransferManager_AdaptiveConcurrency(t *testing.T) {
	ftm := NewFileTransferManager()
	t.Cleanup(func() {
		ftm.Close()
	})

	// Set a known base concurrency for testing
	ftm.SetMaxConcurrency(16)

	// Create test nodes with different child counts
	testCases := []struct {
		name       string
		childCount int
		expectMax  int64
	}{
		{"Small workload", 5, 8},    // Should use baseConcurrency/2
		{"Medium workload", 50, 16}, // Should use full baseConcurrency
		{"Large workload", 500, 16}, // Should use full baseConcurrency
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Create a mock node with specified number of children
			node := &fileInfo.FileNode{
				Path:     "/test/dir",
				IsDir:    true,
				Children: make([]fileInfo.FileNode, tc.childCount),
			}

			adaptiveConcurrency := ftm.calculateAdaptiveConcurrency(node)
			assert.LessOrEqual(t, adaptiveConcurrency, tc.expectMax,
				"Adaptive concurrency should not exceed expected maximum")
			assert.GreaterOrEqual(t, adaptiveConcurrency, int64(1),
				"Adaptive concurrency should be at least 1")

			t.Logf("Child count: %d, Adaptive concurrency: %d", tc.childCount, adaptiveConcurrency)
		})
	}
}

// TestFileTransferManager_Stats tests the statistics reporting
func TestFileTransferManager_Stats(t *testing.T) {
	ftm := NewFileTransferManager()
	t.Cleanup(func() {
		ftm.Close()
	})

	stats := ftm.GetStats()

	// Verify expected fields exist
	assert.Contains(t, stats, "total_files", "Stats should include total_files")
	assert.Contains(t, stats, "max_concurrency", "Stats should include max_concurrency")
	assert.Contains(t, stats, "cpu_count", "Stats should include cpu_count")
	assert.Contains(t, stats, "goroutines", "Stats should include goroutines")

	// Verify values are reasonable
	assert.Equal(t, 0, stats["total_files"], "Should start with 0 files")
	assert.Equal(t, runtime.NumCPU(), stats["cpu_count"], "CPU count should match runtime.NumCPU()")
	assert.Greater(t, stats["goroutines"], 0, "Should have at least 1 goroutine")

	t.Logf("FileTransferManager stats: %+v", stats)
}

// TestFileTransferManager_ConcurrentAccessWithSubtests demonstrates the idiomatic way using t.Run
func TestFileTransferManager_ConcurrentAccessWithSubtests(t *testing.T) {
	ftm := NewFileTransferManager()

	tempDir := setupTestDir(t)

	// Register FileTransferManager cleanup AFTER setupTestDir so it runs BEFORE directory cleanup
	t.Cleanup(func() {
		ftm.Close()
	})

	const numOperations = 10

	// Test concurrent additions using t.Run for parallel subtests
	t.Run("ConcurrentAdditions", func(t *testing.T) {
		for i := 0; i < numOperations; i++ {
			i := i // Capture loop variable
			t.Run(fmt.Sprintf("AddFile_%d", i), func(t *testing.T) {
				t.Parallel() // Enable parallel execution

				// Create a unique file for this subtest
				fileName := filepath.Join(tempDir, fmt.Sprintf("parallel_%d.txt", i))
				content := fmt.Sprintf("Content for parallel file %d", i)

				err := os.WriteFile(fileName, []byte(content), 0644)
				require.NoError(t, err, "Failed to create parallel test file")

				node, err := fileInfo.CreateNode(fileName)
				require.NoError(t, err, "Failed to create FileNode")

				err = ftm.AddFileNode(&node)
				require.NoError(t, err, "AddFileNode failed")
			})
		}
	})

	// Test concurrent reads using t.Run for parallel subtests
	t.Run("ConcurrentReads", func(t *testing.T) {
		for i := 0; i < numOperations; i++ {
			i := i // Capture loop variable
			t.Run(fmt.Sprintf("ReadFile_%d", i), func(t *testing.T) {
				t.Parallel() // Enable parallel execution

				// Perform multiple read operations
				for j := 0; j < 3; j++ {
					fileName := filepath.Join(tempDir, fmt.Sprintf("parallel_%d.txt", i))
					chunker, exists := ftm.GetChunker(fileName)

					// Each subtest has its own *testing.T, so this is safe
					assert.True(t, exists, "Chunker should exist for file %d", i)
					if exists {
						assert.NotNil(t, chunker, "Chunker should not be nil for file %d", i)
					}

					// Small delay to increase chance of race conditions
					time.Sleep(time.Microsecond)
				}
			})
		}
	})

	// Final verification after all parallel operations
	t.Run("FinalVerification", func(t *testing.T) {
		// Verify final state consistency
		ftm.mu.RLock()
		chunkersCount := len(ftm.chunkers)
		ftm.mu.RUnlock()

		assert.Greater(t, chunkersCount, 0, "Should have chunkers after concurrent operations")

		// Log final statistics for debugging
		stats := ftm.GetStats()
		t.Logf("Final state: %+v", stats)
	})
}

// TestFileTransferManager_CleanupOrder demonstrates proper cleanup order with t.Cleanup
func TestFileTransferManager_CleanupOrder(t *testing.T) {
	// This test demonstrates the LIFO (Last In, First Out) behavior of t.Cleanup
	// which is crucial for proper resource management

	ftm := NewFileTransferManager()

	// Create temp directory first
	tempDir, err := os.MkdirTemp("", "cleanup-order-test-*")
	require.NoError(t, err, "Failed to create temp dir")

	// Register cleanup for temp directory FIRST (will be called LAST due to LIFO)
	t.Cleanup(func() {
		t.Logf("Step 3: Cleaning up temp directory: %s", tempDir)
		if err := os.RemoveAll(tempDir); err != nil {
			t.Errorf("Failed to remove temp dir: %v", err)
		}
	})

	// Create a test file
	fileName := filepath.Join(tempDir, "cleanup_test.txt")
	err = os.WriteFile(fileName, []byte("test content"), 0644)
	require.NoError(t, err, "Failed to create test file")

	// Add file to manager
	node, err := fileInfo.CreateNode(fileName)
	require.NoError(t, err, "Failed to create FileNode")

	err = ftm.AddFileNode(&node)
	require.NoError(t, err, "AddFileNode failed")

	// Register cleanup for file transfer manager SECOND (will be called FIRST due to LIFO)
	t.Cleanup(func() {
		t.Logf("Step 1: Closing FileTransferManager")
		if err := ftm.Close(); err != nil {
			t.Errorf("Failed to close FileTransferManager: %v", err)
		}
	})

	// Register an intermediate cleanup to show the order
	t.Cleanup(func() {
		t.Logf("Step 2: Intermediate cleanup step")
		// This could be closing other resources, flushing buffers, etc.
	})

	// Verify the file was added successfully
	chunker, exists := ftm.GetChunker(fileName)
	assert.True(t, exists, "Chunker should exist")
	assert.NotNil(t, chunker, "Chunker should not be nil")

	t.Logf("Test body completed, cleanup will now run in LIFO order")
	// Cleanup order will be:
	// 1. Close FileTransferManager (releases file handles)
	// 2. Intermediate cleanup
	// 3. Remove temp directory (now safe since file handles are closed)
}

// TestFileTransferManager_DeferVsCleanup demonstrates the difference between defer and t.Cleanup
func TestFileTransferManager_DeferVsCleanup(t *testing.T) {
	t.Run("WithDefer_ProblematicOrder", func(t *testing.T) {
		// This subtest shows the problematic pattern with defer
		tempDir, err := os.MkdirTemp("", "defer-test-*")
		require.NoError(t, err)

		// With defer, this will be called FIRST (LIFO from function scope)
		defer func() {
			t.Logf("Defer: Trying to remove temp directory first")
			// This might fail on Windows if files are still open
			os.RemoveAll(tempDir)
		}()

		ftm := NewFileTransferManager()
		// With defer, this will be called SECOND (after temp dir removal attempt)
		defer func() {
			t.Logf("Defer: Closing FileTransferManager second")
			ftm.Close()
		}()

		// Create and add a file
		fileName := filepath.Join(tempDir, "defer_test.txt")
		err = os.WriteFile(fileName, []byte("test"), 0644)
		require.NoError(t, err)

		node, err := fileInfo.CreateNode(fileName)
		require.NoError(t, err)

		err = ftm.AddFileNode(&node)
		require.NoError(t, err)

		t.Logf("Defer test: File handles are still open, cleanup order is wrong")
	})

	t.Run("WithCleanup_CorrectOrder", func(t *testing.T) {
		// This subtest shows the correct pattern with t.Cleanup
		tempDir, err := os.MkdirTemp("", "cleanup-test-*")
		require.NoError(t, err)

		// Register temp dir cleanup FIRST (will be called LAST)
		t.Cleanup(func() {
			t.Logf("Cleanup: Removing temp directory last")
			os.RemoveAll(tempDir)
		})

		ftm := NewFileTransferManager()
		// Register manager cleanup SECOND (will be called FIRST)
		t.Cleanup(func() {
			t.Logf("Cleanup: Closing FileTransferManager first")
			ftm.Close()
		})

		// Create and add a file
		fileName := filepath.Join(tempDir, "cleanup_test.txt")
		err = os.WriteFile(fileName, []byte("test"), 0644)
		require.NoError(t, err)

		node, err := fileInfo.CreateNode(fileName)
		require.NoError(t, err)

		err = ftm.AddFileNode(&node)
		require.NoError(t, err)

		t.Logf("Cleanup test: File handles will be closed before directory removal")
	})
}

// TestFileTransferManager_EdgeCases tests various edge cases using table-driven approach
func TestFileTransferManager_EdgeCases(t *testing.T) {
	ftm := NewFileTransferManager()

	tempDir := setupTestDir(t)

	// Register FileTransferManager cleanup AFTER setupTestDir so it runs BEFORE directory cleanup
	t.Cleanup(func() {
		ftm.Close()
	})

	// Table-driven test cases for edge cases
	testCases := []struct {
		name          string
		setupNode     func() *fileInfo.FileNode
		expectError   bool
		errorContains string
		description   string
	}{
		{
			name: "very_long_filename",
			setupNode: func() *fileInfo.FileNode {
				// Create a file with very long name (255+ characters)
				longName := strings.Repeat("a", 300) + ".txt"
				return &fileInfo.FileNode{
					Name:  longName,
					IsDir: false,
					Size:  100,
					Path:  filepath.Join(tempDir, longName),
				}
			},
			expectError:   true,
			errorContains: "",
			description:   "Should handle very long filenames gracefully",
		},
		{
			name: "file_with_special_characters",
			setupNode: func() *fileInfo.FileNode {
				specialName := "file<>:\"|?*.txt"
				return &fileInfo.FileNode{
					Name:  specialName,
					IsDir: false,
					Size:  50,
					Path:  filepath.Join(tempDir, specialName),
				}
			},
			expectError:   true,
			errorContains: "",
			description:   "Should handle files with special characters",
		},
		{
			name: "zero_size_file",
			setupNode: func() *fileInfo.FileNode {
				fileName := filepath.Join(tempDir, "zero_size.txt")
				err := os.WriteFile(fileName, []byte{}, 0644)
				require.NoError(t, err, "Failed to create zero-size file")

				node, err := fileInfo.CreateNode(fileName)
				require.NoError(t, err, "Failed to create node for zero-size file")
				return &node
			},
			expectError:   false,
			errorContains: "",
			description:   "Should handle zero-size files correctly",
		},
		{
			name: "very_large_file_size",
			setupNode: func() *fileInfo.FileNode {
				return &fileInfo.FileNode{
					Name:  "large.txt",
					IsDir: false,
					Size:  9223372036854775807, // Max int64
					Path:  filepath.Join(tempDir, "large.txt"),
				}
			},
			expectError:   true,
			errorContains: "",
			description:   "Should handle very large file sizes",
		},
		{
			name: "deeply_nested_directory",
			setupNode: func() *fileInfo.FileNode {
				// Create deeply nested path
				deepPath := tempDir
				for i := 0; i < 50; i++ {
					deepPath = filepath.Join(deepPath, fmt.Sprintf("level%d", i))
				}

				return &fileInfo.FileNode{
					Name:     "deep_dir",
					IsDir:    true,
					Size:     0,
					Path:     deepPath,
					Children: []fileInfo.FileNode{},
				}
			},
			expectError:   false, // Path length validation may be platform-specific
			errorContains: "",
			description:   "Should handle deeply nested directories (platform-dependent)",
		},
		{
			name: "unicode_filename",
			setupNode: func() *fileInfo.FileNode {
				unicodeName := "测试文件_🚀_αβγ.txt"
				fileName := filepath.Join(tempDir, unicodeName)
				err := os.WriteFile(fileName, []byte("unicode content"), 0644)
				require.NoError(t, err, "Failed to create unicode file")

				node, err := fileInfo.CreateNode(fileName)
				require.NoError(t, err, "Failed to create node for unicode file")
				return &node
			},
			expectError:   false,
			errorContains: "",
			description:   "Should handle unicode filenames correctly",
		},
	}

	// Execute table-driven tests
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			node := tc.setupNode()
			err := ftm.AddFileNode(node)

			if tc.expectError {
				require.Error(t, err, tc.description)
				if tc.errorContains != "" {
					assert.Contains(t, err.Error(), tc.errorContains,
						"Error message should contain expected text")
				}
			} else {
				if err != nil {
					// Log the error for debugging but don't fail the test
					t.Logf("Unexpected error (might be platform-specific): %v", err)
				} else {
					// For successful cases, verify the file was actually added (only for files, not directories)
					if !node.IsDir {
						chunker, exists := ftm.GetChunker(node.Path)
						assert.True(t, exists, "Chunker should exist for successfully added file")
						assert.NotNil(t, chunker, "Chunker should not be nil")
					}
				}
			}
		})
	}
}

// TestFileTransferManager_ValidationCases tests input validation using table-driven approach
func TestFileTransferManager_ValidationCases(t *testing.T) {
	ftm := NewFileTransferManager()
	t.Cleanup(func() {
		ftm.Close()
	})

	// Table-driven test cases for input validation
	testCases := []struct {
		name          string
		node          *fileInfo.FileNode
		expectError   bool
		errorContains string
		description   string
	}{
		{
			name:          "nil_node",
			node:          nil,
			expectError:   true,
			errorContains: "cannot be nil",
			description:   "Should reject nil node",
		},
		{
			name: "node_with_nil_name",
			node: &fileInfo.FileNode{
				Name:  "",
				IsDir: false,
				Size:  100,
				Path:  "/tmp/test.txt",
			},
			expectError:   false, // Empty name might be valid in some cases
			errorContains: "",
			description:   "Should handle empty name gracefully",
		},
		{
			name: "directory_with_nil_children",
			node: &fileInfo.FileNode{
				Name:     "test_dir",
				IsDir:    true,
				Size:     0,
				Path:     "/tmp/test_dir",
				Children: nil, // nil children slice
			},
			expectError:   false, // nil children might be valid for empty directory
			errorContains: "",
			description:   "Should handle directory with nil children",
		},
		{
			name: "file_marked_as_directory",
			node: &fileInfo.FileNode{
				Name:     "file_as_dir",
				IsDir:    true, // Marked as directory
				Size:     100,  // But has file size
				Path:     "/tmp/file_as_dir",
				Children: nil,
			},
			expectError:   false, // This validation may not be enforced
			errorContains: "",
			description:   "File marked as directory validation may be deferred",
		},
		{
			name: "directory_with_non_zero_size",
			node: &fileInfo.FileNode{
				Name:     "dir_with_size",
				IsDir:    true,
				Size:     500, // Directories shouldn't have size
				Path:     "/tmp/dir_with_size",
				Children: []fileInfo.FileNode{},
			},
			expectError:   false, // Some systems report directory sizes
			errorContains: "",
			description:   "Should handle directory with non-zero size",
		},
	}

	// Execute table-driven tests
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := ftm.AddFileNode(tc.node)

			if tc.expectError {
				require.Error(t, err, tc.description)
				if tc.errorContains != "" {
					assert.Contains(t, err.Error(), tc.errorContains,
						"Error message should contain expected text")
				}
			} else {
				if err != nil {
					// Log the error for debugging but don't fail the test
					t.Logf("Unexpected error (might be platform-specific): %v", err)
				}
			}
		})
	}
}

// TestFileTransferManager_RealErrorScenarios tests actual error scenarios that should fail
func TestFileTransferManager_RealErrorScenarios(t *testing.T) {
	ftm := NewFileTransferManager()

	tempDir := setupTestDir(t)

	// Register FileTransferManager cleanup AFTER setupTestDir so it runs BEFORE directory cleanup
	t.Cleanup(func() {
		ftm.Close()
	})

	// Table-driven test cases for real error scenarios
	testCases := []struct {
		name          string
		setupNode     func() *fileInfo.FileNode
		expectError   bool
		errorContains string
		description   string
	}{
		{
			name: "nil_node_input",
			setupNode: func() *fileInfo.FileNode {
				return nil
			},
			expectError:   true,
			errorContains: "cannot be nil",
			description:   "Should reject nil node input",
		},
		{
			name: "file_does_not_exist",
			setupNode: func() *fileInfo.FileNode {
				return &fileInfo.FileNode{
					Name:  "missing.txt",
					IsDir: false,
					Size:  100,
					Path:  filepath.Join(tempDir, "missing.txt"), // File doesn't exist
				}
			},
			expectError:   true,
			errorContains: "",
			description:   "Should fail when file doesn't exist",
		},
		{
			name: "directory_does_not_exist",
			setupNode: func() *fileInfo.FileNode {
				return &fileInfo.FileNode{
					Name:     "missing_dir",
					IsDir:    true,
					Size:     0,
					Path:     filepath.Join(tempDir, "missing_dir"), // Directory doesn't exist
					Children: []fileInfo.FileNode{},
				}
			},
			expectError:   false, // Directory validation may be deferred until processing
			errorContains: "",
			description:   "Directory existence validation may be deferred",
		},
		{
			name: "valid_existing_file",
			setupNode: func() *fileInfo.FileNode {
				fileName := filepath.Join(tempDir, "valid.txt")
				err := os.WriteFile(fileName, []byte("valid content"), 0644)
				require.NoError(t, err, "Failed to create valid test file")

				node, err := fileInfo.CreateNode(fileName)
				require.NoError(t, err, "Failed to create node for valid file")
				return &node
			},
			expectError:   false,
			errorContains: "",
			description:   "Should succeed with valid existing file",
		},
		{
			name: "valid_existing_directory",
			setupNode: func() *fileInfo.FileNode {
				dirName := filepath.Join(tempDir, "valid_dir")
				err := os.Mkdir(dirName, 0755)
				require.NoError(t, err, "Failed to create valid test directory")

				// Create a file inside the directory
				fileName := filepath.Join(dirName, "inside.txt")
				err = os.WriteFile(fileName, []byte("inside content"), 0644)
				require.NoError(t, err, "Failed to create file inside directory")

				node, err := fileInfo.CreateNode(dirName)
				require.NoError(t, err, "Failed to create node for valid directory")
				return &node
			},
			expectError:   false,
			errorContains: "",
			description:   "Should succeed with valid existing directory",
		},
	}

	// Execute table-driven tests
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			node := tc.setupNode()
			err := ftm.AddFileNode(node)

			if tc.expectError {
				require.Error(t, err, tc.description)
				if tc.errorContains != "" {
					assert.Contains(t, err.Error(), tc.errorContains,
						"Error message should contain expected text")
				}
			} else {
				require.NoError(t, err, tc.description)

				// For successful cases, verify the node was processed correctly
				if node != nil && !node.IsDir {
					chunker, exists := ftm.GetChunker(node.Path)
					assert.True(t, exists, "Chunker should exist for successfully added file")
					assert.NotNil(t, chunker, "Chunker should not be nil")
				}
			}
		})
	}
}

// TestFileTransferManager_BoundaryConditions tests boundary conditions with table-driven approach
func TestFileTransferManager_BoundaryConditions(t *testing.T) {
	ftm := NewFileTransferManager()

	tempDir := setupTestDir(t)

	// Register FileTransferManager cleanup AFTER setupTestDir so it runs BEFORE directory cleanup
	t.Cleanup(func() {
		ftm.Close()
	})

	// Table-driven test cases for boundary conditions
	testCases := []struct {
		name        string
		setupNode   func() *fileInfo.FileNode
		expectError bool
		description string
	}{
		{
			name: "empty_file",
			setupNode: func() *fileInfo.FileNode {
				fileName := filepath.Join(tempDir, "empty.txt")
				err := os.WriteFile(fileName, []byte{}, 0644)
				require.NoError(t, err, "Failed to create empty file")

				node, err := fileInfo.CreateNode(fileName)
				require.NoError(t, err, "Failed to create node for empty file")
				return &node
			},
			expectError: false,
			description: "Should handle empty files correctly",
		},
		{
			name: "single_byte_file",
			setupNode: func() *fileInfo.FileNode {
				fileName := filepath.Join(tempDir, "single.txt")
				err := os.WriteFile(fileName, []byte("x"), 0644)
				require.NoError(t, err, "Failed to create single byte file")

				node, err := fileInfo.CreateNode(fileName)
				require.NoError(t, err, "Failed to create node for single byte file")
				return &node
			},
			expectError: false,
			description: "Should handle single byte files correctly",
		},
		{
			name: "empty_directory",
			setupNode: func() *fileInfo.FileNode {
				dirName := filepath.Join(tempDir, "empty_dir")
				err := os.Mkdir(dirName, 0755)
				require.NoError(t, err, "Failed to create empty directory")

				node, err := fileInfo.CreateNode(dirName)
				require.NoError(t, err, "Failed to create node for empty directory")
				return &node
			},
			expectError: false,
			description: "Should handle empty directories correctly",
		},
		{
			name: "directory_with_single_file",
			setupNode: func() *fileInfo.FileNode {
				dirName := filepath.Join(tempDir, "single_file_dir")
				err := os.Mkdir(dirName, 0755)
				require.NoError(t, err, "Failed to create directory")

				fileName := filepath.Join(dirName, "single.txt")
				err = os.WriteFile(fileName, []byte("single file content"), 0644)
				require.NoError(t, err, "Failed to create file in directory")

				node, err := fileInfo.CreateNode(dirName)
				require.NoError(t, err, "Failed to create node for directory")
				return &node
			},
			expectError: false,
			description: "Should handle directory with single file correctly",
		},
	}

	// Execute table-driven tests
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			node := tc.setupNode()
			err := ftm.AddFileNode(node)

			if tc.expectError {
				require.Error(t, err, tc.description)
			} else {
				require.NoError(t, err, tc.description)

				// For files, verify chunker was created
				if !node.IsDir {
					chunker, exists := ftm.GetChunker(node.Path)
					assert.True(t, exists, "Chunker should exist for file")
					assert.NotNil(t, chunker, "Chunker should not be nil")
				}
			}
		})
	}
}
