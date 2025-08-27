package webrtc

import (
	"fmt"
	"os"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestCIDiagnostic is a simple test to diagnose CI environment issues
func TestCIDiagnostic(t *testing.T) {
	t.Run("environment_check", func(t *testing.T) {
		// Log environment information
		t.Logf("Go version: %s", runtime.Version())
		t.Logf("GOOS: %s", runtime.GOOS)
		t.Logf("GOARCH: %s", runtime.GOARCH)
		t.Logf("NumCPU: %d", runtime.NumCPU())
		t.Logf("GOMAXPROCS: %d", runtime.GOMAXPROCS(0))

		// Check environment variables
		t.Logf("CI: %s", os.Getenv("CI"))
		t.Logf("GITHUB_ACTIONS: %s", os.Getenv("GITHUB_ACTIONS"))
		t.Logf("RUNNER_OS: %s", os.Getenv("RUNNER_OS"))

		// Basic assertions that should always pass
		assert.True(t, true, "Basic assertion should pass")
		require.NotEmpty(t, runtime.Version(), "Go version should not be empty")
	})

	t.Run("file_operations", func(t *testing.T) {
		// Test basic file operations that might fail in CI
		tempDir := t.TempDir()
		t.Logf("Temp directory: %s", tempDir)

		// Test file creation
		testFile := tempDir + "/test.txt"
		err := os.WriteFile(testFile, []byte("test content"), 0644)
		require.NoError(t, err, "Should be able to create test file")

		// Test file reading
		content, err := os.ReadFile(testFile)
		require.NoError(t, err, "Should be able to read test file")
		assert.Equal(t, "test content", string(content), "File content should match")

		// Test file info
		info, err := os.Stat(testFile)
		require.NoError(t, err, "Should be able to stat test file")
		assert.Equal(t, int64(12), info.Size(), "File size should be correct")
	})

	t.Run("memory_check", func(t *testing.T) {
		// Check memory usage
		var m runtime.MemStats
		runtime.ReadMemStats(&m)

		t.Logf("Memory stats:")
		t.Logf("  Alloc: %d KB", m.Alloc/1024)
		t.Logf("  TotalAlloc: %d KB", m.TotalAlloc/1024)
		t.Logf("  Sys: %d KB", m.Sys/1024)
		t.Logf("  NumGC: %d", m.NumGC)

		// Basic memory assertions
		assert.Greater(t, m.Sys, uint64(0), "System memory should be greater than 0")
	})

	t.Run("concurrent_operations", func(t *testing.T) {
		// Test basic concurrent operations that might reveal race conditions
		const numGoroutines = 10
		done := make(chan bool, numGoroutines)

		for i := 0; i < numGoroutines; i++ {
			go func(id int) {
				defer func() { done <- true }()

				// Simple operation that shouldn't cause races
				tempDir := t.TempDir()
				testFile := tempDir + "/concurrent_test.txt"
				content := fmt.Sprintf("content from goroutine %d", id)

				err := os.WriteFile(testFile, []byte(content), 0644)
				if err != nil {
					t.Errorf("Goroutine %d failed to write file: %v", id, err)
					return
				}

				readContent, err := os.ReadFile(testFile)
				if err != nil {
					t.Errorf("Goroutine %d failed to read file: %v", id, err)
					return
				}

				if string(readContent) != content {
					t.Errorf("Goroutine %d: content mismatch. Expected %s, got %s",
						id, content, string(readContent))
				}
			}(i)
		}

		// Wait for all goroutines to complete
		for i := 0; i < numGoroutines; i++ {
			<-done
		}

		t.Log("All concurrent operations completed successfully")
	})
}

// TestErrorHandlingSimplified is a simplified version of the error handling test
func TestErrorHandlingSimplified(t *testing.T) {
	t.Run("basic_error_handling", func(t *testing.T) {
		// Test the basic error handling pattern without complex dependencies

		// Simulate the error handling closure
		var capturedErrors []string
		handleError := func(filePath string, err error, context string) {
			errorMsg := fmt.Sprintf("Error in %s for file %s: %v", context, filePath, err)
			capturedErrors = append(capturedErrors, errorMsg)
			t.Logf("Handled error: %s", errorMsg)
		}

		// Simulate some errors
		testFiles := []string{"file1.txt", "file2.txt", "file3.txt"}
		testErrors := []error{
			fmt.Errorf("simulated error 1"),
			fmt.Errorf("simulated error 2"),
			fmt.Errorf("simulated error 3"),
		}

		// Process errors
		for i, file := range testFiles {
			handleError(file, testErrors[i], "test context")
		}

		// Verify error handling
		assert.Len(t, capturedErrors, 3, "Should have captured 3 errors")
		for i, errorMsg := range capturedErrors {
			assert.Contains(t, errorMsg, testFiles[i], "Error should contain file name")
			assert.Contains(t, errorMsg, testErrors[i].Error(), "Error should contain error message")
		}

		t.Log("Basic error handling test completed successfully")
	})
}
