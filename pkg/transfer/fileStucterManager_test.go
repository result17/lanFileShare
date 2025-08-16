package transfer

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/rescp17/lanFileSharer/pkg/fileInfo"
)

// TestNewFileStructureManager tests the initialization of FileStructureManager
func TestNewFileStructureManager(t *testing.T) {
	fsm := NewFileStructureManager()

	// Verify the manager was created successfully
	if fsm == nil {
		t.Fatal("NewFileStructureManager returned nil")
	}

	// Verify all fields are properly initialized
	if fsm.RootNodes == nil {
		t.Error("RootNodes slice not initialized")
	}

	if fsm.fileMap == nil {
		t.Error("fileMap not initialized")
	}

	if fsm.dirMap == nil {
		t.Error("dirMap not initialized")
	}

	// Verify initial state is empty
	if len(fsm.RootNodes) != 0 {
		t.Error("RootNodes should be empty initially")
	}

	if fsm.GetFileCount() != 0 {
		t.Error("File count should be 0 initially")
	}

	if fsm.GetDirCount() != 0 {
		t.Error("Directory count should be 0 initially")
	}

	if fsm.GetTotalSize() != 0 {
		t.Error("Total size should be 0 initially")
	}
}

// TestFileStructureManager_AddSingleFile tests adding a single file
func TestFileStructureManager_AddSingleFile(t *testing.T) {
	fsm := NewFileStructureManager()

	// Create test file
	tempDir, cleanup := setupTestDir(t)
	defer cleanup()

	filePath := filepath.Join(tempDir, "file1.txt")
	node, err := fileInfo.CreateNode(filePath)
	if err != nil {
		t.Fatalf("Failed to create FileNode: %v", err)
	}

	// Add the file to the manager
	fsm.AddFileNode(&node)

	// Verify file count increased
	if fsm.GetFileCount() != 1 {
		t.Errorf("Expected 1 file, got %d", fsm.GetFileCount())
	}

	// Verify no directories were added
	if fsm.GetDirCount() != 0 {
		t.Errorf("Expected 0 directories, got %d", fsm.GetDirCount())
	}

	// Verify the file can be retrieved
	retrievedNode, exists := fsm.GetFile(node.Path)
	if !exists {
		t.Error("File not found in fileMap")
	}

	if retrievedNode == nil {
		t.Fatal("Retrieved node is nil")
	}

	if retrievedNode.Name != node.Name {
		t.Errorf("Expected file name %s, got %s", node.Name, retrievedNode.Name)
	}

	if retrievedNode.Size != node.Size {
		t.Errorf("Expected file size %d, got %d", node.Size, retrievedNode.Size)
	}

	// Verify total size calculation
	expectedSize := node.Size
	if fsm.GetTotalSize() != expectedSize {
		t.Errorf("Expected total size %d, got %d", expectedSize, fsm.GetTotalSize())
	}
}

// TestFileStructureManager_AddDirectory tests adding a directory structure
func TestFileStructureManager_AddDirectory(t *testing.T) {
	fsm := NewFileStructureManager()

	// Create test directory structure
	tempDir, cleanup := setupTestDir(t)
	defer cleanup()

	node, err := fileInfo.CreateNode(tempDir)
	if err != nil {
		t.Fatalf("Failed to create FileNode for directory: %v", err)
	}

	// Add the entire directory structure
	fsm.AddFileNode(&node)

	// Verify file count (3 files in root + 2 files in subdir = 5 total)
	expectedFiles := 5
	if fsm.GetFileCount() != expectedFiles {
		t.Errorf("Expected %d files, got %d", expectedFiles, fsm.GetFileCount())
	}

	// Verify directory count (root dir + subdir = 2 total)
	expectedDirs := 2
	if fsm.GetDirCount() != expectedDirs {
		t.Errorf("Expected %d directories, got %d", expectedDirs, fsm.GetDirCount())
	}

	// Verify root directory exists
	_, exists := fsm.GetDir(tempDir)
	if !exists {
		t.Error("Root directory not found in dirMap")
	}

	// Verify subdirectory exists
	subDirPath := filepath.Join(tempDir, "subdir")
	_, exists = fsm.GetDir(subDirPath)
	if !exists {
		t.Error("Subdirectory not found in dirMap")
	}

	// Verify all expected files exist
	expectedFilePaths := []string{
		filepath.Join(tempDir, "file1.txt"),
		filepath.Join(tempDir, "file2.txt"),
		filepath.Join(tempDir, "file3.txt"),
		filepath.Join(tempDir, "subdir", "sub1.txt"),
		filepath.Join(tempDir, "subdir", "sub2.txt"),
	}

	for _, filePath := range expectedFilePaths {
		_, exists := fsm.GetFile(filePath)
		if !exists {
			t.Errorf("Expected file not found: %s", filePath)
		}
	}

	// Verify total size is positive
	totalSize := fsm.GetTotalSize()
	if totalSize <= 0 {
		t.Errorf("Expected positive total size, got %d", totalSize)
	}
}

