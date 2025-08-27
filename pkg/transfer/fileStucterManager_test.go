package transfer

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/rescp17/lanFileSharer/pkg/fileInfo"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestNewFileStructureManager tests the initialization of FileStructureManager
func TestNewFileStructureManager(t *testing.T) {
	fsm := NewFileStructureManager()

	// Verify the manager was created successfully
	require.NotNil(t, fsm, "NewFileStructureManager returned nil")

	// Verify all fields are properly initialized
	assert.NotNil(t, fsm.RootNodes, "RootNodes slice not initialized")
	assert.NotNil(t, fsm.fileMap, "fileMap not initialized")
	assert.NotNil(t, fsm.dirMap, "dirMap not initialized")

	// Verify initial state is empty
	assert.Equal(t, 0, len(fsm.RootNodes), "RootNodes should be empty initially")
	assert.Equal(t, 0, fsm.GetFileCount(), "File count should be 0 initially")
	assert.Equal(t, 0, fsm.GetDirCount(), "Directory count should be 0 initially")
	assert.Equal(t, int64(0), fsm.GetTotalSize(), "Total size should be 0 initially")
}

// TestFileStructureManager_AddSingleFile tests adding a single file
func TestFileStructureManager_AddSingleFile(t *testing.T) {
	fsm := NewFileStructureManager()

	// Create test file
	tempDir := setupTestDir(t)

	filePath := filepath.Join(tempDir, "file1.txt")
	node, err := fileInfo.CreateNode(filePath)
	require.NoError(t, err, "Failed to create FileNode")

	// Add the file to the manager
	fsm.AddFileNode(&node)

	// Verify file count increased
	assert.Equal(t, 1, fsm.GetFileCount(), "Expected 1 file")

	// Verify no directories were added
	assert.Equal(t, 0, fsm.GetDirCount(), "Expected 0 directories")

	// Verify the file can be retrieved
	retrievedNode, exists := fsm.GetFile(node.Path)
	assert.True(t, exists, "File not found in fileMap")
	require.NotNil(t, retrievedNode, "Retrieved node is nil")

	assert.Equal(t, node.Name, retrievedNode.Name, "Expected file name mismatch")
	assert.Equal(t, node.Size, retrievedNode.Size, "Expected file size mismatch")

	// Verify total size calculation
	expectedSize := node.Size
	assert.Equal(t, expectedSize, fsm.GetTotalSize(), "Expected total size mismatch")
}

// TestFileStructureManager_AddDirectory tests adding a directory structure
func TestFileStructureManager_AddDirectory(t *testing.T) {
	fsm := NewFileStructureManager()

	// Create test directory structure
	tempDir := setupTestDir(t)

	node, err := fileInfo.CreateNode(tempDir)
	require.NoError(t, err, "Failed to create FileNode for directory")

	// Add the entire directory structure
	fsm.AddFileNode(&node)

	// Verify file count (3 files in root + 2 files in subdir = 5 total)
	expectedFiles := 5
	assert.Equal(t, expectedFiles, fsm.GetFileCount(), "Expected files count mismatch")

	// Verify directory count (root dir + subdir = 2 total)
	expectedDirs := 2
	assert.Equal(t, expectedDirs, fsm.GetDirCount(), "Expected directories count mismatch")

	// Verify root directory exists
	_, exists := fsm.GetDir(tempDir)
	assert.True(t, exists, "Root directory not found in dirMap")

	// Verify subdirectory exists
	subDirPath := filepath.Join(tempDir, "subdir")
	_, exists = fsm.GetDir(subDirPath)
	assert.True(t, exists, "Subdirectory not found in dirMap")

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
		assert.True(t, exists, "Expected file not found: %s", filePath)
	}

	// Verify total size is positive
	totalSize := fsm.GetTotalSize()
	assert.Greater(t, totalSize, int64(0), "Expected positive total size")
}

