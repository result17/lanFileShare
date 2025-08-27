package transfer

import (
	"errors"
	"time"
)

// TransferConfig holds all configuration for the transfer system
// This centralizes configuration that was previously scattered across multiple files
type TransferConfig struct {
	// Chunk configuration (moved from chunker.go constants)
	ChunkSize    int32 `json:"chunk_size"`     // Size of each data chunk
	MaxChunkSize int32 `json:"max_chunk_size"` // Maximum allowed chunk size
	MinChunkSize int32 `json:"min_chunk_size"` // Minimum allowed chunk size

	// Concurrency limits
	MaxConcurrentTransfers int `json:"max_concurrent_transfers"`
	MaxConcurrentChunks    int `json:"max_concurrent_chunks"`

	// Performance settings
	BufferSize            int           `json:"buffer_size"`
	RateCalculationWindow time.Duration `json:"rate_calculation_window"`

	// Retry policy
	DefaultRetryPolicy *RetryPolicy `json:"default_retry_policy"`

	// History settings
	HistoryRetentionDays int `json:"history_retention_days"`
	MaxHistoryRecords    int `json:"max_history_records"`

	// Event settings
	EventBufferSize      int           `json:"event_buffer_size"`
	EventDeliveryTimeout time.Duration `json:"event_delivery_timeout"`
}

// Chunk size constants (moved from chunker.go)
const (
	DefaultChunkSize = 64 * 1024  // 64KB - proven optimal for most network conditions
	MaxChunkSize     = 256 * 1024 // 256KB - maximum to prevent memory issues
	MinChunkSize     = 4 * 1024   // 4KB - minimum for efficiency
)

// DefaultTransferConfig returns a configuration with sensible defaults
func DefaultTransferConfig() *TransferConfig {
	return &TransferConfig{
		// Use the proven chunk size from chunker.go
		ChunkSize:    DefaultChunkSize,
		MaxChunkSize: MaxChunkSize,
		MinChunkSize: MinChunkSize,

		// Concurrency settings
		MaxConcurrentTransfers: 10,
		MaxConcurrentChunks:    50,

		// Performance settings
		BufferSize:            8192, // 8KB
		RateCalculationWindow: 30 * time.Second,

		// Retry policy
		DefaultRetryPolicy: DefaultRetryPolicy(),

		// History settings
		HistoryRetentionDays: 30,
		MaxHistoryRecords:    1000,

		// Event settings
		EventBufferSize:      100,
		EventDeliveryTimeout: 5 * time.Second,
	}
}

// Validate checks if the configuration values are valid
func (tc *TransferConfig) Validate() error {
	// Validate chunk size settings
	if tc.ChunkSize <= 0 {
		return errors.New("chunk_size must be positive")
	}
	if tc.MinChunkSize <= 0 {
		return errors.New("min_chunk_size must be positive")
	}
	if tc.MaxChunkSize <= 0 {
		return errors.New("max_chunk_size must be positive")
	}
	if tc.ChunkSize < tc.MinChunkSize {
		return errors.New("chunk_size cannot be less than min_chunk_size")
	}
	if tc.ChunkSize > tc.MaxChunkSize {
		return errors.New("chunk_size cannot be greater than max_chunk_size")
	}
	if tc.MinChunkSize > tc.MaxChunkSize {
		return errors.New("min_chunk_size cannot be greater than max_chunk_size")
	}

	// Validate concurrency settings
	if tc.MaxConcurrentTransfers <= 0 {
		return errors.New("max_concurrent_transfers must be positive")
	}
	if tc.MaxConcurrentChunks <= 0 {
		return errors.New("max_concurrent_chunks must be positive")
	}

	// Validate performance settings
	if tc.BufferSize <= 0 {
		return errors.New("buffer_size must be positive")
	}

	// Validate retry policy
	if tc.DefaultRetryPolicy == nil {
		return errors.New("default_retry_policy cannot be nil")
	}

	// Validate history settings
	if tc.HistoryRetentionDays < 0 {
		return errors.New("history_retention_days cannot be negative")
	}
	if tc.MaxHistoryRecords < 0 {
		return errors.New("max_history_records cannot be negative")
	}

	// Validate event settings
	if tc.EventBufferSize <= 0 {
		return errors.New("event_buffer_size must be positive")
	}

	return nil
}

// GetChunkSizeForFile returns the appropriate chunk size for a given file size
// This allows for dynamic chunk size optimization based on file characteristics
func (tc *TransferConfig) GetChunkSizeForFile(fileSize int64) int32 {
	// For very small files, use smaller chunks to reduce overhead
	if fileSize < int64(tc.ChunkSize) {
		minSize := tc.MinChunkSize
		if fileSize < int64(minSize) {
			return int32(fileSize)
		}
		return minSize
	}

	// For very large files, consider using larger chunks for efficiency
	if fileSize > 100*1024*1024 { // 100MB
		// Use larger chunks for big files, but respect the maximum
		largerChunk := tc.ChunkSize * 2
		if largerChunk <= tc.MaxChunkSize {
			return largerChunk
		}
		// If calculated chunk size exceeds maximum, use the maximum
		return tc.MaxChunkSize
	}

	// Use default chunk size for most files
	return tc.ChunkSize
}

// IsValidChunkSize checks if a chunk size is within acceptable bounds
func (tc *TransferConfig) IsValidChunkSize(chunkSize int32) bool {
	return chunkSize >= tc.MinChunkSize && chunkSize <= tc.MaxChunkSize
}
