package transfer

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestDefaultTransferConfig(t *testing.T) {
	config := DefaultTransferConfig()

	require.NotNil(t, config, "DefaultTransferConfig() returned nil")

	// Test that the config is valid
	err := config.Validate()
	require.NoError(t, err, "Default config should be valid")

	// Test chunk size settings
	require.Equal(t, int32(DefaultChunkSize), config.ChunkSize, "Expected ChunkSize to match DefaultChunkSize")
	require.Equal(t, int32(MinChunkSize), config.MinChunkSize, "Expected MinChunkSize to match MinChunkSize")
	require.Equal(t, int32(MaxChunkSize), config.MaxChunkSize, "Expected MaxChunkSize to match MaxChunkSize")

	// Test other settings
	require.Positive(t, config.MaxConcurrentTransfers, "MaxConcurrentTransfers should be positive")
	require.NotNil(t, config.DefaultRetryPolicy, "DefaultRetryPolicy should not be nil")
}

func TestTransferConfig_Validate(t *testing.T) {
	tests := []struct {
		name        string
		config      *TransferConfig
		expectError bool
		errorMsg    string
	}{
		{
			name:        "valid config",
			config:      DefaultTransferConfig(),
			expectError: false,
		},
		{
			name: "invalid chunk size - zero",
			config: &TransferConfig{
				ChunkSize:              0,
				MinChunkSize:           MinChunkSize,
				MaxChunkSize:           MaxChunkSize,
				MaxConcurrentTransfers: 10,
				MaxConcurrentChunks:    10,
				BufferSize:             1024,
				DefaultRetryPolicy:     DefaultRetryPolicy(),
				EventBufferSize:        10,
			},
			expectError: true,
			errorMsg:    "chunk_size must be positive",
		},
		{
			name: "invalid chunk size - too small",
			config: &TransferConfig{
				ChunkSize:              1024,
				MinChunkSize:           2048,
				MaxChunkSize:           MaxChunkSize,
				MaxConcurrentTransfers: 10,
				MaxConcurrentChunks:    10,
				BufferSize:             1024,
				DefaultRetryPolicy:     DefaultRetryPolicy(),
				EventBufferSize:        10,
			},
			expectError: true,
			errorMsg:    "chunk_size cannot be less than min_chunk_size",
		},
		{
			name: "invalid chunk size - too large",
			config: &TransferConfig{
				ChunkSize:              MaxChunkSize + 1,
				MinChunkSize:           MinChunkSize,
				MaxChunkSize:           MaxChunkSize,
				MaxConcurrentTransfers: 10,
				MaxConcurrentChunks:    10,
				BufferSize:             1024,
				DefaultRetryPolicy:     DefaultRetryPolicy(),
				EventBufferSize:        10,
			},
			expectError: true,
			errorMsg:    "chunk_size cannot be greater than max_chunk_size",
		},
		{
			name: "invalid min/max chunk size",
			config: &TransferConfig{
				ChunkSize:              MinChunkSize, // Use valid chunk size
				MinChunkSize:           MaxChunkSize,
				MaxChunkSize:           MinChunkSize,
				MaxConcurrentTransfers: 10,
				MaxConcurrentChunks:    10,
				BufferSize:             1024,
				DefaultRetryPolicy:     DefaultRetryPolicy(),
				EventBufferSize:        10,
			},
			expectError: true,
			errorMsg:    "chunk_size cannot be less than min_chunk_size",
		},
		{
			name: "invalid max concurrent transfers",
			config: &TransferConfig{
				ChunkSize:              DefaultChunkSize,
				MinChunkSize:           MinChunkSize,
				MaxChunkSize:           MaxChunkSize,
				MaxConcurrentTransfers: 0,
				MaxConcurrentChunks:    10,
				BufferSize:             1024,
				DefaultRetryPolicy:     DefaultRetryPolicy(),
				EventBufferSize:        10,
			},
			expectError: true,
			errorMsg:    "max_concurrent_transfers must be positive",
		},
		{
			name: "nil retry policy",
			config: &TransferConfig{
				ChunkSize:              DefaultChunkSize,
				MinChunkSize:           MinChunkSize,
				MaxChunkSize:           MaxChunkSize,
				MaxConcurrentTransfers: 10,
				MaxConcurrentChunks:    10,
				BufferSize:             1024,
				DefaultRetryPolicy:     nil,
				EventBufferSize:        10,
			},
			expectError: true,
			errorMsg:    "default_retry_policy cannot be nil",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := test.config.Validate()
			if test.expectError {
				if err == nil {
					t.Error("Expected validation error, but got nil")
				} else if test.errorMsg != "" && err.Error() != test.errorMsg {
					t.Errorf("Expected error message %q, got %q", test.errorMsg, err.Error())
				}
			} else {
				if err != nil {
					t.Errorf("Expected no validation error, but got: %v", err)
				}
			}
		})
	}
}