// TestFileStructureManager_GetAllFiles tests retrieving all files
func TestFileStructureManager_GetAllFiles(t *testing.T) {
	fsm := NewFileStructureManager()

	tempDir := setupTestDir(t)

	node, err := fileInfo.CreateNode(tempDir)
	require.NoError(t, err, "Failed to create FileNode")

	fsm.AddFileNode(&node)

	// Get all files
	files := fsm.GetAllFiles()

	// Verify count matches
	assert.Equal(t, fsm.GetFileCount(), len(files), "GetAllFiles count mismatch with GetFileCount")

	// Verify all files are valid
	for i, file := range files {
		assert.NotNil(t, file, "File at index %d is nil", i)
		if file == nil {
			continue
		}

		assert.False(t, file.IsDir, "GetAllFiles returned a directory: %s", file.Path)
		assert.Greater(t, file.Size, int64(0), "File %s has invalid size", file.Path)
	}
}

// TestFileStructureManager_EmptyDirectory tests handling of empty directories
func TestFileStructureManager_EmptyDirectory(t *testing.T) {
	fsm := NewFileStructureManager()

	// Create empty directory
	tempDir, err := os.MkdirTemp("", "empty-dir-test-*")
	require.NoError(t, err, "Failed to create temp dir")
	defer os.RemoveAll(tempDir)

	node, err := fileInfo.CreateNode(tempDir)
	require.NoError(t, err, "Failed to create FileNode for empty directory")

	fsm.AddFileNode(&node)

	// Verify empty directory is recorded
	assert.Equal(t, 1, fsm.GetDirCount(), "Expected 1 directory")
	assert.Equal(t, 0, fsm.GetFileCount(), "Expected 0 files")

	// Verify the empty directory can be retrieved
	retrievedDir, exists := fsm.GetDir(node.Path)
	assert.True(t, exists, "Empty directory not found in dirMap")
	require.NotNil(t, retrievedDir, "Retrieved directory node is nil")
	assert.True(t, retrievedDir.IsDir, "Retrieved node is not marked as directory")

	// Verify total size is zero
	assert.Equal(t, int64(0), fsm.GetTotalSize(), "Expected total size 0 for empty directory")
}

// TestFileStructureManager_Clear tests clearing all data
func TestFileStructureManager_Clear(t *testing.T) {
	fsm := NewFileStructureManager()

	// Add some data first
	tempDir := setupTestDir(t)

	node, err := fileInfo.CreateNode(tempDir)
	require.NoError(t, err, "Failed to create FileNode")

	fsm.AddFileNode(&node)

	// Verify data was added
	require.Greater(t, fsm.GetFileCount(), 0, "No files were added before clear test")
	require.Greater(t, fsm.GetDirCount(), 0, "No directories were added before clear test")

	// Clear all data
	fsm.Clear()

	// Verify everything is cleared
	assert.Equal(t, 0, fsm.GetFileCount(), "Expected 0 files after clear")
	assert.Equal(t, 0, fsm.GetDirCount(), "Expected 0 directories after clear")
	assert.Equal(t, 0, len(fsm.RootNodes), "Expected 0 root nodes after clear")
	assert.Equal(t, int64(0), fsm.GetTotalSize(), "Expected 0 total size after clear")
}

