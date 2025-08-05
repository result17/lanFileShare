package transfer

import (
	"crypto/sha256"
	"encoding/hex"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/rescp17/lanFileSharer/pkg/fileInfo"
)

// setupTestFile creates a temporary file with specified content for testing
// Works with both *testing.T and *testing.B using the common testing.TB interface
func setupTestFile(tb testing.TB, content []byte) (string, func()) {
	tb.Helper()
	
	tempDir, err := os.MkdirTemp("", "chunker-test-*")
	if err != nil {
		tb.Fatalf("Failed to create temp dir: %v", err)
	}
	
	filePath := filepath.Join(tempDir, "test-file.txt")
	if err := os.WriteFile(filePath, content, 0644); err != nil {
		tb.Fatalf("Failed to create test file: %v", err)
	}
	
	cleanup := func() {
		if err := os.RemoveAll(tempDir); err != nil {
			tb.Errorf("Failed to clean up temp dir: %v", err)
		}
	}
	
	return filePath, cleanup
}

// createFileNode creates a FileNode from a file path for testing
// Works with both *testing.T and *testing.B using the common testing.TB interface
func createFileNode(tb testing.TB, filePath string) *fileInfo.FileNode {
	tb.Helper()
	
	node, err := fileInfo.CreateNode(filePath)
	if err != nil {
		tb.Fatalf("Failed to create FileNode: %v", err)
	}
	
	return &node
}

func TestNewChunkerFromFileNode_Success(t *testing.T) {
	// Create test file
	content := []byte("Hello, World! This is a test file for chunking.")
	filePath, cleanup := setupTestFile(t, content)
	defer cleanup()
	
	// Create FileNode
	node := createFileNode(t, filePath)
	
	// Create Chunker
	chunker, err := NewChunkerFromFileNode(node, DefaultChunkSize)
	if err != nil {
		t.Fatalf("Failed to create chunker: %v", err)
	}
	defer chunker.Close()
	
	// Verify chunker properties
	if chunker.expectedHash != node.Checksum {
		t.Errorf("Expected hash %s, got %s", node.Checksum, chunker.expectedHash)
	}
	
	if chunker.chunkSize != DefaultChunkSize {
		t.Errorf("Expected chunk size %d, got %d", DefaultChunkSize, chunker.chunkSize)
	}
	
	if chunker.totalByteSize != node.Size {
		t.Errorf("Expected total size %d, got %d", node.Size, chunker.totalByteSize)
	}
	
	if chunker.currentSeq != 0 {
		t.Errorf("Expected initial sequence 0, got %d", chunker.currentSeq)
	}
	
	if chunker.bytesRead != 0 {
		t.Errorf("Expected initial bytes read 0, got %d", chunker.bytesRead)
	}
}

func TestNewChunkerFromFileNode_DirectoryError(t *testing.T) {
	// Create temporary directory
	tempDir, err := os.MkdirTemp("", "chunker-test-dir-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)
	
	// Create FileNode for directory
	node := createFileNode(t, tempDir)
	
	// Attempt to create Chunker for directory
	chunker, err := NewChunkerFromFileNode(node, DefaultChunkSize)
	if err == nil {
		chunker.Close()
		t.Fatal("Expected error when creating chunker for directory, got nil")
	}
	
	expectedError := "cannot chunk a directory"
	if err.Error() != expectedError {
		t.Errorf("Expected error '%s', got '%s'", expectedError, err.Error())
	}
}

func TestNewChunkerFromFileNode_InvalidChunkSize(t *testing.T) {
	// Create test file
	content := []byte("test content")
	filePath, cleanup := setupTestFile(t, content)
	defer cleanup()
	
	node := createFileNode(t, filePath)
	
	testCases := []struct {
		name      string
		chunkSize int32
		wantError bool
	}{
		{"Too small", MinChunkSize - 1, true},
		{"Too large", MaxChunkSize + 1, true},
		{"Minimum valid", MinChunkSize, false},
		{"Maximum valid", MaxChunkSize, false},
		{"Default", DefaultChunkSize, false},
	}
	
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			chunker, err := NewChunkerFromFileNode(node, tc.chunkSize)
			
			if tc.wantError {
				if err == nil {
					chunker.Close()
					t.Errorf("Expected error for chunk size %d, got nil", tc.chunkSize)
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error for chunk size %d: %v", tc.chunkSize, err)
				} else {
					chunker.Close()
				}
			}
		})
	}
}

