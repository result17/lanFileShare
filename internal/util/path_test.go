package util

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCheckDirectory(t *testing.T) {
	// Create a temporary directory for testing
	tempDir, err := os.MkdirTemp("", "path_test")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create a temporary file for testing
	tempFile := filepath.Join(tempDir, "testfile.txt")
	file, err := os.Create(tempFile)
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	file.Close()

	tests := []struct {
		name           string
		path           string
		expectedExists bool
		expectedIsDir  bool
		expectError    bool
	}{
		{
			name:           "Existing directory",
			path:           tempDir,
			expectedExists: true,
			expectedIsDir:  true,
			expectError:    false,
		},
		{
			name:           "Existing file (not directory)",
			path:           tempFile,
			expectedExists: true,
			expectedIsDir:  false,
			expectError:    false,
		},
		{
			name:           "Non-existent path",
			path:           filepath.Join(tempDir, "nonexistent"),
			expectedExists: false,
			expectedIsDir:  false,
			expectError:    false,
		},
		{
			name:           "Current directory",
			path:           ".",
			expectedExists: true,
			expectedIsDir:  true,
			expectError:    false,
		},
		{
			name:           "Parent directory",
			path:           "..",
			expectedExists: true,
			expectedIsDir:  true,
			expectError:    false,
		},
		{
			name:           "Empty path",
			path:           "",
			expectedExists: false,
			expectedIsDir:  false,
			expectError:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			exists, isDir, err := CheckDirectory(tt.path)

			if tt.expectError && err == nil {
				t.Errorf("Expected error but got none")
			}
			if !tt.expectError && err != nil {
				t.Errorf("Unexpected error: %v", err)
			}
			if exists != tt.expectedExists {
				t.Errorf("Expected exists=%v, got %v", tt.expectedExists, exists)
			}
			if isDir != tt.expectedIsDir {
				t.Errorf("Expected isDir=%v, got %v", tt.expectedIsDir, isDir)
			}
		})
	}
}

func TestCheckDirectoryEdgeCases(t *testing.T) {
	tests := []struct {
		name        string
		path        string
		description string
	}{
		{
			name:        "Very long path",
			path:        string(make([]byte, 1000)),
			description: "Should handle very long paths gracefully",
		},
		{
			name:        "Path with special characters",
			path:        "test@#$%^&*()",
			description: "Should handle special characters in path",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// These should not panic and should return reasonable results
			exists, isDir, err := CheckDirectory(tt.path)
			
			// We don't assert specific values since these are edge cases,
			// but we ensure the function doesn't panic and returns consistent types
			_ = exists
			_ = isDir
			_ = err
			
			t.Logf("%s: exists=%v, isDir=%v, err=%v", tt.description, exists, isDir, err)
		})
	}
}

func TestCheckDirectorySymlinks(t *testing.T) {
	// Create a temporary directory for testing
	tempDir, err := os.MkdirTemp("", "symlink_test")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create a subdirectory
	subDir := filepath.Join(tempDir, "subdir")
	err = os.Mkdir(subDir, 0755)
	if err != nil {
		t.Fatalf("Failed to create subdirectory: %v", err)
	}

	// Create a symlink to the directory (skip on Windows if not supported)
	symlinkPath := filepath.Join(tempDir, "symlink")
	err = os.Symlink(subDir, symlinkPath)
	if err != nil {
		t.Skipf("Symlinks not supported or permission denied: %v", err)
	}

	// Test symlink to directory
	exists, isDir, err := CheckDirectory(symlinkPath)
	if err != nil {
		t.Errorf("Unexpected error checking symlink: %v", err)
	}
	if !exists {
		t.Errorf("Expected symlink to exist")
	}
	if !isDir {
		t.Errorf("Expected symlink to directory to be reported as directory")
	}
}

func BenchmarkCheckDirectory(b *testing.B) {
	// Create a temporary directory for benchmarking
	tempDir, err := os.MkdirTemp("", "bench_test")
	if err != nil {
		b.Fatalf("Failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(tempDir)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		CheckDirectory(tempDir)
	}
}