// TestFileStructureManager_ConcurrentAccess tests thread safety
func TestFileStructureManager_ConcurrentAccess(t *testing.T) {
	tempDir := setupTestDir(t)

	fsm, err := NewFileStructureManagerFromPath(tempDir)
	require.NoError(t, err, "Failed to create FileStructureManager")

	const numWriters = 5
	const numReaders = 3
	var wg sync.WaitGroup

	// Use channels to collect errors from goroutines to avoid data races
	errorChan := make(chan error, numWriters+numReaders*4) // Buffer for potential errors
	
	// Concurrent writers - add files
	for i := 0; i < numWriters; i++ {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()

			// Create unique test file
			fileName := filepath.Join(tempDir, fmt.Sprintf("concurrent_%d.txt", index))
			content := fmt.Sprintf("Content for concurrent file %d", index)

			if err := os.WriteFile(fileName, []byte(content), 0644); err != nil {
				errorChan <- fmt.Errorf("writer %d: failed to create concurrent test file: %w", index, err)
				return
			}

			node, err := fileInfo.CreateNode(fileName)
			if err != nil {
				errorChan <- fmt.Errorf("writer %d: failed to create FileNode: %w", index, err)
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

			// Verify read consistency - collect errors instead of asserting directly
			if fileCount < 0 {
				errorChan <- fmt.Errorf("reader %d: invalid file count: %d", index, fileCount)
			}
			if dirCount < 0 {
				errorChan <- fmt.Errorf("reader %d: invalid directory count: %d", index, dirCount)
			}
			if totalSize < 0 {
				errorChan <- fmt.Errorf("reader %d: invalid total size: %d", index, totalSize)
			}
			if fileCount != len(allFiles) {
				errorChan <- fmt.Errorf("reader %d: inconsistent file count: got %d, expected %d", index, len(allFiles), fileCount)
			}
		}(i)
	}

	wg.Wait()
	close(errorChan)

	// Check for any errors from goroutines
	for err := range errorChan {
		t.Errorf("Goroutine error: %v", err)
	}

	// Verify final state
	// Should have at least the concurrent files plus the original setupTestDir files
	minExpectedFiles := numWriters + 5 // 5 from setupTestDir
	assert.GreaterOrEqual(t, fsm.GetFileCount(), minExpectedFiles, "Expected at least %d files", minExpectedFiles)

	// Verify all concurrent files were added
	for i := 0; i < numWriters; i++ {
		fileName := filepath.Join(tempDir, fmt.Sprintf("concurrent_%d.txt", i))
		_, exists := fsm.GetFile(fileName)
		assert.True(t, exists, "Concurrent file %d not found: %s", i, fileName)
	}
}

// TestFileStructureManager_DuplicateFiles tests handling of duplicate file additions
func TestFileStructureManager_DuplicateFiles(t *testing.T) {
	fsm := NewFileStructureManager()

	tempDir := setupTestDir(t)

	filePath := filepath.Join(tempDir, "file1.txt")
	node, err := fileInfo.CreateNode(filePath)
	require.NoError(t, err, "Failed to create FileNode")

	// Add the same file multiple times
	fsm.AddFileNode(&node)
	fsm.AddFileNode(&node)
	fsm.AddFileNode(&node)

	// Should still only count as one file
	assert.Equal(t, 1, fsm.GetFileCount(), "Expected 1 file after adding duplicates")

	// Should be able to retrieve the file
	retrievedNode, exists := fsm.GetFile(node.Path)
	assert.True(t, exists, "File not found after adding duplicates")
	assert.Equal(t, node.Name, retrievedNode.Name, "Expected file name mismatch")
}

