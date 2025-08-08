package transfer

import (
	"path/filepath"
	"testing"
	"fmt"
	"os"
	"sync"

	"github.com/rescp17/lanFileSharer/pkg/fileInfo"
)

func TestNewFileStructureManage(t *testing.T) {
	fsm := NewFileStructureManager()

	if fsm == nil {
		t.Fatal("NewFileStructureManager returned nil")
	}

	if fsm.RootNodes == nil {
		t.Error("RootNodes not initialized")
	}

	if fsm.fileMap == nil {
		t.Error("fileMap not initialized")
	}

	if fsm.dirMap == nil {
		t.Error("dirMap not initialized")
	}

	if len(fsm.RootNodes) != 0 {
		t.Error("RootNodes not empty")
	}
	
	if fsm.GetFileCount() != 0 {
		t.Errorf("File count should be 0 initially")

		if fsm.GetDirCount() != 0 {
			t.Error("Directory count should be 0 initially")
		}
	}
}

func TestFileStructureManager_AddSingleFile(t *testing.T) {
	fsm := NewFileStructureManager()
	
	tempDir, cleanup := setupTestDir(t)
	defer cleanup()
	
	filePath := filepath.Join(tempDir, "file1.txt")
	node, err := fileInfo.CreateNode(filePath)
	if err != nil {
		t.Fatalf("Failed to create FileNode: %v", err)
	}

	fsm.AddFileNode(&node)
	
	if fsm.GetFileCount() != 1 {
		t.Errorf("Expected 1 file, got %d", fsm.GetFileCount())
	}
	
	if fsm.GetDirCount() != 0 {
		t.Errorf("Expected 0 directories, got %d", fsm.GetDirCount())
	}

	retrievedNode, exists := fsm.GetFile(node.Path)
	if !exists {
		t.Error("File not found in fileMap")
	}
	
	if retrievedNode.Name != node.Name {
		t.Errorf("Expected file name %s, got %s", node.Name, retrievedNode.Name)
	}
}

func TestFileStructureManager_AddDirectory(t *testing.T) {
	fsm := NewFileStructureManager()
	
	tempDir, cleanup := setupTestDir(t)
	defer cleanup()
	
	node, err := fileInfo.CreateNode(tempDir)
	if err != nil {
		t.Fatalf("Failed to create FileNode for directory: %v", err)
	}
	
	fsm.AddFileNode(&node)
	
	expectedFiles := 5
	if fsm.GetFileCount() != expectedFiles {
		t.Errorf("Expected %d files, got %d", expectedFiles, fsm.GetFileCount())
	}
	
	expectedDirs := 2
	if fsm.GetDirCount() != expectedDirs {
		t.Errorf("Expected %d directories, got %d", expectedDirs, fsm.GetDirCount())
	}
	
	_, exists := fsm.GetDir(tempDir)
	if !exists {
		t.Error("Root directory not found in dirMap")
	}
	
	subDirPath := filepath.Join(tempDir, "subdir")
	_, exists = fsm.GetDir(subDirPath)
	if !exists {
		t.Error("Subdirectory not found in dirMap")
	}
}

func TestFileStructureManager_ConcurrentAccess(t *testing.T) {
	fsm := NewFileStructureManager()
	
	tempDir, cleanup := setupTestDir(t)
	defer cleanup()
	
	const numGoroutines = 10
	var wg sync.WaitGroup
	
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()
			
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
			
			fsm.AddFileNode(&node)
		}(i)
	}
	
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()
			
			_ = fsm.GetFileCount()
			_ = fsm.GetDirCount()
			_ = fsm.GetTotalSize()
		}(i)
	}
	
	wg.Wait()
	
	if fsm.GetFileCount() < numGoroutines {
		t.Errorf("Expected at least %d files, got %d", numGoroutines, fsm.GetFileCount())
	}
}
