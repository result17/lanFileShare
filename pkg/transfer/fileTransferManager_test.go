package transfer

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/rescp17/lanFileSharer/pkg/fileInfo"
)

// setupTestDir creates a temporary directory structure for testing
func setupTestDir(tb testing.TB) (string, func()) {
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
	
	cleanup := func() {
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
	}
	
	return tempDir, cleanup
}

func TestNewFileTransferManager(t *testing.T) {
	ftm := NewFileTransferManager()
	
	if ftm == nil {
		t.Fatal("NewFileTransferManager returned nil")
	}
	
	if ftm.chunkers == nil {
		t.Error("chunkers map not initialized")
	}
	
	if len(ftm.chunkers) != 0 {
		t.Error("chunkers map should be empty initially")
	}
}

func TestFileTransferManager_AddSingleFile(t *testing.T) {
	ftm := NewFileTransferManager()
	
	tempDir, cleanup := setupTestDir(t)
	defer func() {
		// Close file transfer manager first to release file handles
		ftm.Close()
		cleanup()
	}()
	
	filePath := filepath.Join(tempDir, "file1.txt")
	node, err := fileInfo.CreateNode(filePath)
	if err != nil {
		t.Fatalf("Failed to create FileNode: %v", err)
	}
	
	err = ftm.AddFileNode(&node)
	if err != nil {
		t.Fatalf("AddFileNode failed: %v", err)
	}
	
	// Verify chunker was added
	chunker, exists := ftm.GetChunker(filePath)
	if !exists {
		t.Error("Chunker not found after adding file")
	}
	if chunker == nil {
		t.Error("Retrieved chunker is nil")
	}
}

func TestFileTransferManager_AddDirectory(t *testing.T) {
	ftm := NewFileTransferManager()
	
	tempDir, cleanup := setupTestDir(t)
	defer func() {
		ftm.Close()
		cleanup()
	}()
	
	node, err := fileInfo.CreateNode(tempDir)
	if err != nil {
		t.Fatalf("Failed to create FileNode for directory: %v", err)
	}
	
	err = ftm.AddFileNode(&node)
	if err != nil {
		t.Fatalf("AddFileNode failed for directory: %v", err)
	}
	
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
		if !exists {
			t.Errorf("Chunker not found for file: %s", filePath)
		}
		if chunker == nil {
			t.Errorf("Retrieved chunker is nil for file: %s", filePath)
		}
	}
}

func TestFileTransferManager_ReplaceExistingFile(t *testing.T) {
	ftm := NewFileTransferManager()
	
	tempDir, cleanup := setupTestDir(t)
	defer func() {
		ftm.Close()
		cleanup()
	}()
	
	filePath := filepath.Join(tempDir, "file1.txt")
	node, err := fileInfo.CreateNode(filePath)
	if err != nil {
		t.Fatalf("Failed to create FileNode: %v", err)
	}
	
	// Add file first time
	err = ftm.AddFileNode(&node)
	if err != nil {
		t.Fatalf("First AddFileNode failed: %v", err)
	}
	
	firstChunker, _ := ftm.GetChunker(filePath)
	
	// Add same file again (should replace)
	err = ftm.AddFileNode(&node)
	if err != nil {
		t.Fatalf("Second AddFileNode failed: %v", err)
	}
	
	secondChunker, exists := ftm.GetChunker(filePath)
	if !exists {
		t.Error("Chunker not found after replacement")
	}
	
	// Should be different chunker instances
	if firstChunker == secondChunker {
		t.Error("Chunker was not replaced")
	}
}

func TestFileTransferManager_GetChunker(t *testing.T) {
	ftm := NewFileTransferManager()
	
	tempDir, cleanup := setupTestDir(t)
	defer func() {
		ftm.Close()
		cleanup()
	}()
	
	filePath := filepath.Join(tempDir, "file1.txt")
	node, err := fileInfo.CreateNode(filePath)
	if err != nil {
		t.Fatalf("Failed to create FileNode: %v", err)
	}
	
	// Test getting non-existent chunker
	_, exists := ftm.GetChunker(filePath)
	if exists {
		t.Error("GetChunker should return false for non-existent file")
	}
	
	// Add file and test getting existing chunker
	err = ftm.AddFileNode(&node)
	if err != nil {
		t.Fatalf("AddFileNode failed: %v", err)
	}
	
	chunker, exists := ftm.GetChunker(filePath)
	if !exists {
		t.Error("GetChunker should return true for existing file")
	}
	if chunker == nil {
		t.Error("GetChunker should return non-nil chunker")
	}
}