// TestConvertFileNodePointers tests converting []*fileInfo.FileNode to []fileInfo.FileNode
func TestConvertFileNodePointers(t *testing.T) {
	t.Run("convert_valid_pointers", func(t *testing.T) {
		// Create test data
		tempDir := setupTestDir(t)

		fsm := NewFileStructureManager()
		node, err := fileInfo.CreateNode(tempDir)
		require.NoError(t, err, "Failed to create FileNode")
		fsm.AddFileNode(&node)

		// Get pointer slice
		pointers := fsm.GetAllFiles()
		require.Greater(t, len(pointers), 0, "Expected some files")

		// Convert to value slice
		values := convertFileNodePointers(pointers)

		// Verify conversion
		assert.Equal(t, len(pointers), len(values), "Length should match")
		
		for i, ptr := range pointers {
			if ptr != nil {
				assert.Equal(t, ptr.Name, values[i].Name, "Name should match at index %d", i)
				assert.Equal(t, ptr.Path, values[i].Path, "Path should match at index %d", i)
				assert.Equal(t, ptr.Size, values[i].Size, "Size should match at index %d", i)
				assert.Equal(t, ptr.IsDir, values[i].IsDir, "IsDir should match at index %d", i)
			}
		}
	})

	t.Run("convert_with_nil_pointers", func(t *testing.T) {
		// Create slice with nil pointers
		pointers := []*fileInfo.FileNode{
			nil,
			&fileInfo.FileNode{Name: "test1.txt", Path: "/test1.txt", Size: 100, IsDir: false},
			nil,
			&fileInfo.FileNode{Name: "test2.txt", Path: "/test2.txt", Size: 200, IsDir: false},
			nil,
		}

		// Convert to value slice
		values := convertFileNodePointers(pointers)

		// Should only have non-nil values
		assert.Equal(t, 2, len(values), "Should only have 2 non-nil values")
		assert.Equal(t, "test1.txt", values[0].Name, "First value name should match")
		assert.Equal(t, "test2.txt", values[1].Name, "Second value name should match")
	})

	t.Run("convert_empty_slice", func(t *testing.T) {
		pointers := []*fileInfo.FileNode{}
		values := convertFileNodePointers(pointers)
		assert.Equal(t, 0, len(values), "Empty slice should remain empty")
	})

	t.Run("convert_nil_slice", func(t *testing.T) {
		var pointers []*fileInfo.FileNode = nil
		values := convertFileNodePointers(pointers)
		assert.Nil(t, values, "Nil slice should return nil")
	})
}

// TestConvertFileNodePointersUnsafe tests the unsafe conversion function
func TestConvertFileNodePointersUnsafe(t *testing.T) {
	t.Run("convert_valid_pointers_unsafe", func(t *testing.T) {
		// Create test data with no nil pointers
		pointers := []*fileInfo.FileNode{
			&fileInfo.FileNode{Name: "test1.txt", Path: "/test1.txt", Size: 100, IsDir: false},
			&fileInfo.FileNode{Name: "test2.txt", Path: "/test2.txt", Size: 200, IsDir: false},
		}

		// Convert using unsafe method
		values := convertFileNodePointersUnsafe(pointers)

		// Verify conversion
		assert.Equal(t, len(pointers), len(values), "Length should match")
		
		for i, ptr := range pointers {
			assert.Equal(t, ptr.Name, values[i].Name, "Name should match at index %d", i)
			assert.Equal(t, ptr.Path, values[i].Path, "Path should match at index %d", i)
			assert.Equal(t, ptr.Size, values[i].Size, "Size should match at index %d", i)
			assert.Equal(t, ptr.IsDir, values[i].IsDir, "IsDir should match at index %d", i)
		}
	})

	t.Run("convert_nil_slice_unsafe", func(t *testing.T) {
		var pointers []*fileInfo.FileNode = nil
		values := convertFileNodePointersUnsafe(pointers)
		assert.Nil(t, values, "Nil slice should return nil")
	})

	// Note: We don't test with nil pointers in the slice because it would panic
	// That's the expected behavior for the "unsafe" version
}

// TestConvertFileNodeValues tests converting []fileInfo.FileNode to []*fileInfo.FileNode
func TestConvertFileNodeValues(t *testing.T) {
	t.Run("convert_values_to_pointers", func(t *testing.T) {
		// Create test data
		values := []fileInfo.FileNode{
			{Name: "test1.txt", Path: "/test1.txt", Size: 100, IsDir: false},
			{Name: "test2.txt", Path: "/test2.txt", Size: 200, IsDir: false},
		}

		// Convert to pointer slice
		pointers := convertFileNodeValues(values)

		// Verify conversion
		assert.Equal(t, len(values), len(pointers), "Length should match")
		
		for i, value := range values {
			require.NotNil(t, pointers[i], "Pointer should not be nil at index %d", i)
			assert.Equal(t, value.Name, pointers[i].Name, "Name should match at index %d", i)
			assert.Equal(t, value.Path, pointers[i].Path, "Path should match at index %d", i)
			assert.Equal(t, value.Size, pointers[i].Size, "Size should match at index %d", i)
			assert.Equal(t, value.IsDir, pointers[i].IsDir, "IsDir should match at index %d", i)
		}
	})

	t.Run("convert_empty_values", func(t *testing.T) {
		values := []fileInfo.FileNode{}
		pointers := convertFileNodeValues(values)
		assert.Equal(t, 0, len(pointers), "Empty slice should remain empty")
	})

	t.Run("convert_nil_values", func(t *testing.T) {
		var values []fileInfo.FileNode = nil
		pointers := convertFileNodeValues(values)
		assert.Nil(t, pointers, "Nil slice should return nil")
	})
}

