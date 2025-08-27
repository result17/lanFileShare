package transfer

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestJSONSerializer(t *testing.T) {
	serializer := NewJSONSerializer()

	// Create test message
	session := TransferSession{
		ServiceID:       "test-service",
		SessionID:       "test-session",
		SessionCreateAt: 1704067200, // Unix timestamp for 2024-01-01T00:00:00Z
	}

	originalMsg := &ChunkMessage{
		Type:         ChunkData,
		Session:      session,
		FileID:       "file123",
		FileName:     "test.txt",
		SequenceNo:   1,
		Data:         []byte("hello world"),
		ChunkHash:    "abc123",
		TotalSize:    11,
		ExpectedHash: "def456",
	}

	// Test serialization
	data, err := serializer.Marshal(originalMsg)
	require.NoError(t, err, "Marshal failed")

	// Test deserialization
	deserializedMsg, err := serializer.Unmarshal(data)
	require.NoError(t, err, "Unmarshal failed")

	// Verify data consistency
	assert.Equal(t, originalMsg.Type, deserializedMsg.Type, "Type mismatch")
	assert.Equal(t, originalMsg.FileID, deserializedMsg.FileID, "FileID mismatch")
	assert.Equal(t, string(originalMsg.Data), string(deserializedMsg.Data), "Data mismatch")

	// Test serializer properties
	assert.Equal(t, "json", serializer.Name(), "Name should be 'json'")
	assert.False(t, serializer.IsBinary(), "JSON serializer should not be binary")
}