func TestFileTransferManager_Close(t *testing.T) {
	ftm := NewFileTransferManager()
	
	tempDir, cleanup := setupTestDir(t)
	defer cleanup()
	
	// Add some files
	filePaths := []string{
		filepath.Join(tempDir, "file1.txt"),
		filepath.Join(tempDir, "file2.txt"),
	}
	
	for _, filePath := range filePaths {
		node, err := fileInfo.CreateNode(filePath)
		if err != nil {
			t.Fatalf("Failed to create FileNode: %v", err)
		}
		
		err = ftm.AddFileNode(&node)
		if err != nil {
			t.Fatalf("AddFileNode failed: %v", err)
		}
	}
	
	// Verify files were added
	if len(ftm.chunkers) != 2 {
		t.Errorf("Expected 2 chunkers, got %d", len(ftm.chunkers))
	}
	
	// Close manager
	err := ftm.Close()
	if err != nil {
		t.Fatalf("Close failed: %v", err)
	}
	
	// Verify all chunkers were removed
	if len(ftm.chunkers) != 0 {
		t.Errorf("Expected 0 chunkers after close, got %d", len(ftm.chunkers))
	}
	
	// Verify chunkers are no longer accessible
	for _, filePath := range filePaths {
		_, exists := ftm.GetChunker(filePath)
		if exists {
			t.Errorf("Chunker still exists after close: %s", filePath)
		}
	}
}

func TestFileTransferManager_ConcurrentAccess(t *testing.T) {
	ftm := NewFileTransferManager()
	
	tempDir, cleanup := setupTestDir(t)
	defer func() {
		ftm.Close()
		cleanup()
	}()
	
	const numGoroutines = 10
	var wg sync.WaitGroup
	
	// Test concurrent additions
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()
			
			// Create a unique file for this goroutine
			fileName := filepath.Join(tempDir, fmt.Sprintf("concurrent_%d.txt", index))
			content := fmt.Sprintf("Content for file %d", index)
			
			if err := os.WriteFile(fileName, []byte(content), 0644); err != nil {
				t.Errorf("Failed to create concurrent test file: %v", err)
				return
			}
			
			node, err := fileInfo.CreateNode(fileName)
			if err != nil {
				t.Errorf("Failed to create FileNode: %v", err)
				return
			}
			
			err = ftm.AddFileNode(&node)
			if err != nil {
				t.Errorf("AddFileNode failed: %v", err)
				return
			}
		}(i)
	}
	
	wg.Wait()
	
	// Verify all files were added
	if len(ftm.chunkers) < numGoroutines {
		t.Errorf("Expected at least %d chunkers, got %d", numGoroutines, len(ftm.chunkers))
	}
	
	// Test concurrent reads
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()
			
			fileName := filepath.Join(tempDir, fmt.Sprintf("concurrent_%d.txt", index))
			chunker, exists := ftm.GetChunker(fileName)
			if !exists {
				t.Errorf("Chunker not found for concurrent file %d", index)
				return
			}
			if chunker == nil {
				t.Errorf("Retrieved chunker is nil for concurrent file %d", index)
				return
			}
		}(i)
	}
	
	wg.Wait()
}
func TestFileTransferManager_AddFileNode_ErrorHandling(t *testing.T) {
	ftm := NewFileTransferManager()
	defer ftm.Close()
	
	// Test with nil node
	err := ftm.AddFileNode(nil)
	if err == nil {
		t.Error("Expected error when adding nil node")
	}
	
	// Test with non-existent file
	nonExistentNode := &fileInfo.FileNode{
		Name:  "nonexistent.txt",
		IsDir: false,
		Size:  100,
		Path:  "/path/to/nonexistent/file.txt",
	}
	
	err = ftm.AddFileNode(nonExistentNode)
	if err == nil {
		t.Error("Expected error when adding non-existent file")
	}
}

