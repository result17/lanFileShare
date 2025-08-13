package receiver

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/rescp17/lanFileSharer/internal/app_events/receiver"
	"github.com/rescp17/lanFileSharer/pkg/transfer"
)

// TestFileReceiver_IntegrityVerification tests the integrity verification functionality
func TestFileReceiver_IntegrityVerification(t *testing.T) {
	// Create temporary directory for test files
	tempDir, err := os.MkdirTemp("", "file_receiver_test")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create UI messages channel for testing
	uiMessages := make(chan tea.Msg, 10)

	// Create file receiver
	fileReceiver := NewFileReceiver(tempDir, uiMessages)

	t.Run("successful_verification", func(t *testing.T) {
		// Test data
		testData := []byte("Hello, World! This is test file content.")
		expectedHash := calculateTestHash(testData)
		fileName := "test_file.txt"
		fileID := "test_file_1"

		// Create chunk message
		chunkMsg := &transfer.ChunkMessage{
			Type:         transfer.ChunkData,
			FileID:       fileID,
			FileName:     fileName,
			SequenceNo:   1,
			Data:         testData,
			TotalSize:    int64(len(testData)),
			ExpectedHash: expectedHash,
		}

		// Serialize and process chunk
		serializer := transfer.NewJSONSerializer()
		data, err := serializer.Marshal(chunkMsg)
		if err != nil {
			t.Fatalf("Failed to marshal chunk message: %v", err)
		}

		err = fileReceiver.ProcessChunk(data)
		if err != nil {
			t.Fatalf("Failed to process chunk: %v", err)
		}

		// Verify file was created and has correct content
		outputPath := filepath.Join(tempDir, fileName)
		if _, err := os.Stat(outputPath); os.IsNotExist(err) {
			t.Fatalf("Output file was not created: %s", outputPath)
		}

		// Read and verify file content
		content, err := os.ReadFile(outputPath)
		if err != nil {
			t.Fatalf("Failed to read output file: %v", err)
		}

		if string(content) != string(testData) {
			t.Errorf("File content mismatch. Expected: %s, Got: %s", string(testData), string(content))
		}

		// Check UI messages
		select {
		case msg := <-uiMessages:
			if statusMsg, ok := msg.(receiver.StatusUpdateMsg); ok {
				expectedMsg := fmt.Sprintf("Receiving file: %s", fileName)
				if statusMsg.Message != expectedMsg {
					t.Errorf("Expected UI message: %s, Got: %s", expectedMsg, statusMsg.Message)
				}
			} else {
				t.Errorf("Expected StatusUpdateMsg, got: %T", msg)
			}
		default:
			t.Error("Expected UI message for file reception start")
		}
	})

	t.Run("verification_failure_with_cleanup", func(t *testing.T) {
		// Test data with incorrect hash
		testData := []byte("This is corrupted file content.")
		incorrectHash := "incorrect_hash_value"
		fileName := "corrupted_file.txt"
		fileID := "corrupted_file_1"

		// Create chunk message with incorrect hash
		chunkMsg := &transfer.ChunkMessage{
			Type:         transfer.ChunkData,
			FileID:       fileID,
			FileName:     fileName,
			SequenceNo:   1,
			Data:         testData,
			TotalSize:    int64(len(testData)),
			ExpectedHash: incorrectHash,
		}

		// Serialize and process chunk
		serializer := transfer.NewJSONSerializer()
		data, err := serializer.Marshal(chunkMsg)
		if err != nil {
			t.Fatalf("Failed to marshal chunk message: %v", err)
		}

		err = fileReceiver.ProcessChunk(data)
		if err == nil {
			t.Fatal("Expected error for corrupted file, but got none")
		}

		// Verify error message contains verification failure
		expectedErrMsg := "file integrity verification failed"
		if !containsString(err.Error(), expectedErrMsg) {
			t.Errorf("Expected error to contain '%s', got: %s", expectedErrMsg, err.Error())
		}

		// Verify corrupted file was cleaned up
		outputPath := filepath.Join(tempDir, fileName)
		if _, err := os.Stat(outputPath); !os.IsNotExist(err) {
			t.Error("Corrupted file should have been cleaned up")
		}
	})

	t.Run("verification_with_empty_hash", func(t *testing.T) {
		// Test data without expected hash (should skip verification)
		testData := []byte("File without hash verification.")
		fileName := "no_hash_file.txt"
		fileID := "no_hash_file_1"

		// Create chunk message without expected hash
		chunkMsg := &transfer.ChunkMessage{
			Type:       transfer.ChunkData,
			FileID:     fileID,
			FileName:   fileName,
			SequenceNo: 1,
			Data:       testData,
			TotalSize:  int64(len(testData)),
			// ExpectedHash is empty - should skip verification
		}

		// Serialize and process chunk
		serializer := transfer.NewJSONSerializer()
		data, err := serializer.Marshal(chunkMsg)
		if err != nil {
			t.Fatalf("Failed to marshal chunk message: %v", err)
		}

		err = fileReceiver.ProcessChunk(data)
		if err != nil {
			t.Fatalf("Failed to process chunk without hash: %v", err)
		}

		// Verify file was created (verification should be skipped)
		outputPath := filepath.Join(tempDir, fileName)
		if _, err := os.Stat(outputPath); os.IsNotExist(err) {
			t.Fatalf("Output file was not created: %s", outputPath)
		}
	})
}