// TestGetAllFilesEntity tests the new GetAllFilesEntity method
func TestGetAllFilesEntity(t *testing.T) {
	fsm := NewFileStructureManager()

	tempDir := setupTestDir(t)

	node, err := fileInfo.CreateNode(tempDir)
	require.NoError(t, err, "Failed to create FileNode")
	fsm.AddFileNode(&node)

	// Get files as pointers
	filePointers := fsm.GetAllFiles()
	require.Greater(t, len(filePointers), 0, "Expected some files")

	// Get files as values using new method
	fileValues := fsm.GetAllFileEntities()

	// Verify both methods return equivalent data
	assert.Equal(t, len(filePointers), len(fileValues), "Both methods should return same count")

	// Convert pointers to values manually for comparison
	expectedValues := convertFileNodePointers(filePointers)
	
	// Compare the results (order-independent)
	// Create maps for easier comparison since map iteration order is not guaranteed
	expectedMap := make(map[string]fileInfo.FileNode)
	for _, node := range expectedValues {
		expectedMap[node.Path] = node
	}
	
	actualMap := make(map[string]fileInfo.FileNode)
	for _, node := range fileValues {
		actualMap[node.Path] = node
	}
	
	// Verify all expected files are present
	for path, expected := range expectedMap {
		actual, exists := actualMap[path]
		assert.True(t, exists, "File should exist in actual results: %s", path)
		if exists {
			assert.Equal(t, expected.Name, actual.Name, "Name should match for %s", path)
			assert.Equal(t, expected.Path, actual.Path, "Path should match for %s", path)
			assert.Equal(t, expected.Size, actual.Size, "Size should match for %s", path)
			assert.Equal(t, expected.IsDir, actual.IsDir, "IsDir should match for %s", path)
		}
	}
	
	// Verify no extra files in actual results
	for path := range actualMap {
		_, exists := expectedMap[path]
		assert.True(t, exists, "Unexpected file in actual results: %s", path)
	}
}

// Helper functions for conversion

// convertFileNodePointers converts []*fileInfo.FileNode to []fileInfo.FileNode
// This function safely handles nil pointers by filtering them out
func convertFileNodePointers(pointers []*fileInfo.FileNode) []fileInfo.FileNode {
	if pointers == nil {
		return nil
	}
	
	result := make([]fileInfo.FileNode, 0, len(pointers))
	for _, ptr := range pointers {
		if ptr != nil {
			result = append(result, *ptr)
		}
	}
	return result
}

// convertFileNodePointersUnsafe converts []*fileInfo.FileNode to []fileInfo.FileNode
// This function assumes all pointers are non-nil and will panic if any are nil
// Use this only when you're certain all pointers are valid
func convertFileNodePointersUnsafe(pointers []*fileInfo.FileNode) []fileInfo.FileNode {
	if pointers == nil {
		return nil
	}
	
	result := make([]fileInfo.FileNode, len(pointers))
	for i, ptr := range pointers {
		result[i] = *ptr // Will panic if ptr is nil
	}
	return result
}

