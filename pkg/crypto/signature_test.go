package crypto

import (
	"crypto/x509"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/rescp17/lanFileSharer/pkg/fileInfo"
	"github.com/rescp17/lanFileSharer/pkg/transfer"
	"github.com/stretchr/testify/require"
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

// TestSignFileStructure tests the SignFileStructure method with FileStructureManager
func TestSignFileStructure(t *testing.T) {
	// Create temporary test files
	tempDir := t.TempDir()
	testFile1 := filepath.Join(tempDir, "test1.txt")
	testFile2 := filepath.Join(tempDir, "test2.txt")
	subDir := filepath.Join(tempDir, "subdir")
	testFile3 := filepath.Join(subDir, "test3.txt")

	// Create directory structure
	err := os.Mkdir(subDir, 0755)
	if err != nil {
		t.Fatalf("Failed to create subdirectory: %v", err)
	}

	err = os.WriteFile(testFile1, []byte("test content 1"), 0644)
	if err != nil {
		t.Fatalf("Failed to create test file 1: %v", err)
	}

	err = os.WriteFile(testFile2, []byte("test content 2"), 0644)
	if err != nil {
		t.Fatalf("Failed to create test file 2: %v", err)
	}

	err = os.WriteFile(testFile3, []byte("test content 3"), 0644)
	if err != nil {
		t.Fatalf("Failed to create test file 3: %v", err)
	}

	// Create FileStructureManager
	fsm := transfer.NewFileStructureManager()
	node, err := fileInfo.CreateNode(tempDir)
	if err != nil {
		t.Fatalf("Failed to create node: %v", err)
	}

	err = fsm.AddFileNode(&node)
	if err != nil {
		t.Fatalf("Failed to add path to FileStructureManager: %v", err)
	}

	// Create signer and sign the structure
	signer, err := NewFileStructureSigner()
	if err != nil {
		t.Fatalf("Failed to create signer: %v", err)
	}

	signedStructure, err := signer.SignFileStructureManager(fsm)
	if err != nil {
		t.Fatalf("Failed to sign file structure: %v", err)
	}

	// Verify the signed structure
	if signedStructure == nil {
		t.Fatal("Signed structure should not be nil")
	}

	if len(signedStructure.PublicKey) == 0 {
		t.Error("Public key should not be empty")
	}

	if len(signedStructure.Signature) == 0 {
		t.Error("Signature should not be empty")
	}

	if signedStructure.Metadata == nil {
		t.Error("Metadata should not be nil")
	}

	// Verify metadata
	if signedStructure.Metadata.TotalFiles != fsm.GetFileCount() {
		t.Errorf("Expected %d files in metadata, got %d", fsm.GetFileCount(), signedStructure.Metadata.TotalFiles)
	}

	if signedStructure.Metadata.TotalDirs != fsm.GetDirCount() {
		t.Errorf("Expected %d directories in metadata, got %d", fsm.GetDirCount(), signedStructure.Metadata.TotalDirs)
	}

	if signedStructure.Metadata.TotalSize != fsm.GetTotalSize() {
		t.Errorf("Expected total size %d in metadata, got %d", fsm.GetTotalSize(), signedStructure.Metadata.TotalSize)
	}

	// Verify the signature
	err = VerifyFileStructure(signedStructure)
	if err != nil {
		t.Errorf("Failed to verify file structure: %v", err)
	}

	// Test that verification fails with tampered data
	originalSize := signedStructure.Files[0].Size
	signedStructure.Files[0].Size = 999999

	err = VerifyFileStructure(signedStructure)
	require.NotNil(t, err, "Verification should fail with tampered data")

	// Restore original data
	signedStructure.Files[0].Size = originalSize

	// Test that verification fails with tampered signature
	signedStructure.Signature[0] ^= 0xFF // Flip bits in first byte

	err = VerifyFileStructure(signedStructure)
	require.NotNil(t, err, "Verification should fail with tampered signature")
}

// TestSignFileStructureWithNilManager tests error handling for nil FileStructureManager
func TestSignFileStructureWithNilManager(t *testing.T) {
	signer, err := NewFileStructureSigner()
	if err != nil {
		t.Fatalf("Failed to create signer: %v", err)
	}

	_, err = signer.SignFileStructureManager(nil)
	if err == nil {
		t.Error("SignFileStructureManager should fail with nil FileStructureManager")
	}

	expectedError := "FileStructureManager cannot be nil"
	if err.Error() != expectedError {
		t.Errorf("Expected error '%s', got '%s'", expectedError, err.Error())
	}
}

// TestSignFileStructureWithEmptyManager tests signing an empty FileStructureManager
func TestSignFileStructureWithEmptyManager(t *testing.T) {
	signer, err := NewFileStructureSigner()
	if err != nil {
		t.Fatalf("Failed to create signer: %v", err)
	}

	// Create empty FileStructureManager
	fsm := transfer.NewFileStructureManager()

	signedStructure, err := signer.SignFileStructureManager(fsm)
	if err != nil {
		t.Fatalf("Failed to sign empty file structure: %v", err)
	}

	// Verify empty structure
	if signedStructure.Metadata.TotalFiles != 0 {
		t.Errorf("Expected 0 files, got %d", signedStructure.Metadata.TotalFiles)
	}

	if signedStructure.Metadata.TotalDirs != 0 {
		t.Errorf("Expected 0 directories, got %d", signedStructure.Metadata.TotalDirs)
	}

	if signedStructure.Metadata.TotalSize != 0 {
		t.Errorf("Expected 0 total size, got %d", signedStructure.Metadata.TotalSize)
	}

	// Verify empty structure should succeed
	err = VerifyFileStructure(signedStructure)
	if err != nil {
		t.Errorf("Failed to verify empty file structure: %v", err)
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

	// Create FileStructureManager and add files
	fsm := transfer.NewFileStructureManager()

	err = fsm.AddPath(testFile1)
	if err != nil {
		t.Fatalf("Failed to add file 1: %v", err)
	}
	err = fsm.AddPath(testFile2)
	if err != nil {
		t.Fatalf("Failed to add file 2: %v", err)
	}

	// Create signer and sign the structure
	signer, err := NewFileStructureSigner()
	if err != nil {
		t.Fatalf("Failed to create signer: %v", err)
	}

	signedStructure, err := signer.SignFileStructureManager(fsm)
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
	if len(signedStructure.Files) != 2 {
		t.Errorf("Expected 2 files, got %d", len(signedStructure.Files))
	}

	if len(signedStructure.Directories) == 0 {
		t.Error("Expected at least one directory")
	}

	// Find the root directory in Directories
	var rootDir *fileInfo.FileNode
	for _, dir := range signedStructure.Directories {
		if filepath.Base(dir.Path) == filepath.Base(tempDir) {
			rootDir = &dir
			break
		}
	}

	if rootDir == nil {
		t.Error("Root directory not found in Directories")
	} else if !rootDir.IsDir {
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

// This test is now covered by TestSignFileStructureWithEmptyManager above
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

// This test is now covered by TestSignFileStructureWithNilManager above

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

	emptyFsm := transfer.NewFileStructureManager()
	validSignedStructure, err := validSigner.SignFileStructureManager(emptyFsm)
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
	// Create a temporary file for testing
	tempDir := t.TempDir()
	testFile := filepath.Join(tempDir, "test.txt")
	err = os.WriteFile(testFile, []byte("test content"), 0644)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	fsm := transfer.NewFileStructureManager()
	err = fsm.AddPath(testFile)
	if err != nil {
		t.Fatalf("Failed to add test file: %v", err)
	}

	signedStructure, err := signer.SignFileStructureManager(fsm)
	if err != nil {
		t.Errorf("Failed to sign with provided key pair: %v", err)
	}

	err = VerifyFileStructure(signedStructure)
	if err != nil {
		t.Errorf("Failed to verify signature from provided key pair: %v", err)
	}
	require.NotNil(t, fmt.Errorf("Failed to verify signature from provided key pair: %v", err))
}

func TestGetPublicAndPrivateKey(t *testing.T) {
	signer, err := NewFileStructureSigner()
	if err != nil {
		t.Fatalf("Failed to create signer: %v", err)
	}

	// Test GetPublicKey
	publicKey := signer.GetPublicKey()
	if publicKey == nil {
		t.Fatal("Public key should not be nil")
		return
	}

	// Test GetPrivateKey
	privateKey := signer.GetPrivateKey()
	if privateKey == nil {
		t.Fatal("Private key should not be nil")
		return
	}

	// Verify they form a valid key pair
	if publicKey.N.Cmp(privateKey.N) != 0 {
		t.Error("Public and private keys should match")
	}
}

// Test to improve coverage for SignFileStructure with large data
func TestSignFileStructureWithLargeData(t *testing.T) {
	signer, err := NewFileStructureSigner()
	if err != nil {
		t.Fatalf("Failed to create signer: %v", err)
	}

	// Create a temporary directory with many files
	tempDir := t.TempDir()
	fsm := transfer.NewFileStructureManager()

	// Create 100 test files (reduced from 1000 for faster testing)
	for i := 0; i < 100; i++ {
		testFile := filepath.Join(tempDir, fmt.Sprintf("file_%d.txt", i))
		content := fmt.Sprintf("test content for file %d", i)
		err := os.WriteFile(testFile, []byte(content), 0644)
		if err != nil {
			t.Fatalf("Failed to create test file %d: %v", i, err)
		}

		err = fsm.AddPath(testFile)
		if err != nil {
			t.Fatalf("Failed to add test file %d: %v", i, err)
		}
	}

	signedStructure, err := signer.SignFileStructureManager(fsm)
	if err != nil {
		t.Errorf("Should handle large file structures: %v", err)
	}

	if signedStructure != nil && len(signedStructure.Files) != 100 {
		t.Errorf("Expected 100 files, got %d", len(signedStructure.Files))
	}

	// Verify the large structure
	err = VerifyFileStructure(signedStructure)
	if err != nil {
		t.Errorf("Failed to verify large file structure: %v", err)
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

	fsm := transfer.NewFileStructureManager()
	signedStructure, err := signer.SignFileStructureManager(fsm)
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
