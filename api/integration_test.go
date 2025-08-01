package api

import (
	"testing"

	"github.com/rescp17/lanFileSharer/pkg/crypto"
	"github.com/rescp17/lanFileSharer/pkg/fileInfo"
)

// TestAskPayloadWithSignedFiles tests that AskPayload works with SignedFileStructure
func TestAskPayloadWithSignedFiles(t *testing.T) {
	// Create test files
	testFiles := []fileInfo.FileNode{
		{
			Name:     "test.txt",
			IsDir:    false,
			Size:     100,
			Checksum: "abc123",
		},
	}

	// Create signed file structure
	signer, err := crypto.NewFileStructureSigner()
	if err != nil {
		t.Fatalf("Failed to create signer: %v", err)
	}

	signedFiles, err := signer.SignFileStructure(testFiles)
	if err != nil {
		t.Fatalf("Failed to sign file structure: %v", err)
	}

	// Create AskPayload with signed files
	payload := AskPayload{
		SignedFiles: signedFiles,
		// Offer would be set in real usage
	}

	// Verify the payload contains the signed files
	if payload.SignedFiles == nil {
		t.Error("SignedFiles should not be nil")
	}

	if len(payload.SignedFiles.Files) != 1 {
		t.Errorf("Expected 1 file, got %d", len(payload.SignedFiles.Files))
	}

	if payload.SignedFiles.Files[0].Name != "test.txt" {
		t.Errorf("Expected file name 'test.txt', got '%s'", payload.SignedFiles.Files[0].Name)
	}

	// Verify signature can be validated
	err = crypto.VerifyFileStructure(payload.SignedFiles)
	if err != nil {
		t.Errorf("Failed to verify file structure: %v", err)
	}
}
