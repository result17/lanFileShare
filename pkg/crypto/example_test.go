package crypto

import (
	"fmt"
	"os"
	"path/filepath"
	"log/slog"

	"github.com/rescp17/lanFileSharer/pkg/fileInfo"
)

// Example demonstrates how to use the digital signature infrastructure
func ExampleCreateSignedFileStructure() {
	// Create a temporary directory with test files
	tempDir, err := os.MkdirTemp("", "signature_example")
	if err != nil {
		slog.Warn("Error creating temp dir", "error", err)
		return
	}
	defer func() {
		if err := os.RemoveAll(tempDir); err != nil {
			slog.Warn("Error removing temp dir:", "error", err)
		}
	} ()

	// Create test files
	testFile1 := filepath.Join(tempDir, "document.txt")
	testFile2 := filepath.Join(tempDir, "image.jpg")

	err = os.WriteFile(testFile1, []byte("This is a test document"), 0644)
	if err != nil {
		fmt.Printf("Error creating test file: %v\n", err)
		return
	}

	err = os.WriteFile(testFile2, []byte("fake image data"), 0644)
	if err != nil {
		fmt.Printf("Error creating test file: %v\n", err)
		return
	}

	// Create signed file structure
	filePaths := []string{testFile1, testFile2}
	signedStructure, err := CreateSignedFileStructure(filePaths)
	if err != nil {
		fmt.Printf("Error creating signed structure: %v\n", err)
		return
	}

	// Verify the signature
	err = VerifyFileStructure(signedStructure)
	if err != nil {
		fmt.Printf("Signature verification failed: %v\n", err)
		return
	}

	fmt.Printf("Successfully created and verified signed file structure with %d files\n", len(signedStructure.Files))
	// Output: Successfully created and verified signed file structure with 2 files
}

// Example demonstrates manual key generation and signing
func ExampleFileStructureSigner() {
	// Create a signer with generated keys
	signer, err := NewFileStructureSigner()
	if err != nil {
		fmt.Printf("Error creating signer: %v\n", err)
		return
	}

	// Get public key for sharing
	publicKeyBytes, err := signer.GetPublicKeyBytes()
	if err != nil {
		fmt.Printf("Error getting public key: %v\n", err)
		return
	}

	fmt.Printf("Generated public key of %d bytes\n", len(publicKeyBytes))

	// Create empty file structure for demonstration
	emptyFiles := []fileInfo.FileNode{}
	signedStructure, err := signer.SignFileStructure(emptyFiles)
	if err != nil {
		fmt.Printf("Error signing structure: %v\n", err)
		return
	}

	// Verify the signature
	err = VerifyFileStructure(signedStructure)
	if err != nil {
		fmt.Printf("Verification failed: %v\n", err)
		return
	}

	fmt.Println("Successfully signed and verified empty file structure")
	// Output: Generated public key of 294 bytes
	// Successfully signed and verified empty file structure
}