// convertFileNodeValues converts []fileInfo.FileNode to []*fileInfo.FileNode
// This is the reverse operation - converting values to pointers
func convertFileNodeValues(values []fileInfo.FileNode) []*fileInfo.FileNode {
	if values == nil {
		return nil
	}
	
	result := make([]*fileInfo.FileNode, len(values))
	for i := range values {
		result[i] = &values[i]
	}
	return result
}
// TestFileStructureManager_LoopVariablePointerBug tests the fix for the critical pointer bug
// This test verifies that when adding directory structures with multiple children,
// each child maintains its correct data and doesn't get overwritten by the loop variable reuse bug
func TestFileStructureManager_LoopVariablePointerBug(t *testing.T) {
	fsm := NewFileStructureManager()

	// Create a directory structure with multiple children having different data
	parentNode := &fileInfo.FileNode{
		Name:  "parent",
		Path:  "/parent",
		IsDir: true,
		Size:  0,
		Children: []fileInfo.FileNode{
			{
				Name:  "child1.txt",
				Path:  "/parent/child1.txt",
				IsDir: false,
				Size:  100, // Different sizes to detect the bug
			},
			{
				Name:  "child2.txt",
				Path:  "/parent/child2.txt",
				IsDir: false,
				Size:  200,
			},
			{
				Name:  "child3.txt",
				Path:  "/parent/child3.txt",
				IsDir: false,
				Size:  300,
			},
			{
				Name:  "child4.txt",
				Path:  "/parent/child4.txt",
				IsDir: false,
				Size:  400,
			},
		},
	}

	// Add the directory structure
	err := fsm.AddFileNode(parentNode)
	require.NoError(t, err, "Failed to add directory structure")

	// Verify that we have the correct number of files
	files := fsm.GetAllFiles()
	assert.Equal(t, 4, len(files), "Should have 4 files")

	// Create a map of expected data for easy verification
	expectedData := map[string]struct {
		name string
		size int64
	}{
		"/parent/child1.txt": {"child1.txt", 100},
		"/parent/child2.txt": {"child2.txt", 200},
		"/parent/child3.txt": {"child3.txt", 300},
		"/parent/child4.txt": {"child4.txt", 400},
	}

	// Verify that each file has the correct data
	// If the pointer bug existed, all files would have the same data (from the last loop iteration)
	for _, file := range files {
		expected, exists := expectedData[file.Path]
		require.True(t, exists, "Unexpected file path: %s", file.Path)
		
		assert.Equal(t, expected.name, file.Name, 
			"File %s should have name %s, got %s", file.Path, expected.name, file.Name)
		assert.Equal(t, expected.size, file.Size,
			"File %s should have size %d, got %d", file.Path, expected.size, file.Size)
		assert.False(t, file.IsDir, "File %s should not be a directory", file.Path)
	}

	// Additional verification: ensure all expected files are present
	actualPaths := make(map[string]bool)
	for _, file := range files {
		actualPaths[file.Path] = true
	}

	for expectedPath := range expectedData {
		assert.True(t, actualPaths[expectedPath], "Expected file not found: %s", expectedPath)
	}
}

// TestFileStructureManager_NestedDirectoryPointerBug tests the pointer bug fix with nested directories
func TestFileStructureManager_NestedDirectoryPointerBug(t *testing.T) {
	fsm := NewFileStructureManager()

	// Create a more complex nested structure to thoroughly test the fix
	rootNode := &fileInfo.FileNode{
		Name:  "root",
		Path:  "/root",
		IsDir: true,
		Children: []fileInfo.FileNode{
			{
				Name:  "subdir1",
				Path:  "/root/subdir1",
				IsDir: true,
				Children: []fileInfo.FileNode{
					{
						Name:  "file1.txt",
						Path:  "/root/subdir1/file1.txt",
						IsDir: false,
						Size:  1000,
					},
					{
						Name:  "file2.txt",
						Path:  "/root/subdir1/file2.txt",
						IsDir: false,
						Size:  2000,
					},
				},
			},
			{
				Name:  "subdir2",
				Path:  "/root/subdir2",
				IsDir: true,
				Children: []fileInfo.FileNode{
					{
						Name:  "file3.txt",
						Path:  "/root/subdir2/file3.txt",
						IsDir: false,
						Size:  3000,
					},
				},
			},
		},
	}

	err := fsm.AddFileNode(rootNode)
	require.NoError(t, err, "Failed to add nested directory structure")

	// Verify files
	files := fsm.GetAllFiles()
	assert.Equal(t, 3, len(files), "Should have 3 files")

	// Verify directories
	dirs := fsm.GetAllDirs()
	assert.Equal(t, 3, len(dirs), "Should have 3 directories (root + 2 subdirs)")

	// Verify specific file data
	expectedFiles := map[string]int64{
		"/root/subdir1/file1.txt": 1000,
		"/root/subdir1/file2.txt": 2000,
		"/root/subdir2/file3.txt": 3000,
	}

	for _, file := range files {
		expectedSize, exists := expectedFiles[file.Path]
		require.True(t, exists, "Unexpected file: %s", file.Path)
		assert.Equal(t, expectedSize, file.Size, 
			"File %s should have size %d, got %d", file.Path, expectedSize, file.Size)
	}
}