func TestTransferConfig_GetChunkSizeForFile(t *testing.T) {
	config := DefaultTransferConfig()

	tests := []struct {
		name     string
		fileSize int64
		expected int32
	}{
		{
			name:     "zero byte file",
			fileSize: 0,
			expected: 0, // Should return 0 for empty files
		},
		{
			name:     "very small file",
			fileSize: 1024,
			expected: 1024,
		},
		{
			name:     "small file",
			fileSize: int64(config.MinChunkSize / 2),
			expected: config.MinChunkSize / 2, // Should return actual file size
		},
		{
			name:     "file smaller than MinChunkSize",
			fileSize: int64(config.MinChunkSize - 1),
			expected: config.MinChunkSize - 1, // Should return actual file size
		},
		{
			name:     "file equal to MinChunkSize",
			fileSize: int64(config.MinChunkSize),
			expected: config.MinChunkSize,
		},
		{
			name:     "normal file",
			fileSize: 10 * 1024 * 1024, // 10MB
			expected: config.ChunkSize,
		},
		{
			name:     "large file",
			fileSize: 200 * 1024 * 1024,    // 200MB
			expected: config.ChunkSize * 2, // Should use larger chunks
		},
		{
			name:     "very large file with max chunk limit",
			fileSize: 1024 * 1024 * 1024,   // 1GB
			expected: config.ChunkSize * 2, // Should use 2x default, which is within max
		},
		{
			name:     "extremely large file (default config doesn't exceed max)",
			fileSize: 10 * 1024 * 1024 * 1024, // 10GB
			expected: config.ChunkSize * 2,    // 128KB, which is still within MaxChunkSize (256KB)
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result := config.GetChunkSizeForFile(test.fileSize)
			if result != test.expected {
				t.Errorf("GetChunkSizeForFile(%d) = %d, want %d", test.fileSize, result, test.expected)
			}
		})
	}
}

func TestTransferConfig_GetChunkSizeForFile_MaxChunkSizeLimit(t *testing.T) {
	// Create a custom config where ChunkSize * 2 would exceed MaxChunkSize
	config := &TransferConfig{
		ChunkSize:    200 * 1024, // 200KB
		MaxChunkSize: 300 * 1024, // 300KB
		MinChunkSize: 4 * 1024,   // 4KB
	}

	tests := []struct {
		name     string
		fileSize int64
		expected int32
	}{
		{
			name:     "large file within scaling limit",
			fileSize: 200 * 1024 * 1024, // 200MB
			expected: config.ChunkSize * 2, // 400KB, but should be capped at MaxChunkSize
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result := config.GetChunkSizeForFile(test.fileSize)
			// Since ChunkSize * 2 = 400KB > MaxChunkSize (300KB), should return MaxChunkSize
			expectedResult := config.MaxChunkSize
			if result != expectedResult {
				t.Errorf("GetChunkSizeForFile(%d) = %d, want %d (should be capped at MaxChunkSize)", 
					test.fileSize, result, expectedResult)
			}
		})
	}
}

func TestTransferConfig_IsValidChunkSize(t *testing.T) {
	config := DefaultTransferConfig()

	tests := []struct {
		name      string
		chunkSize int32
		expected  bool
	}{
		{
			name:      "valid chunk size",
			chunkSize: config.ChunkSize,
			expected:  true,
		},
		{
			name:      "minimum valid chunk size",
			chunkSize: config.MinChunkSize,
			expected:  true,
		},
		{
			name:      "maximum valid chunk size",
			chunkSize: config.MaxChunkSize,
			expected:  true,
		},
		{
			name:      "too small chunk size",
			chunkSize: config.MinChunkSize - 1,
			expected:  false,
		},
		{
			name:      "too large chunk size",
			chunkSize: config.MaxChunkSize + 1,
			expected:  false,
		},
		{
			name:      "zero chunk size",
			chunkSize: 0,
			expected:  false,
		},
		{
			name:      "negative chunk size",
			chunkSize: -1,
			expected:  false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result := config.IsValidChunkSize(test.chunkSize)
			if result != test.expected {
				t.Errorf("IsValidChunkSize(%d) = %v, want %v", test.chunkSize, result, test.expected)
			}
		})
	}
}

func TestChunkSizeConstants(t *testing.T) {
	// Test that constants are reasonable
	if DefaultChunkSize <= 0 {
		t.Error("DefaultChunkSize should be positive")
	}

	if MinChunkSize <= 0 {
		t.Error("MinChunkSize should be positive")
	}

	if MaxChunkSize <= 0 {
		t.Error("MaxChunkSize should be positive")
	}

	if MinChunkSize >= MaxChunkSize {
		t.Error("MinChunkSize should be less than MaxChunkSize")
	}

	if DefaultChunkSize < MinChunkSize || DefaultChunkSize > MaxChunkSize {
		t.Error("DefaultChunkSize should be between MinChunkSize and MaxChunkSize")
	}

	// Test specific expected values
	if DefaultChunkSize != 64*1024 {
		t.Errorf("Expected DefaultChunkSize to be 64KB, got %d", DefaultChunkSize)
	}

	if MinChunkSize != 4*1024 {
		t.Errorf("Expected MinChunkSize to be 4KB, got %d", MinChunkSize)
	}

	if MaxChunkSize != 256*1024 {
		t.Errorf("Expected MaxChunkSize to be 256KB, got %d", MaxChunkSize)
	}
}
