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
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestFileReceiver_IntegrityVerification tests the integrity verification functionality
//nolint:gocyclo
func TestFileReceiver_IntegrityVerification(t *testing.T) {
	// Create temporary directory for test files
	tempDir, err := os.MkdirTemp("", "file_receiver_test")
	require.NoError(t, err, "Failed to create temp directory")
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
			Offset:       0, // First chunk starts at offset 0
			Data:         testData,
			TotalSize:    int64(len(testData)),
			ExpectedHash: expectedHash,
		}

		// Serialize and process chunk
		serializer := transfer.NewJSONSerializer()
		data, err := serializer.Marshal(chunkMsg)
		require.NoError(t, err, "Failed to marshal chunk message")

		err = fileReceiver.ProcessChunk(data)
		require.NoError(t, err, "Failed to process chunk")

		// Verify file was created and has correct content
		outputPath := filepath.Join(tempDir, fileName)
		assert.FileExists(t, outputPath, "Output file should be created")

		// Read and verify file content
		content, err := os.ReadFile(outputPath)
		require.NoError(t, err, "Failed to read output file")

		assert.Equal(t, string(testData), string(content), "File content should match expected data")

		// Check UI messages
		select {
		case msg := <-uiMessages:
			statusMsg, ok := msg.(receiver.StatusUpdateMsg)
			require.True(t, ok, "Expected StatusUpdateMsg, got: %T", msg)
			
			expectedMsg := fmt.Sprintf("Receiving file: %s", fileName)
			assert.Equal(t, expectedMsg, statusMsg.Message, "UI message should match expected")
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
			Offset:       0,
			Data:         testData,
			TotalSize:    int64(len(testData)),
			ExpectedHash: incorrectHash,
		}

		// Serialize and process chunk
		serializer := transfer.NewJSONSerializer()
		data, err := serializer.Marshal(chunkMsg)
		require.NoError(t, err, "Failed to marshal chunk message")

		err = fileReceiver.ProcessChunk(data)
		require.Error(t, err, "Expected error for corrupted file")

		// Verify error message contains verification failure
		assert.Contains(t, err.Error(), "file integrity verification failed", "Error should indicate verification failure")

		// Verify corrupted file was cleaned up
		outputPath := filepath.Join(tempDir, fileName)
		assert.NoFileExists(t, outputPath, "Corrupted file should have been cleaned up")
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
			Offset:     0,
			Data:       testData,
			TotalSize:  int64(len(testData)),
			// ExpectedHash is empty - should skip verification
		}

		// Serialize and process chunk
		serializer := transfer.NewJSONSerializer()
		data, err := serializer.Marshal(chunkMsg)
		require.NoError(t, err, "Failed to marshal chunk message")

		err = fileReceiver.ProcessChunk(data)
		require.NoError(t, err, "Failed to process chunk without hash")

		// Verify file was created (verification should be skipped)
		outputPath := filepath.Join(tempDir, fileName)
		assert.FileExists(t, outputPath, "Output file should be created even without hash")
	})
}

// TestFileReceiver_VerifyFileIntegrity tests the verifyFileIntegrity method directly
func TestFileReceiver_VerifyFileIntegrity(t *testing.T) {
	// Create temporary directory for test files
	tempDir, err := os.MkdirTemp("", "integrity_test")
	require.NoError(t, err, "Failed to create temp directory")
	defer os.RemoveAll(tempDir)

	// Create file receiver
	fileReceiver := NewFileReceiver(tempDir, nil)

	t.Run("valid_hash_verification", func(t *testing.T) {
		// Create test file
		testData := []byte("Test file content for hash verification")
		testFile := filepath.Join(tempDir, "valid_test.txt")
		err := os.WriteFile(testFile, testData, 0644)
		require.NoError(t, err, "Failed to create test file")

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
		assert.NoError(t, err, "Verification should succeed with correct hash")
	})

	t.Run("invalid_hash_verification", func(t *testing.T) {
		// Create test file
		testData := []byte("Different test file content")
		testFile := filepath.Join(tempDir, "invalid_test.txt")
		err := os.WriteFile(testFile, testData, 0644)
		require.NoError(t, err, "Failed to create test file")

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
		require.Error(t, err, "Verification should fail with incorrect hash")
		assert.Contains(t, err.Error(), "file hash mismatch", "Error should indicate hash mismatch")
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
		require.Error(t, err, "Verification should fail for nonexistent file")
		assert.Contains(t, err.Error(), "failed to calculate file hash", "Error should indicate hash calculation failure")
	})
}

// TestFileReceiver_CleanupCorruptedFile tests the cleanup functionality
func TestFileReceiver_CleanupCorruptedFile(t *testing.T) {
	// Create temporary directory for test files
	tempDir, err := os.MkdirTemp("", "cleanup_test")
	require.NoError(t, err, "Failed to create temp directory")
	defer os.RemoveAll(tempDir)

	// Create file receiver
	fileReceiver := NewFileReceiver(tempDir, nil)

	t.Run("cleanup_existing_file", func(t *testing.T) {
		// Create test file
		testFile := filepath.Join(tempDir, "cleanup_test.txt")
		err := os.WriteFile(testFile, []byte("test content"), 0644)
		require.NoError(t, err, "Failed to create test file")

		// Verify file exists
		assert.FileExists(t, testFile, "Test file should exist before cleanup")

		// Create file reception
		fileReception := &FileReception{
			FileName:   "cleanup_test.txt",
			OutputPath: testFile,
		}

		// Test cleanup
		err = fileReceiver.cleanupCorruptedFile(fileReception)
		assert.NoError(t, err, "Cleanup should succeed")

		// Verify file was removed
		assert.NoFileExists(t, testFile, "File should have been removed after cleanup")
	})

	t.Run("cleanup_nonexistent_file", func(t *testing.T) {
		// Create file reception for nonexistent file
		fileReception := &FileReception{
			FileName:   "nonexistent.txt",
			OutputPath: filepath.Join(tempDir, "nonexistent.txt"),
		}

		// Test cleanup (should not error)
		err = fileReceiver.cleanupCorruptedFile(fileReception)
		assert.NoError(t, err, "Cleanup of nonexistent file should not error")
	})

	t.Run("cleanup_empty_path", func(t *testing.T) {
		// Create file reception with empty path
		fileReception := &FileReception{
			FileName:   "empty_path.txt",
			OutputPath: "",
		}

		// Test cleanup (should not error)
		err = fileReceiver.cleanupCorruptedFile(fileReception)
		assert.NoError(t, err, "Cleanup with empty path should not error")
	})
}

// Helper function to calculate SHA256 hash for test data
func calculateTestHash(data []byte) string {
	hash := sha256.Sum256(data)
	return hex.EncodeToString(hash[:])
}