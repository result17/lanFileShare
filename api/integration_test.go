package api

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/rescp17/lanFileSharer/pkg/crypto"
)

// TestAskPayloadWithSignedFiles tests that AskPayload works with SignedFileStructure
func TestAskPayloadWithSignedFiles(t *testing.T) {
	// Create temporary test file
	tempDir := t.TempDir()
	testFile := filepath.Join(tempDir, "test.txt")
	err := os.WriteFile(testFile, []byte("test content"), 0644)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Create signed file structure using CreateSignedFileStructure
	signedFiles, err := crypto.CreateSignedFileStructure([]string{testFile})
	if err != nil {
		t.Fatalf("Failed to create signed file structure: %v", err)
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
