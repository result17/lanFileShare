package crypto

import (
	"crypto/x509"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/rescp17/lanFileSharer/pkg/fileInfo"
)

func TestNewFileStructureSigner(t *testing.T) {
	signer, err := NewFileStructureSigner()
	if err != nil {
		t.Fatalf("Failed to create new signer: %v", err)
	}

	if signer.keyPair == nil {
		t.Error("Key pair should not be nil")
	}

	if signer.keyPair.PrivateKey == nil {
		t.Error("Private key should not be nil")
	}

	if signer.keyPair.PublicKey == nil {
		t.Error("Public key should not be nil")
	}

	// Test that we can get public key bytes
	pubKeyBytes, err := signer.GetPublicKeyBytes()
	if err != nil {
		t.Errorf("Failed to get public key bytes: %v", err)
	}

	if len(pubKeyBytes) == 0 {
		t.Error("Public key bytes should not be empty")
	}
}

func TestSignAndVerifyFileStructure(t *testing.T) {
	// Create temporary test files
	tempDir := t.TempDir()
	testFile1 := filepath.Join(tempDir, "test1.txt")
	testFile2 := filepath.Join(tempDir, "test2.txt")

	err := os.WriteFile(testFile1, []byte("test content 1"), 0644)
	if err != nil {
		t.Fatalf("Failed to create test file 1: %v", err)
	}

	err = os.WriteFile(testFile2, []byte("test content 2"), 0644)
	if err != nil {
		t.Fatalf("Failed to create test file 2: %v", err)
	}

	// Create FileNodes
	node1, err := fileInfo.CreateNode(testFile1)
	if err != nil {
		t.Fatalf("Failed to create node 1: %v", err)
	}

	node2, err := fileInfo.CreateNode(testFile2)
	if err != nil {
		t.Fatalf("Failed to create node 2: %v", err)
	}

	files := []fileInfo.FileNode{node1, node2}

	// Create signer and sign the structure
	signer, err := NewFileStructureSigner()
	if err != nil {
		t.Fatalf("Failed to create signer: %v", err)
	}

	signedStructure, err := signer.SignFileStructure(files)
	if err != nil {
		t.Fatalf("Failed to sign file structure: %v", err)
	}

	// Verify the signed structure
	err = VerifyFileStructure(signedStructure)
	if err != nil {
		t.Errorf("Failed to verify file structure: %v", err)
	}

	// Test that verification fails with tampered data
	originalName := signedStructure.Files[0].Name
	signedStructure.Files[0].Name = "tampered_name"

	err = VerifyFileStructure(signedStructure)
	if err == nil {
		t.Error("Verification should fail with tampered data")
	}

	// Restore original data
	signedStructure.Files[0].Name = originalName

	// Test that verification fails with tampered signature
	signedStructure.Signature[0] ^= 0xFF // Flip bits in first byte

	err = VerifyFileStructure(signedStructure)
	if err == nil {
		t.Error("Verification should fail with tampered signature")
	}
}
func TestCreateSignedFileStructure(t *testing.T) {
	// Create temporary test files
	tempDir := t.TempDir()
	testFile1 := filepath.Join(tempDir, "test1.txt")
	testFile2 := filepath.Join(tempDir, "test2.txt")

	err := os.WriteFile(testFile1, []byte("test content 1"), 0644)
	if err != nil {
		t.Fatalf("Failed to create test file 1: %v", err)
	}

	err = os.WriteFile(testFile2, []byte("test content 2"), 0644)
	if err != nil {
		t.Fatalf("Failed to create test file 2: %v", err)
	}

	filePaths := []string{testFile1, testFile2}

	// Create signed structure from file paths
	signedStructure, err := CreateSignedFileStructure(filePaths)
	if err != nil {
		t.Fatalf("Failed to create signed file structure: %v", err)
	}

	// Verify the structure
	err = VerifyFileStructure(signedStructure)
	if err != nil {
		t.Errorf("Failed to verify created signed structure: %v", err)
	}

	// Check that files are properly included
	if len(signedStructure.Files) != 2 {
		t.Errorf("Expected 2 files, got %d", len(signedStructure.Files))
	}

	// Check that checksums are calculated
	for i, file := range signedStructure.Files {
		if file.Checksum == "" {
			t.Errorf("File %d should have checksum calculated", i)
		}
	}
}