// TestFileReceiver_VerifyFileIntegrity tests the verifyFileIntegrity method directly
func TestFileReceiver_VerifyFileIntegrity(t *testing.T) {
	// Create temporary directory for test files
	tempDir, err := os.MkdirTemp("", "integrity_test")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create file receiver
	fileReceiver := NewFileReceiver(tempDir, nil)

	t.Run("valid_hash_verification", func(t *testing.T) {
		// Create test file
		testData := []byte("Test file content for hash verification")
		testFile := filepath.Join(tempDir, "valid_test.txt")
		err := os.WriteFile(testFile, testData, 0644)
		if err != nil {
			t.Fatalf("Failed to create test file: %v", err)
		}

		// Calculate expected hash
		expectedHash := calculateTestHash(testData)

		// Create file reception
		fileReception := &FileReception{
			FileName:     "valid_test.txt",
			OutputPath:   testFile,
			ExpectedHash: expectedHash,
		}

		// Test verification
		err = fileReceiver.verifyFileIntegrity(fileReception)
		if err != nil {
			t.Errorf("Verification should succeed, but got error: %v", err)
		}
	})

	t.Run("invalid_hash_verification", func(t *testing.T) {
		// Create test file
		testData := []byte("Different test file content")
		testFile := filepath.Join(tempDir, "invalid_test.txt")
		err := os.WriteFile(testFile, testData, 0644)
		if err != nil {
			t.Fatalf("Failed to create test file: %v", err)
		}

		// Use incorrect hash
		incorrectHash := "incorrect_hash_value"

		// Create file reception
		fileReception := &FileReception{
			FileName:     "invalid_test.txt",
			OutputPath:   testFile,
			ExpectedHash: incorrectHash,
		}

		// Test verification
		err = fileReceiver.verifyFileIntegrity(fileReception)
		if err == nil {
			t.Error("Verification should fail with incorrect hash")
		}

		expectedErrMsg := "file hash mismatch"
		if !containsString(err.Error(), expectedErrMsg) {
			t.Errorf("Expected error to contain '%s', got: %s", expectedErrMsg, err.Error())
		}
	})

	t.Run("nonexistent_file_verification", func(t *testing.T) {
		// Create file reception for nonexistent file
		fileReception := &FileReception{
			FileName:     "nonexistent.txt",
			OutputPath:   filepath.Join(tempDir, "nonexistent.txt"),
			ExpectedHash: "some_hash",
		}

		// Test verification
		err = fileReceiver.verifyFileIntegrity(fileReception)
		if err == nil {
			t.Error("Verification should fail for nonexistent file")
		}

		expectedErrMsg := "failed to calculate file hash"
		if !containsString(err.Error(), expectedErrMsg) {
			t.Errorf("Expected error to contain '%s', got: %s", expectedErrMsg, err.Error())
		}
	})
}

// TestFileReceiver_CleanupCorruptedFile tests the cleanup functionality
func TestFileReceiver_CleanupCorruptedFile(t *testing.T) {
	// Create temporary directory for test files
	tempDir, err := os.MkdirTemp("", "cleanup_test")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create file receiver
	fileReceiver := NewFileReceiver(tempDir, nil)

	t.Run("cleanup_existing_file", func(t *testing.T) {
		// Create test file
		testFile := filepath.Join(tempDir, "cleanup_test.txt")
		err := os.WriteFile(testFile, []byte("test content"), 0644)
		if err != nil {
			t.Fatalf("Failed to create test file: %v", err)
		}

		// Verify file exists
		if _, err := os.Stat(testFile); os.IsNotExist(err) {
			t.Fatal("Test file should exist before cleanup")
		}

		// Create file reception
		fileReception := &FileReception{
			FileName:   "cleanup_test.txt",
			OutputPath: testFile,
		}

		// Test cleanup
		err = fileReceiver.cleanupCorruptedFile(fileReception)
		if err != nil {
			t.Errorf("Cleanup should succeed, but got error: %v", err)
		}

		// Verify file was removed
		if _, err := os.Stat(testFile); !os.IsNotExist(err) {
			t.Error("File should have been removed after cleanup")
		}
	})

	t.Run("cleanup_nonexistent_file", func(t *testing.T) {
		// Create file reception for nonexistent file
		fileReception := &FileReception{
			FileName:   "nonexistent.txt",
			OutputPath: filepath.Join(tempDir, "nonexistent.txt"),
		}

		// Test cleanup (should not error)
		err = fileReceiver.cleanupCorruptedFile(fileReception)
		if err != nil {
			t.Errorf("Cleanup of nonexistent file should not error, but got: %v", err)
		}
	})

	t.Run("cleanup_empty_path", func(t *testing.T) {
		// Create file reception with empty path
		fileReception := &FileReception{
			FileName:   "empty_path.txt",
			OutputPath: "",
		}

		// Test cleanup (should not error)
		err = fileReceiver.cleanupCorruptedFile(fileReception)
		if err != nil {
			t.Errorf("Cleanup with empty path should not error, but got: %v", err)
		}
	})
}

// Helper function to calculate SHA256 hash for test data
func calculateTestHash(data []byte) string {
	hash := sha256.Sum256(data)
	return hex.EncodeToString(hash[:])
}

// Helper function to check if a string contains a substring
func containsString(str, substr string) bool {
	return len(str) >= len(substr) && (str == substr || 
		(len(str) > len(substr) && 
			(str[:len(substr)] == substr || 
			 str[len(str)-len(substr):] == substr || 
			 containsSubstring(str, substr))))
}

func containsSubstring(str, substr string) bool {
	for i := 0; i <= len(str)-len(substr); i++ {
		if str[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}