func TestFileTransferManager_ProcessDirConcurrent_ErrorHandling(t *testing.T) {
	ftm := NewFileTransferManager()
	
	// Create a directory node with some invalid children
	tempDir, cleanup := setupTestDir(t)
	defer func() {
		ftm.Close()
		cleanup()
	}()
	
	// Create a valid directory node first
	node, err := fileInfo.CreateNode(tempDir)
	if err != nil {
		t.Fatalf("Failed to create directory node: %v", err)
	}
	
	// Add an invalid child to test error handling
	invalidChild := fileInfo.FileNode{
		Name:  "invalid.txt",
		IsDir: false,
		Size:  100,
		Path:  "/invalid/path/file.txt",
	}
	node.Children = append(node.Children, invalidChild)
	
	err = ftm.AddFileNode(&node)
	if err == nil {
		t.Error("Expected error when processing directory with invalid children")
	}
}

func TestFileTransferManager_EmptyDirectory(t *testing.T) {
	ftm := NewFileTransferManager()
	defer ftm.Close()
	
	// Create empty directory
	tempDir, err := os.MkdirTemp("", "empty-dir-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)
	
	node, err := fileInfo.CreateNode(tempDir)
	if err != nil {
		t.Fatalf("Failed to create FileNode for empty directory: %v", err)
	}
	
	err = ftm.AddFileNode(&node)
	if err != nil {
		t.Fatalf("AddFileNode failed for empty directory: %v", err)
	}
	
	// Should have no chunkers since directory is empty
	if len(ftm.chunkers) != 0 {
		t.Errorf("Expected 0 chunkers for empty directory, got %d", len(ftm.chunkers))
	}
}

func TestFileTransferManager_LargeDirectory(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping large directory test in short mode")
	}
	
	ftm := NewFileTransferManager()
	defer ftm.Close()
	
	// Create directory with many files
	tempDir, err := os.MkdirTemp("", "large-dir-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)
	
	const numFiles = 20
	for i := 0; i < numFiles; i++ {
		fileName := filepath.Join(tempDir, fmt.Sprintf("file_%03d.txt", i))
		content := fmt.Sprintf("Content of file %d", i)
		if err := os.WriteFile(fileName, []byte(content), 0644); err != nil {
			t.Fatalf("Failed to create test file %d: %v", i, err)
		}
	}
	
	node, err := fileInfo.CreateNode(tempDir)
	if err != nil {
		t.Fatalf("Failed to create FileNode for large directory: %v", err)
	}
	
	err = ftm.AddFileNode(&node)
	if err != nil {
		t.Fatalf("AddFileNode failed for large directory: %v", err)
	}
	
	// Verify all files were processed
	if len(ftm.chunkers) != numFiles {
		t.Errorf("Expected %d chunkers, got %d", numFiles, len(ftm.chunkers))
	}
	
	// Verify each file can be retrieved
	for i := 0; i < numFiles; i++ {
		fileName := filepath.Join(tempDir, fmt.Sprintf("file_%03d.txt", i))
		chunker, exists := ftm.GetChunker(fileName)
		if !exists {
			t.Errorf("Chunker not found for file %d", i)
		}
		if chunker == nil {
			t.Errorf("Retrieved chunker is nil for file %d", i)
		}
	}
}

// Benchmark tests
func BenchmarkFileTransferManager_AddSingleFile(b *testing.B) {
	tempDir, cleanup := setupTestDir(b)
	defer cleanup()
	
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
	tempDir, cleanup := setupTestDir(b)
	defer cleanup()
	
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
	
	tempDir, cleanup := setupTestDir(b)
	defer func() {
		ftm.Close()
		cleanup()
	}()
	
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
	
	tempDir, cleanup := setupTestDir(b)
	defer func() {
		ftm.Close()
		cleanup()
	}()
	
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
	
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			// Randomly access different files
			filePath := filePaths[b.N%len(filePaths)]
			_, exists := ftm.GetChunker(filePath)
			if !exists {
				b.Fatal("Chunker not found")
			}
		}
	})
}