// TestFileStructureManager_RootNodesConsistency tests that RootNodes stays consistent with internal state
// This addresses the bug where AddPath and AddFileNode didn't update RootNodes
func TestFileStructureManager_RootNodesConsistency(t *testing.T) {
	tempDir := setupTestDir(t)

	// Create test files
	file1 := filepath.Join(tempDir, "file1.txt")
	file2 := filepath.Join(tempDir, "file2.txt")
	
	err := os.WriteFile(file1, []byte("content1"), 0644)
	require.NoError(t, err, "Failed to create test file1")
	
	err = os.WriteFile(file2, []byte("content2"), 0644)
	require.NoError(t, err, "Failed to create test file2")

	fsm := NewFileStructureManager()

	// Initial state should be consistent
	assert.Equal(t, 0, len(fsm.RootNodes), "Initial RootNodes should be empty")
	assert.Equal(t, 0, fsm.GetFileCount(), "Initial file count should be 0")

	// Test AddPath consistency
	err = fsm.AddPath(file1)
	require.NoError(t, err, "Failed to add file1 via AddPath")

	assert.Equal(t, 1, len(fsm.RootNodes), "RootNodes should have 1 entry after AddPath")
	assert.Equal(t, 1, fsm.GetFileCount(), "File count should be 1 after AddPath")
	assert.Equal(t, file1, fsm.RootNodes[0].Path, "RootNodes[0] should point to file1")

	// Test AddFileNode consistency
	node2, err := fileInfo.CreateNode(file2)
	require.NoError(t, err, "Failed to create node for file2")

	err = fsm.AddFileNode(&node2)
	require.NoError(t, err, "Failed to add file2 via AddFileNode")

	assert.Equal(t, 2, len(fsm.RootNodes), "RootNodes should have 2 entries after AddFileNode")
	assert.Equal(t, 2, fsm.GetFileCount(), "File count should be 2 after AddFileNode")
	assert.Equal(t, file2, fsm.RootNodes[1].Path, "RootNodes[1] should point to file2")

	// Verify all RootNodes are accessible
	rootPaths := make(map[string]bool)
	for _, node := range fsm.RootNodes {
		rootPaths[node.Path] = true
	}

	assert.True(t, rootPaths[file1], "file1 should be in RootNodes")
	assert.True(t, rootPaths[file2], "file2 should be in RootNodes")

	// Verify internal consistency
	allFiles := fsm.GetAllFiles()
	assert.Equal(t, len(fsm.RootNodes), len(allFiles), 
		"RootNodes count should match internal file count for this test case")
}