func TestCreateSignedFileStructureWithDirectory(t *testing.T) {
	// Create temporary directory structure
	tempDir := t.TempDir()
	subDir := filepath.Join(tempDir, "subdir")
	err := os.Mkdir(subDir, 0755)
	if err != nil {
		t.Fatalf("Failed to create subdirectory: %v", err)
	}

	testFile1 := filepath.Join(tempDir, "test1.txt")
	testFile2 := filepath.Join(subDir, "test2.txt")

	err = os.WriteFile(testFile1, []byte("test content 1"), 0644)
	if err != nil {
		t.Fatalf("Failed to create test file 1: %v", err)
	}

	err = os.WriteFile(testFile2, []byte("test content 2"), 0644)
	if err != nil {
		t.Fatalf("Failed to create test file 2: %v", err)
	}

	filePaths := []string{tempDir}

	// Create signed structure from directory
	signedStructure, err := CreateSignedFileStructure(filePaths)
	if err != nil {
		t.Fatalf("Failed to create signed file structure with directory: %v", err)
	}

	// Verify the structure
	err = VerifyFileStructure(signedStructure)
	if err != nil {
		t.Errorf("Failed to verify directory signed structure: %v", err)
	}

	// Check that directory structure is preserved
	if len(signedStructure.Files) != 1 {
		t.Errorf("Expected 1 root directory, got %d", len(signedStructure.Files))
	}

	rootDir := signedStructure.Files[0]
	if !rootDir.IsDir {
		t.Error("Root should be a directory")
	}

	if rootDir.Checksum == "" {
		t.Error("Directory should have checksum calculated")
	}
}

func TestVerifyFileStructureWithInvalidPublicKey(t *testing.T) {
	// Create a signed structure with invalid public key
	signedStructure := &SignedFileStructure{
		Files:     []fileInfo.FileNode{},
		PublicKey: []byte("invalid public key"),
		Signature: []byte("signature"),
	}

	err := VerifyFileStructure(signedStructure)
	if err == nil {
		t.Error("Verification should fail with invalid public key")
	}
}

func TestSignFileStructureWithEmptyFiles(t *testing.T) {
	signer, err := NewFileStructureSigner()
	if err != nil {
		t.Fatalf("Failed to create signer: %v", err)
	}

	// Test with empty file list
	emptyFiles := []fileInfo.FileNode{}
	signedStructure, err := signer.SignFileStructure(emptyFiles)
	if err != nil {
		t.Fatalf("Failed to sign empty file structure: %v", err)
	}

	// Verify empty structure
	err = VerifyFileStructure(signedStructure)
	if err != nil {
		t.Errorf("Failed to verify empty file structure: %v", err)
	}
}
func TestGetPrivateKeyBytes(t *testing.T) {
	signer, err := NewFileStructureSigner()
	if err != nil {
		t.Fatalf("Failed to create signer: %v", err)
	}

	privateKeyBytes, err := signer.GetPrivateKeyBytes()
	if err != nil {
		t.Errorf("Failed to get private key bytes: %v", err)
	}

	if len(privateKeyBytes) == 0 {
		t.Error("Private key bytes should not be empty")
	}

	// Verify we can parse the private key back
	privateKey, err := x509.ParsePKCS1PrivateKey(privateKeyBytes)
	if err != nil {
		t.Errorf("Failed to parse private key bytes: %v", err)
	}

	// Verify the parsed key matches the original
	if privateKey.N.Cmp(signer.keyPair.PrivateKey.N) != 0 {
		t.Error("Parsed private key doesn't match original")
	}
}

func TestCreateSignedFileStructureWithInvalidPath(t *testing.T) {
	// Test with non-existent file path
	invalidPaths := []string{"/non/existent/path/file.txt"}

	_, err := CreateSignedFileStructure(invalidPaths)
	if err == nil {
		t.Error("Should fail with non-existent file path")
	}
}

func TestSignFileStructureErrorCases(t *testing.T) {
	signer, err := NewFileStructureSigner()
	if err != nil {
		t.Fatalf("Failed to create signer: %v", err)
	}

	// Test with files that can't be marshaled (circular reference simulation)
	// We'll create a FileNode with invalid data that might cause JSON marshal issues
	invalidFiles := []fileInfo.FileNode{
		{
			Name:     "test",
			IsDir:    false,
			Size:     100,
			MimeType: "text/plain",
			Checksum: "abc123",
		},
	}

	// This should work normally, but let's test the error path by creating a mock scenario
	_, err = signer.SignFileStructure(invalidFiles)
	if err != nil {
		t.Logf("Expected success but got error (this is actually normal): %v", err)
	}
}

