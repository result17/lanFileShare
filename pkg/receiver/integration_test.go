package receiver

import (
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/rescp17/lanFileSharer/internal/app_events/receiver"
	"github.com/rescp17/lanFileSharer/pkg/transfer"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestFileReceiver_IntegrationTest tests the complete file reception workflow with integrity verification
//nolint:gocyclo
func TestFileReceiver_IntegrationTest(t *testing.T) {
	t.Run("multi_chunk_file_with_verification", func(t *testing.T) {
		// Create temporary directory for this test
		tempDir, err := os.MkdirTemp("", "multi_chunk_test")
		require.NoError(t, err, "Failed to create temp directory")
		defer os.RemoveAll(tempDir)

		// Create UI messages channel for this test
		uiMessages := make(chan tea.Msg, 20)

		// Create file receiver for this test
		fileReceiver := NewFileReceiver(tempDir, uiMessages)
		// Create test data that will be split into multiple chunks
		testData := make([]byte, 1024) // 1KB of data
		for i := range testData {
			testData[i] = byte(i % 256)
		}
		
		expectedHash := calculateTestHash(testData)
		fileName := "multi_chunk_test.bin"
		fileID := "multi_chunk_1"
		chunkSize := 256 // Split into 4 chunks

		// Process multiple chunks
		for i := 0; i < len(testData); i += chunkSize {
			end := i + chunkSize
			if end > len(testData) {
				end = len(testData)
			}
			
			chunkData := testData[i:end]
			sequenceNo := uint32(i/chunkSize + 1)

			chunkMsg := &transfer.ChunkMessage{
				Type:         transfer.ChunkData,
				FileID:       fileID,
				FileName:     fileName,
				SequenceNo:   sequenceNo,
				Offset:       int64(i), // Set correct offset
				Data:         chunkData,
				TotalSize:    int64(len(testData)),
				ExpectedHash: expectedHash,
			}

			// Serialize and process chunk
			serializer := transfer.NewJSONSerializer()
			data, err := serializer.Marshal(chunkMsg)
			require.NoError(t, err, "Failed to marshal chunk message %d", sequenceNo)

			err = fileReceiver.ProcessChunk(data)
			require.NoError(t, err, "Failed to process chunk %d", sequenceNo)
		}

		// Verify file was created and has correct content
		outputPath := filepath.Join(tempDir, fileName)
		assert.FileExists(t, outputPath, "Output file should be created")

		// Read and verify file content
		content, err := os.ReadFile(outputPath)
		require.NoError(t, err, "Failed to read output file")

		assert.Equal(t, len(testData), len(content), "File size should match")
		assert.Equal(t, testData, content, "File content should match exactly")

		// Verify UI messages were sent
		messageCount := 0
		expectedMessages := []string{
			"Receiving file: " + fileName,
			"Verifying integrity of file: " + fileName,
			"File verified successfully: " + fileName,
			"File reception completed: " + fileName,
		}

		for i := 0; i < len(expectedMessages); i++ {
			select {
			case msg := <-uiMessages:
				statusMsg, ok := msg.(receiver.StatusUpdateMsg)
				require.True(t, ok, "Expected StatusUpdateMsg, got: %T", msg)
				assert.Equal(t, expectedMessages[i], statusMsg.Message, "UI message %d should match expected", i)
				messageCount++
			case <-time.After(2 * time.Second):
				t.Fatalf("Timeout waiting for UI message %q", expectedMessages[i])
			}
		}

		assert.Equal(t, len(expectedMessages), messageCount, "Should receive all expected UI messages")
	})

	t.Run("out_of_order_chunks_with_verification", func(t *testing.T) {
		// Create temporary directory for this test
		tempDir, err := os.MkdirTemp("", "out_of_order_test")
		require.NoError(t, err, "Failed to create temp directory")
		defer os.RemoveAll(tempDir)

		// Create UI messages channel for this test
		uiMessages := make(chan tea.Msg, 20)

		// Create file receiver for this test
		fileReceiver := NewFileReceiver(tempDir, uiMessages)

		// Create test data
		testData := []byte("This is a test for out-of-order chunk processing with integrity verification.")
		expectedHash := calculateTestHash(testData)
		fileName := "out_of_order_test.txt"
		fileID := "out_of_order_1"
		
		// Split into 3 chunks
		chunk1 := testData[:25]
		chunk2 := testData[25:50]
		chunk3 := testData[50:]

		// Process chunks out of order: 3, 1, 2
		chunks := []struct {
			seq    uint32
			offset int64
			data   []byte
		}{
			{3, 50, chunk3}, // chunk3 starts at offset 50
			{1, 0, chunk1},  // chunk1 starts at offset 0
			{2, 25, chunk2}, // chunk2 starts at offset 25
		}

		for _, chunk := range chunks {
			chunkMsg := &transfer.ChunkMessage{
				Type:         transfer.ChunkData,
				FileID:       fileID,
				FileName:     fileName,
				SequenceNo:   chunk.seq,
				Offset:       chunk.offset,
				Data:         chunk.data,
				TotalSize:    int64(len(testData)),
				ExpectedHash: expectedHash,
			}

			// Serialize and process chunk
			serializer := transfer.NewJSONSerializer()
			data, err := serializer.Marshal(chunkMsg)
			require.NoError(t, err, "Failed to marshal chunk message %d", chunk.seq)

			err = fileReceiver.ProcessChunk(data)
			require.NoError(t, err, "Failed to process chunk %d", chunk.seq)
		}

		// Verify file was created and has correct content
		outputPath := filepath.Join(tempDir, fileName)
		content, err := os.ReadFile(outputPath)
		require.NoError(t, err, "Failed to read output file")

		assert.Equal(t, string(testData), string(content), "File content should match expected data")
	})

	t.Run("concurrent_file_reception_with_verification", func(t *testing.T) {
		// Create temporary directory for this test
		tempDir, err := os.MkdirTemp("", "concurrent_test")
		require.NoError(t, err, "Failed to create temp directory")
		defer os.RemoveAll(tempDir)

		// Create UI messages channel for this test
		uiMessages := make(chan tea.Msg, 50) // Larger buffer for multiple files

		// Create file receiver for this test
		fileReceiver := NewFileReceiver(tempDir, uiMessages)

		// Test receiving multiple files concurrently
		files := []struct {
			id       string
			name     string
			content  []byte
		}{
			{"file1", "concurrent1.txt", []byte("Content of first concurrent file")},
			{"file2", "concurrent2.txt", []byte("Content of second concurrent file")},
			{"file3", "concurrent3.txt", []byte("Content of third concurrent file")},
		}
		serializer := transfer.NewJSONSerializer()
		var wg sync.WaitGroup

		// Process all files
		for _, file := range files {
			wg.Add(1)
			go func(f struct {
				id       string
				name     string
				content  []byte
			}) {
				defer wg.Done()
				expectedHash := calculateTestHash(f.content)

				chunkMsg := &transfer.ChunkMessage{
					Type:         transfer.ChunkData,
					FileID:       f.id,
					FileName:     f.name,
					SequenceNo:   1,
					Offset:       0, // Single chunk file starts at offset 0
					Data:         f.content,
					TotalSize:    int64(len(f.content)),
					ExpectedHash: expectedHash,
				}

				// Serialize and process chunk
				data, err := serializer.Marshal(chunkMsg)
				require.NoError(t, err, "Failed to marshal chunk message for %s", f.name)

				err = fileReceiver.ProcessChunk(data)
				require.NoError(t, err, "Failed to process chunk for %s", f.name)
			}(file)
		}
		wg.Wait()
		// Verify all files were created with correct content
		for _, file := range files {
			outputPath := filepath.Join(tempDir, file.name)
			content, err := os.ReadFile(outputPath)
			require.NoError(t, err, "Failed to read output file %s", file.name)

			assert.Equal(t, string(file.content), string(content), 
				"File content should match for %s", file.name)
		}
	})
}

// TestFileReceiver_ErrorRecovery tests error recovery scenarios
func TestFileReceiver_ErrorRecovery(t *testing.T) {
	// Create temporary directory for test files
	tempDir, err := os.MkdirTemp("", "error_recovery_test")
	require.NoError(t, err, "Failed to create temp directory")
	defer os.RemoveAll(tempDir)

	// Create UI messages channel for testing
	uiMessages := make(chan tea.Msg, 10)

	// Create file receiver
	fileReceiver := NewFileReceiver(tempDir, uiMessages)

	t.Run("verification_failure_then_success", func(t *testing.T) {
		testData := []byte("Test data for error recovery scenario")
		correctHash := calculateTestHash(testData)
		incorrectHash := "incorrect_hash_value"
		fileName := "recovery_test.txt"

		// First attempt with incorrect hash (should fail)
		chunkMsg := &transfer.ChunkMessage{
			Type:         transfer.ChunkData,
			FileID:       "recovery_1",
			FileName:     fileName,
			SequenceNo:   1,
			Offset:       0,
			Data:         testData,
			TotalSize:    int64(len(testData)),
			ExpectedHash: incorrectHash,
		}

		serializer := transfer.NewJSONSerializer()
		data, err := serializer.Marshal(chunkMsg)
		require.NoError(t, err, "Failed to marshal chunk message")

		err = fileReceiver.ProcessChunk(data)
		require.Error(t, err, "Expected error for incorrect hash")

		// Verify file was cleaned up
		outputPath := filepath.Join(tempDir, fileName)
		assert.NoFileExists(t, outputPath, "Corrupted file should have been cleaned up")

		// Second attempt with correct hash (should succeed)
		chunkMsg.FileID = "recovery_2"
		chunkMsg.ExpectedHash = correctHash

		data, err = serializer.Marshal(chunkMsg)
		require.NoError(t, err, "Failed to marshal chunk message")

		err = fileReceiver.ProcessChunk(data)
		require.NoError(t, err, "Second attempt should succeed")

		// Verify file was created successfully
		assert.FileExists(t, outputPath, "File should have been created successfully on second attempt")

		// Verify content
		content, err := os.ReadFile(outputPath)
		require.NoError(t, err, "Failed to read output file")

		assert.Equal(t, string(testData), string(content), "File content should match expected data")
	})
}