// TestFileStructureManager_GetAllFiles tests retrieving all files
func TestFileStructureManager_GetAllFiles(t *testing.T) {
	fsm := NewFileStructureManager()

	tempDir, cleanup := setupTestDir(t)
	defer cleanup()

	node, err := fileInfo.CreateNode(tempDir)
	if err != nil {
		t.Fatalf("Failed to create FileNode: %v", err)
	}

	fsm.AddFileNode(&node)

	// Get all files
	files := fsm.GetAllFiles()

	// Verify count matches
	if len(files) != fsm.GetFileCount() {
		t.Errorf("GetAllFiles returned %d files, but GetFileCount returned %d",
			len(files), fsm.GetFileCount())
	}

	// Verify all files are valid
	for i, file := range files {
		if file == nil {
			t.Errorf("File at index %d is nil", i)
			continue
		}

		if file.IsDir {
			t.Errorf("GetAllFiles returned a directory: %s", file.Path)
		}

		if file.Size <= 0 {
			t.Errorf("File %s has invalid size: %d", file.Path, file.Size)
		}
	}
}

// TestFileStructureManager_EmptyDirectory tests handling of empty directories
func TestFileStructureManager_EmptyDirectory(t *testing.T) {
	fsm := NewFileStructureManager()

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

	fsm.AddFileNode(&node)

	// Verify empty directory is recorded
	if fsm.GetDirCount() != 1 {
		t.Errorf("Expected 1 directory, got %d", fsm.GetDirCount())
	}

	if fsm.GetFileCount() != 0 {
		t.Errorf("Expected 0 files, got %d", fsm.GetFileCount())
	}

	// Verify the empty directory can be retrieved
	retrievedDir, exists := fsm.GetDir(node.Path)
	if !exists {
		t.Error("Empty directory not found in dirMap")
	}

	if retrievedDir == nil {
		t.Fatal("Retrieved directory node is nil")
	}

	if !retrievedDir.IsDir {
		t.Error("Retrieved node is not marked as directory")
	}

	// Verify total size is zero
	if fsm.GetTotalSize() != 0 {
		t.Errorf("Expected total size 0 for empty directory, got %d", fsm.GetTotalSize())
	}
}

// TestFileStructureManager_Clear tests clearing all data
func TestFileStructureManager_Clear(t *testing.T) {
	fsm := NewFileStructureManager()

	// Add some data first
	tempDir, cleanup := setupTestDir(t)
	defer cleanup()

	node, err := fileInfo.CreateNode(tempDir)
	if err != nil {
		t.Fatalf("Failed to create FileNode: %v", err)
	}

	fsm.AddFileNode(&node)

	// Verify data was added
	if fsm.GetFileCount() == 0 {
		t.Fatal("No files were added before clear test")
	}

	if fsm.GetDirCount() == 0 {
		t.Fatal("No directories were added before clear test")
	}

	// Clear all data
	fsm.Clear()

	// Verify everything is cleared
	if fsm.GetFileCount() != 0 {
		t.Errorf("Expected 0 files after clear, got %d", fsm.GetFileCount())
	}

	if fsm.GetDirCount() != 0 {
		t.Errorf("Expected 0 directories after clear, got %d", fsm.GetDirCount())
	}

	if len(fsm.RootNodes) != 0 {
		t.Errorf("Expected 0 root nodes after clear, got %d", len(fsm.RootNodes))
	}

	if fsm.GetTotalSize() != 0 {
		t.Errorf("Expected 0 total size after clear, got %d", fsm.GetTotalSize())
	}
}