func TestChunker_Next_SmallFile(t *testing.T) {
	// Create small test file (smaller than default chunk size)
	content := []byte("Hello, World!")
	filePath, cleanup := setupTestFile(t, content)
	defer cleanup()
	
	node := createFileNode(t, filePath)
	chunker, err := NewChunkerFromFileNode(node, DefaultChunkSize)
	if err != nil {
		t.Fatalf("Failed to create chunker: %v", err)
	}
	defer chunker.Close()
	
	// Read first (and only) chunk
	chunk, err := chunker.Next()
	if err != nil {
		t.Fatalf("Failed to read chunk: %v", err)
	}
	
	// Verify chunk properties
	if chunk.SequenceNo != 1 {
		t.Errorf("Expected sequence number 1, got %d", chunk.SequenceNo)
	}
	
	if string(chunk.Data) != string(content) {
		t.Errorf("Expected data %q, got %q", string(content), string(chunk.Data))
	}
	
	if !chunk.IsLast {
		t.Error("Expected chunk to be marked as last")
	}
	
	if chunk.Size != int32(len(content)) {
		t.Errorf("Expected size %d, got %d", len(content), chunk.Size)
	}
	
	// Verify hash
	expectedHash := sha256.Sum256(content)
	expectedHashStr := hex.EncodeToString(expectedHash[:])
	if chunk.Hash != expectedHashStr {
		t.Errorf("Expected hash %s, got %s", expectedHashStr, chunk.Hash)
	}
	
	// Try to read next chunk (should return EOF)
	_, err = chunker.Next()
	if err != io.EOF {
		t.Errorf("Expected EOF when reading beyond file, got %v", err)
	}
}

func TestChunker_Next_LargeFile(t *testing.T) {
	// Create large test file (larger than chunk size)
	chunkSize := int32(4096) // Minimum valid chunk size
	content := make([]byte, 12288) // 12KB content (3 chunks of 4KB each)
	for i := range content {
		content[i] = byte(i % 256) // Fill with pattern
	}
	filePath, cleanup := setupTestFile(t, content)
	defer cleanup()
	
	node := createFileNode(t, filePath)
	chunker, err := NewChunkerFromFileNode(node, chunkSize)
	if err != nil {
		t.Fatalf("Failed to create chunker: %v", err)
	}
	defer chunker.Close()
	
	var chunks []*Chunk
	var totalBytesRead int32
	
	// Read all chunks
	for {
		chunk, err := chunker.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("Failed to read chunk: %v", err)
		}
		
		chunks = append(chunks, chunk)
		totalBytesRead += chunk.Size
		
		// Verify sequence number
		expectedSeq := uint32(len(chunks))
		if chunk.SequenceNo != expectedSeq {
			t.Errorf("Expected sequence number %d, got %d", expectedSeq, chunk.SequenceNo)
		}
		
		// Verify chunk size (except possibly the last chunk)
		if !chunk.IsLast && chunk.Size != chunkSize {
			t.Errorf("Expected chunk size %d, got %d", chunkSize, chunk.Size)
		}
	}
	
	// Verify total chunks and bytes
	expectedChunks := 3 // 12KB / 4KB per chunk = 3 chunks
	if len(chunks) != expectedChunks {
		t.Errorf("Expected %d chunks, got %d", expectedChunks, len(chunks))
	}
	
	if totalBytesRead != int32(len(content)) {
		t.Errorf("Expected total bytes %d, got %d", len(content), totalBytesRead)
	}
	
	// Verify last chunk is marked as last
	if !chunks[len(chunks)-1].IsLast {
		t.Error("Last chunk should be marked as last")
	}
	
	// Verify all chunks except last are not marked as last
	for i := 0; i < len(chunks)-1; i++ {
		if chunks[i].IsLast {
			t.Errorf("Chunk %d should not be marked as last", i)
		}
	}
	
	// Reconstruct content from chunks
	var reconstructed []byte
	for _, chunk := range chunks {
		reconstructed = append(reconstructed, chunk.Data...)
	}
	
	if string(reconstructed) != string(content) {
		t.Errorf("Reconstructed content doesn't match original.\nExpected: %q\nGot: %q", 
			string(content), string(reconstructed))
	}
}

