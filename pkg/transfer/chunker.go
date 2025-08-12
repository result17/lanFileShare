package transfer

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"

	"github.com/rescp17/lanFileSharer/pkg/fileInfo"
)

type Chunk struct {
	SequenceNo uint32
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

// Chunk size constants are now defined in config.go to avoid duplication

func NewChunkerFromFileNode(node *fileInfo.FileNode, chunkSize int32) (*Chunker, error) {
	if node.IsDir {
		return nil, fmt.Errorf("cannot chunk a directory")
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
	if err != nil {
		return nil, err
	}

	c.bytesRead += int64(n)
	c.currentSeq++

	hash := sha256.Sum256(c.buffer[:n])
	hashStr := hex.EncodeToString(hash[:])

	return &Chunk{
		SequenceNo: c.currentSeq,
		Data:       c.buffer[:n],
		Hash:       hashStr,
		IsLast:     c.bytesRead >= c.totalByteSize,
		Size:       int32(n),
	}, nil
}

func (c *Chunker) Close() error {
	return c.file.Close()
}