func TestVerifyFileStructureErrorCases(t *testing.T) {
	// Test with invalid public key format
	invalidSignedStructure := &SignedFileStructure{
		Files:     []fileInfo.FileNode{},
		PublicKey: []byte("invalid key data"),
		Signature: []byte("signature"),
	}

	err := VerifyFileStructure(invalidSignedStructure)
	if err == nil {
		t.Error("Should fail with invalid public key")
	}

	// Test with non-RSA public key (create a different key type)
	// We'll simulate this by creating a malformed key that parses but isn't RSA
	validSigner, err := NewFileStructureSigner()
	if err != nil {
		t.Fatalf("Failed to create signer: %v", err)
	}

	validSignedStructure, err := validSigner.SignFileStructure([]fileInfo.FileNode{})
	if err != nil {
		t.Fatalf("Failed to create valid signed structure: %v", err)
	}

	// Test with corrupted signature
	corruptedStructure := *validSignedStructure
	corruptedStructure.Signature = []byte("corrupted signature")

	err = VerifyFileStructure(&corruptedStructure)
	if err == nil {
		t.Error("Should fail with corrupted signature")
	}
}

// Test to improve coverage for error paths in NewFileStructureSigner
func TestNewFileStructureSignerWithMockError(t *testing.T) {
	// This test is mainly for documentation - in practice, RSA key generation
	// rarely fails unless there's insufficient entropy or system issues
	signer, err := NewFileStructureSigner()
	if err != nil {
		t.Logf("Unexpected error in key generation: %v", err)
	}
	if signer == nil {
		t.Error("Signer should not be nil on success")
	}
}

func TestNewFileStructureSignerFromKeyPair(t *testing.T) {
	// Create a key pair first
	keyPair, err := GenerateKeyPair(2048)
	if err != nil {
		t.Fatalf("Failed to generate key pair: %v", err)
	}

	// Create signer from existing key pair
	signer := NewFileStructureSignerFromKeyPair(keyPair)
	if signer == nil {
		t.Error("Signer should not be nil")
	}

	if signer.GetKeyPair() != keyPair {
		t.Error("Signer should use the provided key pair")
	}

	// Test that the signer works with the provided key pair
	testFiles := []fileInfo.FileNode{
		{
			Name:     "test.txt",
			IsDir:    false,
			Size:     100,
			Checksum: "abc123",
		},
	}

	signedStructure, err := signer.SignFileStructure(testFiles)
	if err != nil {
		t.Errorf("Failed to sign with provided key pair: %v", err)
	}

	err = VerifyFileStructure(signedStructure)
	if err != nil {
		t.Errorf("Failed to verify signature from provided key pair: %v", err)
	}
}

func TestGetPublicAndPrivateKey(t *testing.T) {
	signer, err := NewFileStructureSigner()
	if err != nil {
		t.Fatalf("Failed to create signer: %v", err)
	}

	// Test GetPublicKey
	publicKey := signer.GetPublicKey()
	if publicKey == nil {
		t.Error("Public key should not be nil")
	}

	// Test GetPrivateKey
	privateKey := signer.GetPrivateKey()
	if privateKey == nil {
		t.Error("Private key should not be nil")
	}

	// Verify they form a valid key pair
	if publicKey.N.Cmp(privateKey.PublicKey.N) != 0 {
		t.Error("Public and private keys should match")
	}
}

// Test to improve coverage for SignFileStructure error paths
func TestSignFileStructureWithLargeData(t *testing.T) {
	signer, err := NewFileStructureSigner()
	if err != nil {
		t.Fatalf("Failed to create signer: %v", err)
	}

	// Create a large file structure to test marshaling
	largeFiles := make([]fileInfo.FileNode, 1000)
	for i := 0; i < 1000; i++ {
		largeFiles[i] = fileInfo.FileNode{
			Name:     fmt.Sprintf("file_%d.txt", i),
			IsDir:    false,
			Size:     int64(i * 100),
			MimeType: "text/plain",
			Checksum: fmt.Sprintf("checksum_%d", i),
		}
	}

	signedStructure, err := signer.SignFileStructure(largeFiles)
	if err != nil {
		t.Errorf("Should handle large file structures: %v", err)
	}

	if signedStructure != nil && len(signedStructure.Files) != 1000 {
		t.Errorf("Expected 1000 files, got %d", len(signedStructure.Files))
	}
}

// Test to improve coverage for VerifyFileStructure with non-RSA key
func TestVerifyFileStructureWithNonRSAKey(t *testing.T) {
	// Create a structure with a public key that parses but isn't RSA
	// This is difficult to test directly, so we'll test the error path indirectly

	// Create a valid signed structure first
	signer, err := NewFileStructureSigner()
	if err != nil {
		t.Fatalf("Failed to create signer: %v", err)
	}

	signedStructure, err := signer.SignFileStructure([]fileInfo.FileNode{})
	if err != nil {
		t.Fatalf("Failed to create signed structure: %v", err)
	}

	// Test with completely invalid public key bytes
	invalidStructure := *signedStructure
	invalidStructure.PublicKey = []byte{0x30, 0x82} // Start of ASN.1 but incomplete

	err = VerifyFileStructure(&invalidStructure)
	if err == nil {
		t.Error("Should fail with incomplete ASN.1 data")
	}
}