func TestChunker_Next_EmptyFile(t *testing.T) {
	// Create empty test file
	content := []byte{}
	filePath, cleanup := setupTestFile(t, content)
	defer cleanup()
	
	node := createFileNode(t, filePath)
	chunker, err := NewChunkerFromFileNode(node, DefaultChunkSize)
	if err != nil {
		t.Fatalf("Failed to create chunker: %v", err)
	}
	defer chunker.Close()
	
	// Try to read from empty file
	_, err = chunker.Next()
	if err != io.EOF {
		t.Errorf("Expected EOF when reading empty file, got %v", err)
	}
}

func TestChunker_Close(t *testing.T) {
	// Create test file
	content := []byte("test content")
	filePath, cleanup := setupTestFile(t, content)
	defer cleanup()
	
	node := createFileNode(t, filePath)
	chunker, err := NewChunkerFromFileNode(node, DefaultChunkSize)
	if err != nil {
		t.Fatalf("Failed to create chunker: %v", err)
	}
	
	// Close chunker
	err = chunker.Close()
	if err != nil {
		t.Errorf("Failed to close chunker: %v", err)
	}
	
	// Try to read after close (should fail)
	_, err = chunker.Next()
	if err == nil {
		t.Error("Expected error when reading from closed chunker, got nil")
	}
}

func TestChunker_HashConsistency(t *testing.T) {
	// Create test file with known content
	content := []byte("consistent hash test content")
	filePath, cleanup := setupTestFile(t, content)
	defer cleanup()
	
	node := createFileNode(t, filePath)
	
	// Create two chunkers with same parameters
	chunker1, err := NewChunkerFromFileNode(node, DefaultChunkSize)
	if err != nil {
		t.Fatalf("Failed to create first chunker: %v", err)
	}
	defer chunker1.Close()
	
	chunker2, err := NewChunkerFromFileNode(node, DefaultChunkSize)
	if err != nil {
		t.Fatalf("Failed to create second chunker: %v", err)
	}
	defer chunker2.Close()
	
	// Read chunks from both chunkers
	chunk1, err := chunker1.Next()
	if err != nil {
		t.Fatalf("Failed to read from first chunker: %v", err)
	}
	
	chunk2, err := chunker2.Next()
	if err != nil {
		t.Fatalf("Failed to read from second chunker: %v", err)
	}
	
	// Verify hashes are identical
	if chunk1.Hash != chunk2.Hash {
		t.Errorf("Hash mismatch between chunkers.\nChunker1: %s\nChunker2: %s", 
			chunk1.Hash, chunk2.Hash)
	}
	
	// Verify data is identical
	if string(chunk1.Data) != string(chunk2.Data) {
		t.Errorf("Data mismatch between chunkers.\nChunker1: %q\nChunker2: %q", 
			string(chunk1.Data), string(chunk2.Data))
	}
}

// Benchmark tests
func BenchmarkChunker_Next_SmallChunks(b *testing.B) {
	// Create 1MB test file
	content := make([]byte, 1024*1024)
	for i := range content {
		content[i] = byte(i % 256)
	}
	
	filePath, cleanup := setupTestFile(b, content)
	defer cleanup()
	
	node := createFileNode(b, filePath)
	
	b.ResetTimer()
	
	for i := 0; i < b.N; i++ {
		chunker, err := NewChunkerFromFileNode(node, 4*1024) // 4KB chunks
		if err != nil {
			b.Fatalf("Failed to create chunker: %v", err)
		}
		
		for {
			_, err := chunker.Next()
			if err == io.EOF {
				break
			}
			if err != nil {
				b.Fatalf("Failed to read chunk: %v", err)
			}
		}
		
		chunker.Close()
	}
}

func BenchmarkChunker_Next_LargeChunks(b *testing.B) {
	// Create 1MB test file
	content := make([]byte, 1024*1024)
	for i := range content {
		content[i] = byte(i % 256)
	}
	
	filePath, cleanup := setupTestFile(b, content)
	defer cleanup()
	
	node := createFileNode(b, filePath)
	
	b.ResetTimer()
	
	for i := 0; i < b.N; i++ {
		chunker, err := NewChunkerFromFileNode(node, 256*1024) // 256KB chunks
		if err != nil {
			b.Fatalf("Failed to create chunker: %v", err)
		}
		
		for {
			_, err := chunker.Next()
			if err == io.EOF {
				break
			}
			if err != nil {
				b.Fatalf("Failed to read chunk: %v", err)
			}
		}
		
		chunker.Close()
	}
}