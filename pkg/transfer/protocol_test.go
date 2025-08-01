package transfer

import (
	"testing"
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
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}

	// Test deserialization
	deserializedMsg, err := serializer.Unmarshal(data)
	if err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}

	// Verify data consistency
	if deserializedMsg.Type != originalMsg.Type {
		t.Errorf("Type mismatch: expected %s, got %s", originalMsg.Type, deserializedMsg.Type)
	}

	if deserializedMsg.FileID != originalMsg.FileID {
		t.Errorf("FileID mismatch: expected %s, got %s", originalMsg.FileID, deserializedMsg.FileID)
	}

	if string(deserializedMsg.Data) != string(originalMsg.Data) {
		t.Errorf("Data mismatch: expected %s, got %s", string(originalMsg.Data), string(deserializedMsg.Data))
	}

	// Test serializer properties
	if serializer.Name() != "json" {
		t.Errorf("Name should be 'json', got %s", serializer.Name())
	}

	if serializer.IsBinary() {
		t.Error("JSON serializer should not be binary")
	}
}
