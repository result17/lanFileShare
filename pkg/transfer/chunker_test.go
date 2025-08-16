package transfer

import (
	"crypto/sha256"
	"encoding/hex"
	"io"
	"os"
	"path/filepath"
	"testing"
	"bytes"

	"github.com/rescp17/lanFileSharer/pkg/fileInfo"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
	require.NoError(t, err, "Failed to create chunker")
	defer chunker.Close()

	// Verify chunker properties
	assert.Equal(t, node.Checksum, chunker.expectedHash, "Expected hash mismatch")
	assert.Equal(t, int32(DefaultChunkSize), chunker.chunkSize, "Expected chunk size mismatch")
	assert.Equal(t, node.Size, chunker.totalByteSize, "Expected total size mismatch")
	assert.Equal(t, uint32(0), chunker.currentSeq, "Expected initial sequence mismatch")
	assert.Equal(t, int64(0), chunker.bytesRead, "Expected initial bytes read mismatch")
}

func TestNewChunkerFromFileNode_DirectoryError(t *testing.T) {
	// Create temporary directory
	tempDir, err := os.MkdirTemp("", "chunker-test-dir-*")
	require.NoError(t, err, "Failed to create temp dir")
	defer os.RemoveAll(tempDir)

	// Create FileNode for directory
	node := createFileNode(t, tempDir)

	// Attempt to create Chunker for directory
	chunker, err := NewChunkerFromFileNode(node, DefaultChunkSize)
	if err == nil {
		chunker.Close()
	}
	require.Error(t, err, "Expected error when creating chunker for directory")

	assert.Equal(t, ErrIsDir, err, "Expected specific error message")
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
				assert.Error(t, err, "Expected error for chunk size %d", tc.chunkSize)
				if chunker != nil {
					chunker.Close()
				}
			} else {
				assert.NoError(t, err, "Unexpected error for chunk size %d", tc.chunkSize)
				if chunker != nil {
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
	require.NoError(t, err, "Failed to create chunker")
	defer chunker.Close()

	// Read first (and only) chunk
	chunk, err := chunker.Next()
	require.NoError(t, err, "Failed to read chunk")

	// Verify chunk properties
	assert.Equal(t, uint32(1), chunk.SequenceNo, "Expected sequence number 1")
	assert.Equal(t, string(content), string(chunk.Data), "Expected data mismatch")
	assert.True(t, chunk.IsLast, "Expected chunk to be marked as last")
	assert.Equal(t, int32(len(content)), chunk.Size, "Expected size mismatch")

	// Verify hash
	expectedHash := sha256.Sum256(content)
	expectedHashStr := hex.EncodeToString(expectedHash[:])
	assert.Equal(t, expectedHashStr, chunk.Hash, "Expected hash mismatch")

	// Try to read next chunk (should return EOF)
	_, err = chunker.Next()
	assert.Equal(t, io.EOF, err, "Expected EOF when reading beyond file")
}

//nolint:gocyclo // Test function complexity is acceptable
func TestChunker_Next_LargeFile(t *testing.T) {
	// Create large test file (larger than chunk size)
	chunkSize := int32(4096)       // Minimum valid chunk size
	content := make([]byte, 12288) // 12KB content (3 chunks of 4KB each)

	var reconstructed bytes.Buffer
	reconstructed.Grow(len(content))

	for i := range content {
		content[i] = byte(i % 256) // Fill with pattern
	}
	filePath, cleanup := setupTestFile(t, content)
	defer cleanup()

	node := createFileNode(t, filePath)
	chunker, err := NewChunkerFromFileNode(node, chunkSize)
	require.NoError(t, err, "Failed to create chunker")
	defer chunker.Close()

	var chunks []*Chunk
	var totalBytesRead int32

	// Read all chunks
	for {
		chunk, err := chunker.Next()
		if err == io.EOF {
			break
		}
		require.NoError(t, err, "Failed to read chunk")

		chunks = append(chunks, chunk)
		totalBytesRead += chunk.Size

		// Verify sequence number
		expectedSeq := uint32(len(chunks))
		assert.Equal(t, expectedSeq, chunk.SequenceNo, "Expected sequence number mismatch")

		// Verify chunk size (except possibly the last chunk)
		if !chunk.IsLast {
			assert.Equal(t, chunkSize, chunk.Size, "Expected chunk size mismatch")
		}
	}

	// Verify total chunks and bytes
	expectedChunks := 3 // 12KB / 4KB per chunk = 3 chunks
	assert.Equal(t, expectedChunks, len(chunks), "Expected chunks count mismatch")
	assert.Equal(t, int32(len(content)), totalBytesRead, "Expected total bytes mismatch")

	// Verify last chunk is marked as last
	assert.True(t, chunks[len(chunks)-1].IsLast, "Last chunk should be marked as last")

	// Verify all chunks except last are not marked as last
	for i := 0; i < len(chunks)-1; i++ {
		assert.False(t, chunks[i].IsLast, "Chunk %d should not be marked as last", i)
	}

	for _, chunk := range chunks {
		reconstructed.Write(chunk.Data)
	}

	assert.Equal(t, content, reconstructed.Bytes(), "Reconstructed content doesn't match original")
}

func TestChunker_Next_EmptyFile(t *testing.T) {
	// Create empty test file
	content := []byte{}
	filePath, cleanup := setupTestFile(t, content)
	defer cleanup()

	node := createFileNode(t, filePath)
	chunker, err := NewChunkerFromFileNode(node, DefaultChunkSize)
	require.NoError(t, err, "Failed to create chunker")
	defer chunker.Close()

	// Try to read from empty file
	_, err = chunker.Next()
	assert.Equal(t, io.EOF, err, "Expected EOF when reading empty file")
}

func TestChunker_Close(t *testing.T) {
	// Create test file
	content := []byte("test content")
	filePath, cleanup := setupTestFile(t, content)
	defer cleanup()

	node := createFileNode(t, filePath)
	chunker, err := NewChunkerFromFileNode(node, DefaultChunkSize)
	require.NoError(t, err, "Failed to create chunker")

	// Close chunker
	err = chunker.Close()
	assert.NoError(t, err, "Failed to close chunker")

	// Try to read after close (should fail)
	_, err = chunker.Next()
	assert.Error(t, err, "Expected error when reading from closed chunker")
}

func TestChunker_HashConsistency(t *testing.T) {
	// Create test file with known content
	content := []byte("consistent hash test content")
	filePath, cleanup := setupTestFile(t, content)
	defer cleanup()

	node := createFileNode(t, filePath)

	// Create two chunkers with same parameters
	chunker1, err := NewChunkerFromFileNode(node, DefaultChunkSize)
	require.NoError(t, err, "Failed to create first chunker")
	defer chunker1.Close()

	chunker2, err := NewChunkerFromFileNode(node, DefaultChunkSize)
	require.NoError(t, err, "Failed to create second chunker")
	defer chunker2.Close()

	// Read chunks from both chunkers
	chunk1, err := chunker1.Next()
	require.NoError(t, err, "Failed to read from first chunker")

	chunk2, err := chunker2.Next()
	require.NoError(t, err, "Failed to read from second chunker")

	// Verify hashes are identical
	assert.Equal(t, chunk1.Hash, chunk2.Hash, "Hash mismatch between chunkers")

	// Verify data is identical
	assert.Equal(t, string(chunk1.Data), string(chunk2.Data), "Data mismatch between chunkers")
}

func TestChunker_ChunkDataIsolation(t *testing.T) {
	// This test verifies that chunk data is not overwritten when multiple chunks are read
	// It addresses the bug where chunks shared the same underlying buffer
	
	// Create test file with distinct patterns for each chunk
	chunkSize := int32(4096) // Minimum valid chunk size
	numChunks := 3
	totalSize := int(chunkSize) * numChunks
	
	content := make([]byte, totalSize)
	// Fill each chunk with a distinct pattern
	for i := 0; i < numChunks; i++ {
		pattern := byte('A' + i) // A, B, C
		start := i * int(chunkSize)
		end := start + int(chunkSize)
		for j := start; j < end; j++ {
			content[j] = pattern
		}
	}
	
	filePath, cleanup := setupTestFile(t, content)
	defer cleanup()
	
	node := createFileNode(t, filePath)
	chunker, err := NewChunkerFromFileNode(node, chunkSize)
	require.NoError(t, err, "Failed to create chunker")
	defer chunker.Close()
	
	// Read all chunks and store them
	var chunks []*Chunk
	for {
		chunk, err := chunker.Next()
		if err == io.EOF {
			break
		}
		require.NoError(t, err, "Failed to read chunk")
		chunks = append(chunks, chunk)
	}
	
	// Verify we got the expected number of chunks
	require.Equal(t, numChunks, len(chunks), "Expected %d chunks, got %d", numChunks, len(chunks))
	
	// Verify that each chunk still contains its original data
	// This is the critical test - if buffer reuse bug exists, all chunks would have the same data
	for i, chunk := range chunks {
		expectedPattern := byte('A' + i)
		
		// Check that the entire chunk contains the expected pattern
		for j, b := range chunk.Data {
			if b != expectedPattern {
				t.Errorf("Chunk %d, byte %d: expected %c, got %c", i, j, expectedPattern, b)
				break // Don't spam with too many errors
			}
		}
		
		// Also verify chunk metadata
		assert.Equal(t, uint32(i+1), chunk.SequenceNo, "Chunk %d has wrong sequence number", i)
		assert.Equal(t, int64(i)*int64(chunkSize), chunk.Offset, "Chunk %d has wrong offset", i)
		assert.Equal(t, chunkSize, chunk.Size, "Chunk %d has wrong size", i)
		
		// Verify IsLast flag
		expectedIsLast := (i == numChunks-1)
		assert.Equal(t, expectedIsLast, chunk.IsLast, "Chunk %d has wrong IsLast flag", i)
	}
	
	// Additional verification: check that chunks have different data
	for i := 0; i < len(chunks)-1; i++ {
		for j := i + 1; j < len(chunks); j++ {
			// Chunks should have different data (different patterns)
			assert.NotEqual(t, string(chunks[i].Data), string(chunks[j].Data), 
				"Chunk %d and chunk %d should have different data", i, j)
		}
	}
	
	// Verify that we can reconstruct the original content
	var reconstructed []byte
	for _, chunk := range chunks {
		reconstructed = append(reconstructed, chunk.Data...)
	}
	assert.Equal(t, content, reconstructed, "Reconstructed content doesn't match original")
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
	chunker, err := NewChunkerFromFileNode(node, 256*1024) // 256KB chunks
	if err != nil {
		b.Fatalf("Failed to create chunker: %v", err)
	}
	defer chunker.Close()

	b.ResetTimer()

	for i := 0; i < b.N; i++ {

		for {
			_, err := chunker.Next()
			if err == io.EOF {
				break
			}
			if err != nil {
				b.Fatalf("Failed to read chunk: %v", err)
			}
		}
		chunker.file.Seek(0, io.SeekStart)
		chunker.currentSeq = 0
	}
}