// TestFileStructureManager_ConcurrentAccess tests thread safety
func TestFileStructureManager_ConcurrentAccess(t *testing.T) {

	tempDir, cleanup := setupTestDir(t)
	defer cleanup()

	fsm, err := NewFileStructureManagerFromPath(tempDir)

	if err != nil {
		t.Fatalf("Failed to create FileStructureManager: %v", err)
	}

	const numWriters = 5
	const numReaders = 3
	var wg sync.WaitGroup

	// Concurrent writers - add files
	for i := 0; i < numWriters; i++ {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()

			// Create unique test file
			fileName := filepath.Join(tempDir, fmt.Sprintf("concurrent_%d.txt", index))
			content := fmt.Sprintf("Content for concurrent file %d", index)

			if err := os.WriteFile(fileName, []byte(content), 0644); err != nil {
				t.Errorf("Failed to create concurrent test file: %v", err)
				return
			}

			node, err := fileInfo.CreateNode(fileName)
			if err != nil {
				t.Errorf("Failed to create FileNode: %v", err)
				return
			}

			fsm.AddFileNode(&node)
		}(i)
	}

	// Concurrent readers - read statistics and verify consistency
	for i := 0; i < numReaders; i++ {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()

			// Perform multiple read operations
			fileCount := fsm.GetFileCount()
			dirCount := fsm.GetDirCount()
			totalSize := fsm.GetTotalSize()
			allFiles := fsm.GetAllFiles()

			// Verify read consistency
			if fileCount < 0 {
				t.Errorf("Reader %d: Invalid file count: %d", index, fileCount)
			}

			if dirCount < 0 {
				t.Errorf("Reader %d: Invalid directory count: %d", index, dirCount)
			}

			if totalSize < 0 {
				t.Errorf("Reader %d: Invalid total size: %d", index, totalSize)
			}

			if len(allFiles) != fileCount {
				t.Errorf("Reader %d: Inconsistent file count: GetFileCount()=%d, len(GetAllFiles())=%d",
					index, fileCount, len(allFiles))
			}
		}(i)
	}

	wg.Wait()

	// Verify final state
	// Should have at least the concurrent files plus the original setupTestDir files
	minExpectedFiles := numWriters + 5 // 5 from setupTestDir
	if fsm.GetFileCount() < minExpectedFiles {
		t.Errorf("Expected at least %d files, got %d", minExpectedFiles, fsm.GetFileCount())
	}

	// Verify all concurrent files were added
	for i := 0; i < numWriters; i++ {
		fileName := filepath.Join(tempDir, fmt.Sprintf("concurrent_%d.txt", i))
		_, exists := fsm.GetFile(fileName)
		if !exists {
			t.Errorf("Concurrent file %d not found: %s", i, fileName)
		}
	}
}

// TestFileStructureManager_DuplicateFiles tests handling of duplicate file additions
func TestFileStructureManager_DuplicateFiles(t *testing.T) {
	fsm := NewFileStructureManager()

	tempDir, cleanup := setupTestDir(t)
	defer cleanup()

	filePath := filepath.Join(tempDir, "file1.txt")
	node, err := fileInfo.CreateNode(filePath)
	if err != nil {
		t.Fatalf("Failed to create FileNode: %v", err)
	}

	// Add the same file multiple times
	fsm.AddFileNode(&node)
	fsm.AddFileNode(&node)
	fsm.AddFileNode(&node)

	// Should still only count as one file
	if fsm.GetFileCount() != 1 {
		t.Errorf("Expected 1 file after adding duplicates, got %d", fsm.GetFileCount())
	}

	// Should be able to retrieve the file
	retrievedNode, exists := fsm.GetFile(node.Path)
	if !exists {
		t.Error("File not found after adding duplicates")
	}

	if retrievedNode.Name != node.Name {
		t.Errorf("Expected file name %s, got %s", node.Name, retrievedNode.Name)
	}
}
