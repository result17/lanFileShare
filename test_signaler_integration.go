// This is a temporary test file to verify our changes work
package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/rescp17/lanFileSharer/pkg/crypto"
	"github.com/rescp17/lanFileSharer/pkg/transfer"
)

func main() {
	// Test 1: Create FileStructureManager and sign it
	fmt.Println("Testing SignFileStructure with FileStructureManager...")

	// Create temporary test file
	tempDir, err := os.MkdirTemp("", "test_signaler")
	if err != nil {
		fmt.Printf("Failed to create temp dir: %v\n", err)
		return
	}
	defer os.RemoveAll(tempDir)

	testFile := filepath.Join(tempDir, "test.txt")
	err = os.WriteFile(testFile, []byte("test content"), 0644)
	if err != nil {
		fmt.Printf("Failed to create test file: %v\n", err)
		return
	}

	// Create FileStructureManager
	fsm := transfer.NewFileStructureManager()
	err = fsm.AddPath(testFile)
	if err != nil {
		fmt.Printf("Failed to add path: %v\n", err)
		return
	}

	// Create signer and sign
	signer, err := crypto.NewFileStructureSigner()
	if err != nil {
		fmt.Printf("Failed to create signer: %v\n", err)
		return
	}

	signedStructure, err := signer.SignFileStructureManager(fsm)
	if err != nil {
		fmt.Printf("Failed to sign file structure: %v\n", err)
		return
	}

	// Verify signature
	err = crypto.VerifyFileStructure(signedStructure)
	if err != nil {
		fmt.Printf("Failed to verify signature: %v\n", err)
		return
	}

	fmt.Printf("✓ SignFileStructure test passed!\n")
	fmt.Printf("  - Files: %d\n", len(signedStructure.Files))
	fmt.Printf("  - Directories: %d\n", len(signedStructure.Directories))
	fmt.Printf("  - Signature length: %d bytes\n", len(signedStructure.Signature))

	// Test 2: CreateSignedFileStructure function
	fmt.Println("\nTesting CreateSignedFileStructure...")

	signedStructure2, err := crypto.CreateSignedFileStructure([]string{testFile})
	if err != nil {
		fmt.Printf("Failed to create signed file structure: %v\n", err)
		return
	}

	err = crypto.VerifyFileStructure(signedStructure2)
	if err != nil {
		fmt.Printf("Failed to verify created structure: %v\n", err)
		return
	}

	fmt.Printf("✓ CreateSignedFileStructure test passed!\n")
	fmt.Printf("  - Files: %d\n", len(signedStructure2.Files))

	fmt.Println("\n🎉 All tests passed! The SignFileStructure implementation is working correctly.")
}
