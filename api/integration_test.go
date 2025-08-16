package api

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/pion/webrtc/v4"
	"github.com/rescp17/lanFileSharer/pkg/crypto"
	"github.com/stretchr/testify/require"
)

// TestAskPayloadWithSignedFiles tests that AskPayload works with SignedFileStructure
func TestAskPayloadWithSignedFiles(t *testing.T) {
	// Create temporary test file
	tempDir := t.TempDir()
	testFile := filepath.Join(tempDir, "test.txt")
	err := os.WriteFile(testFile, []byte("test content"), 0644)
	require.NoError(t, err, "Failed to create test file")

	// Create signed file structure using CreateSignedFileStructure
	signedFiles, err := crypto.CreateSignedFileStructure([]string{testFile})
	require.NoError(t, err, "Failed to create signed file structure")

	// Create AskPayload with signed files
	payload := AskPayload{
		SignedFiles: signedFiles,
		// Offer would be set in real usage
	}

	// Verify the payload contains the signed files
	require.NotNil(t, payload.SignedFiles, "SignedFiles should not be nil")
	require.Len(t, payload.SignedFiles.Files, 1, "Expected 1 file")
	require.Equal(t, "test.txt", payload.SignedFiles.Files[0].Name, "Expected file name 'test.txt'")

	// Verify signature can be validated
	err = crypto.VerifyFileStructure(payload.SignedFiles)
	require.NoError(t, err, "Failed to verify file structure")
}

func TestAskPayload_Serialization(t *testing.T) {
	// Setup: Create a temporary file
	tempDir := t.TempDir()
	testFile := filepath.Join(tempDir, "test.txt")
	err := os.WriteFile(testFile, []byte("test content"), 0644)
	require.NoError(t, err, "Failed to create test file")

	// Create the signed file structure
	signedFiles, err := crypto.CreateSignedFileStructure([]string{testFile})
	require.NoError(t, err, "Failed to create signed file structure")

	// Create the payload
	payload := AskPayload{
		SignedFiles: signedFiles,
		Offer: webrtc.SessionDescription{
			Type: webrtc.SDPTypeOffer,
			SDP:  "v=0\r\no=- 0 0 IN IP4 127.0.0.1\r\ns=-\r\nt=0 0\r\n",
		},
	}

	// Marshal the payload to JSON (simulating sending)
	jsonData, err := json.Marshal(payload)
	require.NoError(t, err, "Failed to marshal payload")

	// Unmarshal the JSON back into a new payload (simulating receiving)
	var receivedPayload AskPayload
	err = json.Unmarshal(jsonData, &receivedPayload)
	require.NoError(t, err, "Failed to unmarshal payload")

	// Verify the received payload's contents
	require.NotNil(t, receivedPayload.SignedFiles, "Received SignedFiles should not be nil")
	require.Len(t, receivedPayload.SignedFiles.Files, 1, "Expected 1 file")
	require.Equal(t, "test.txt", receivedPayload.SignedFiles.Files[0].Name, "Expected file name 'test.txt'")

	// Verify the signature on the received data
	err = crypto.VerifyFileStructure(receivedPayload.SignedFiles)
	require.NoError(t, err, "Failed to verify file structure on received payload")
}
