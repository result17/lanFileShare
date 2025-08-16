package transfer

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"errors"

	"github.com/rescp17/lanFileSharer/pkg/fileInfo"
)

type Chunk struct {
	SequenceNo uint32
	Offset     int64    // File offset
	Data       []byte
	Hash       string
	IsLast     bool
	Size       int32
}

type Chunker struct {
	file          *os.File
	expectedHash  string
	chunkSize     int32
	currentSeq    uint32
	totalByteSize int64
	bytesRead     int64
	buffer        []byte
}

var ErrIsDir = errors.New("cannot chunk a directory")

// Chunk size constants are now defined in config.go to avoid duplication

func NewChunkerFromFileNode(node *fileInfo.FileNode, chunkSize int32) (*Chunker, error) {
	if node.IsDir {
		return nil, ErrIsDir
	}

	if chunkSize < MinChunkSize || chunkSize > MaxChunkSize {
		return nil, fmt.Errorf("chunk size must be between %d and %d", MinChunkSize, MaxChunkSize)
	}
	file, err := os.Open(node.Path)
	if err != nil {
		return nil, err
	}

	return &Chunker{
		file:          file,
		expectedHash:  node.Checksum,
		chunkSize:     chunkSize,
		currentSeq:    0,
		totalByteSize: node.Size,
		bytesRead:     0,
		buffer:        make([]byte, chunkSize),
	}, nil
}

func (c *Chunker) Next() (*Chunk, error) {
	if c.bytesRead >= c.totalByteSize {
		return nil, io.EOF
	}

	n, err := c.file.Read(c.buffer)

	if n > 0 {
		c.bytesRead += int64(n)
		c.currentSeq++

		hash := sha256.Sum256(c.buffer[:n])
		hashStr := hex.EncodeToString(hash[:])

		// Calculate the offset for the current chunk
		offset := c.bytesRead - int64(n)
		
		// Create a copy of the data to avoid buffer reuse issues
		data := make([]byte, n)
		copy(data, c.buffer[:n])
		
		return &Chunk{
			SequenceNo: c.currentSeq,
			Offset:     offset,
			Data:       data,
			Hash:       hashStr,
			IsLast:     c.bytesRead >= c.totalByteSize,
			Size:       int32(n),
		}, nil
		}

	if err == io.EOF {
		return nil, io.EOF
	}
	
	return nil, err
}

func (c *Chunker) Close() error {
	return c.file.Close()
}
