package receiver

import (
	"os"
	"path/filepath"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/rescp17/lanFileSharer/internal/app_events/receiver"
	"github.com/rescp17/lanFileSharer/pkg/transfer"
)

// TestFileReceiver_IntegrationTest tests the complete file reception workflow with integrity verification
func TestFileReceiver_IntegrationTest(t *testing.T) {
	// Create temporary directory for test files
	tempDir, err := os.MkdirTemp("", "integration_test")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create UI messages channel for testing
	uiMessages := make(chan tea.Msg, 20)

	// Create file receiver
	fileReceiver := NewFileReceiver(tempDir, uiMessages)

	t.Run("multi_chunk_file_with_verification", func(t *testing.T) {
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
				Data:         chunkData,
				TotalSize:    int64(len(testData)),
				ExpectedHash: expectedHash,
			}

			// Serialize and process chunk
			serializer := transfer.NewJSONSerializer()
			data, err := serializer.Marshal(chunkMsg)
			if err != nil {
				t.Fatalf("Failed to marshal chunk message %d: %v", sequenceNo, err)
			}

			err = fileReceiver.ProcessChunk(data)
			if err != nil {
				t.Fatalf("Failed to process chunk %d: %v", sequenceNo, err)
			}
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

		if len(content) != len(testData) {
			t.Errorf("File size mismatch. Expected: %d, Got: %d", len(testData), len(content))
		}

		for i, b := range content {
			if b != testData[i] {
				t.Errorf("File content mismatch at byte %d. Expected: %d, Got: %d", i, testData[i], b)
				break
			}
		}

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
				if statusMsg, ok := msg.(receiver.StatusUpdateMsg); ok {
					if statusMsg.Message != expectedMessages[i] {
						t.Errorf("Expected UI message %d: %s, Got: %s", i, expectedMessages[i], statusMsg.Message)
					}
					messageCount++
				} else {
					t.Errorf("Expected StatusUpdateMsg, got: %T", msg)
				}
			default:
				t.Errorf("Expected UI message %d: %s", i, expectedMessages[i])
			}
		}

		if messageCount != len(expectedMessages) {
			t.Errorf("Expected %d UI messages, got %d", len(expectedMessages), messageCount)
		}
	})

	t.Run("out_of_order_chunks_with_verification", func(t *testing.T) {
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
			seq  uint32
			data []byte
		}{
			{3, chunk3},
			{1, chunk1},
			{2, chunk2},
		}

		for _, chunk := range chunks {
			chunkMsg := &transfer.ChunkMessage{
				Type:         transfer.ChunkData,
				FileID:       fileID,
				FileName:     fileName,
				SequenceNo:   chunk.seq,
				Data:         chunk.data,
				TotalSize:    int64(len(testData)),
				ExpectedHash: expectedHash,
			}

			// Serialize and process chunk
			serializer := transfer.NewJSONSerializer()
			data, err := serializer.Marshal(chunkMsg)
			if err != nil {
				t.Fatalf("Failed to marshal chunk message %d: %v", chunk.seq, err)
			}

			err = fileReceiver.ProcessChunk(data)
			if err != nil {
				t.Fatalf("Failed to process chunk %d: %v", chunk.seq, err)
			}
		}

		// Verify file was created and has correct content
		outputPath := filepath.Join(tempDir, fileName)
		content, err := os.ReadFile(outputPath)
		if err != nil {
			t.Fatalf("Failed to read output file: %v", err)
		}

		if string(content) != string(testData) {
			t.Errorf("File content mismatch. Expected: %s, Got: %s", string(testData), string(content))
		}
	})

	t.Run("concurrent_file_reception_with_verification", func(t *testing.T) {
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

		// Process all files
		for _, file := range files {
			expectedHash := calculateTestHash(file.content)
			
			chunkMsg := &transfer.ChunkMessage{
				Type:         transfer.ChunkData,
				FileID:       file.id,
				FileName:     file.name,
				SequenceNo:   1,
				Data:         file.content,
				TotalSize:    int64(len(file.content)),
				ExpectedHash: expectedHash,
			}

			// Serialize and process chunk
			serializer := transfer.NewJSONSerializer()
			data, err := serializer.Marshal(chunkMsg)
			if err != nil {
				t.Fatalf("Failed to marshal chunk message for %s: %v", file.name, err)
			}

			err = fileReceiver.ProcessChunk(data)
			if err != nil {
				t.Fatalf("Failed to process chunk for %s: %v", file.name, err)
			}
		}

		// Verify all files were created with correct content
		for _, file := range files {
			outputPath := filepath.Join(tempDir, file.name)
			content, err := os.ReadFile(outputPath)
			if err != nil {
				t.Fatalf("Failed to read output file %s: %v", file.name, err)
			}

			if string(content) != string(file.content) {
				t.Errorf("File content mismatch for %s. Expected: %s, Got: %s", 
					file.name, string(file.content), string(content))
			}
		}
	})
}

// TestFileReceiver_ErrorRecovery tests error recovery scenarios
func TestFileReceiver_ErrorRecovery(t *testing.T) {
	// Create temporary directory for test files
	tempDir, err := os.MkdirTemp("", "error_recovery_test")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}
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
			Data:         testData,
			TotalSize:    int64(len(testData)),
			ExpectedHash: incorrectHash,
		}

		serializer := transfer.NewJSONSerializer()
		data, err := serializer.Marshal(chunkMsg)
		if err != nil {
			t.Fatalf("Failed to marshal chunk message: %v", err)
		}

		err = fileReceiver.ProcessChunk(data)
		if err == nil {
			t.Fatal("Expected error for incorrect hash, but got none")
		}

		// Verify file was cleaned up
		outputPath := filepath.Join(tempDir, fileName)
		if _, err := os.Stat(outputPath); !os.IsNotExist(err) {
			t.Error("Corrupted file should have been cleaned up")
		}

		// Second attempt with correct hash (should succeed)
		chunkMsg.FileID = "recovery_2"
		chunkMsg.ExpectedHash = correctHash

		data, err = serializer.Marshal(chunkMsg)
		if err != nil {
			t.Fatalf("Failed to marshal chunk message: %v", err)
		}

		err = fileReceiver.ProcessChunk(data)
		if err != nil {
			t.Fatalf("Second attempt should succeed, but got error: %v", err)
		}

		// Verify file was created successfully
		if _, err := os.Stat(outputPath); os.IsNotExist(err) {
			t.Error("File should have been created successfully on second attempt")
		}

		// Verify content
		content, err := os.ReadFile(outputPath)
		if err != nil {
			t.Fatalf("Failed to read output file: %v", err)
		}

		if string(content) != string(testData) {
			t.Errorf("File content mismatch. Expected: %s, Got: %s", string(testData), string(content))
		}
	})
}