// TestFileStructureManager_RootNodesWithDirectories tests RootNodes consistency with directories
func TestFileStructureManager_RootNodesWithDirectories(t *testing.T) {
	tempDir := setupTestDir(t)

	// Create a subdirectory with files (use different name to avoid conflict with setupTestDir)
	subDir := filepath.Join(tempDir, "testsubdir")
	err := os.Mkdir(subDir, 0755)
	require.NoError(t, err, "Failed to create subdirectory")

	subFile := filepath.Join(subDir, "subfile.txt")
	err = os.WriteFile(subFile, []byte("subcontent"), 0644)
	require.NoError(t, err, "Failed to create subfile")

	fsm := NewFileStructureManager()

	// Add directory via AddPath
	err = fsm.AddPath(subDir)
	require.NoError(t, err, "Failed to add directory via AddPath")

	// Verify RootNodes contains the directory
	assert.Equal(t, 1, len(fsm.RootNodes), "RootNodes should have 1 entry (the directory)")
	assert.Equal(t, subDir, fsm.RootNodes[0].Path, "RootNodes[0] should point to the directory")
	assert.True(t, fsm.RootNodes[0].IsDir, "RootNodes[0] should be marked as directory")

	// Verify internal state
	assert.Equal(t, 1, fsm.GetDirCount(), "Should have 1 directory")
	assert.Equal(t, 1, fsm.GetFileCount(), "Should have 1 file (the subfile)")

	// Verify the directory structure is preserved
	assert.Equal(t, 1, len(fsm.RootNodes[0].Children), "Directory should have 1 child")
	assert.Equal(t, subFile, fsm.RootNodes[0].Children[0].Path, "Child should be the subfile")
}
// TestFileStructureManager_ConcurrentAccessWithSubtests demonstrates the idiomatic way using t.Run
func TestFileStructureManager_ConcurrentAccessWithSubtests(t *testing.T) {
	tempDir := setupTestDir(t)

	fsm, err := NewFileStructureManagerFromPath(tempDir)
	require.NoError(t, err, "Failed to create FileStructureManager")

	const numOperations = 5

	// Test concurrent writes using t.Run for parallel subtests
	t.Run("ConcurrentWrites", func(t *testing.T) {
		for i := 0; i < numOperations; i++ {
			i := i // Capture loop variable
			t.Run(fmt.Sprintf("Writer_%d", i), func(t *testing.T) {
				t.Parallel() // Enable parallel execution

				// Create unique test file
				fileName := filepath.Join(tempDir, fmt.Sprintf("parallel_%d.txt", i))
				content := fmt.Sprintf("Content for parallel file %d", i)

				err := os.WriteFile(fileName, []byte(content), 0644)
				require.NoError(t, err, "Failed to create parallel test file")

				node, err := fileInfo.CreateNode(fileName)
				require.NoError(t, err, "Failed to create FileNode")

				fsm.AddFileNode(&node)
			})
		}
	})

	// Test concurrent reads using t.Run for parallel subtests
	t.Run("ConcurrentReads", func(t *testing.T) {
		for i := 0; i < numOperations; i++ {
			i := i // Capture loop variable
			t.Run(fmt.Sprintf("Reader_%d", i), func(t *testing.T) {
				t.Parallel() // Enable parallel execution

				// Perform multiple read operations
				for j := 0; j < 3; j++ {
					fileCount := fsm.GetFileCount()
					dirCount := fsm.GetDirCount()
					totalSize := fsm.GetTotalSize()
					allFiles := fsm.GetAllFiles()

					// Basic sanity checks - each subtest has its own *testing.T
					assert.GreaterOrEqual(t, fileCount, 0, "File count should be non-negative")
					assert.GreaterOrEqual(t, dirCount, 0, "Dir count should be non-negative")
					assert.GreaterOrEqual(t, totalSize, int64(0), "Total size should be non-negative")
					assert.NotNil(t, allFiles, "GetAllFiles should not return nil")

					// Small delay to increase chance of race conditions
					time.Sleep(time.Microsecond)
				}
			})
		}
	})

	// Final verification after all parallel operations
	t.Run("FinalVerification", func(t *testing.T) {
		// Verify final state consistency
		fileCount := fsm.GetFileCount()

		assert.Greater(t, fileCount, 0, "Should have files after concurrent operations")

		// Verify no data corruption by checking that we can still perform operations
		allFiles := fsm.GetAllFiles()
		assert.NotEmpty(t, allFiles, "Should have files after concurrent operations")

		// Log final statistics for debugging
		t.Logf("Final state: %d files, %d dirs", 
			fsm.GetFileCount(), fsm.GetDirCount())
	})
}