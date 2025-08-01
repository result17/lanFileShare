package crypto

import (
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/json"
	"fmt"

	"github.com/rescp17/lanFileSharer/pkg/fileInfo"
)

// FileStructureSigner handles signing for FileNode structures using composition
type FileStructureSigner struct {
	keyPair *KeyPair
}

// SignedFileStructure contains the file structure with digital signature
type SignedFileStructure struct {
	Files     []fileInfo.FileNode `json:"files"`
	PublicKey []byte              `json:"public_key"`
	Signature []byte              `json:"signature"`
}

const (
	KEY_PAIR_BIT_SIZE = 2048
)

// NewFileStructureSigner creates a new signer with generated RSA key pair
func NewFileStructureSigner() (*FileStructureSigner, error) {
	keyPair, err := GenerateKeyPair(KEY_PAIR_BIT_SIZE)
	if err != nil {
		return nil, fmt.Errorf("failed to generate key pair: %w", err)
	}

	return &FileStructureSigner{
		keyPair: keyPair,
	}, nil
}

// SignFileStructure creates a signed file structure from FileNode array
func (s *FileStructureSigner) SignFileStructure(files []fileInfo.FileNode) (*SignedFileStructure, error) {
	// Serialize the file structure for signing
	filesJSON, err := json.Marshal(files)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal files for signing: %w", err)
	}

	// Create hash of the file structure
	hash := sha256.Sum256(filesJSON)

	// Sign the hash using the composed key pair
	signature, err := rsa.SignPKCS1v15(rand.Reader, s.keyPair.PrivateKey, crypto.SHA256, hash[:])
	if err != nil {
		return nil, fmt.Errorf("failed to sign file structure: %w", err)
	}

	// Encode public key for transmission
	publicKeyBytes, err := x509.MarshalPKIXPublicKey(s.keyPair.PublicKey)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal public key: %w", err)
	}

	return &SignedFileStructure{
		Files:     files,
		PublicKey: publicKeyBytes,
		Signature: signature,
	}, nil
}

// VerifyFileStructure verifies the signature of a signed file structure
func VerifyFileStructure(signed *SignedFileStructure) error {
	// Parse the public key
	publicKeyInterface, err := x509.ParsePKIXPublicKey(signed.PublicKey)
	if err != nil {
		return fmt.Errorf("failed to parse public key: %w", err)
	}

	publicKey, ok := publicKeyInterface.(*rsa.PublicKey)
	if !ok {
		return fmt.Errorf("public key is not RSA key")
	}

	// Serialize the file structure for verification
	filesJSON, err := json.Marshal(signed.Files)
	if err != nil {
		return fmt.Errorf("failed to marshal files for verification: %w", err)
	}

	// Create hash of the file structure
	hash := sha256.Sum256(filesJSON)

	// Verify the signature
	err = rsa.VerifyPKCS1v15(publicKey, crypto.SHA256, hash[:], signed.Signature)
	if err != nil {
		return fmt.Errorf("signature verification failed: %w", err)
	}

	return nil
}

// CreateSignedFileStructure is a helper function to create signed structure from file paths
func CreateSignedFileStructure(filePaths []string) (*SignedFileStructure, error) {
	var nodes []fileInfo.FileNode
	for _, path := range filePaths {
		node, err := fileInfo.CreateNode(path)
		if err != nil {
			return nil, fmt.Errorf("failed to create node for %s: %w", path, err)
		}
		nodes = append(nodes, node)
	}

	signer, err := NewFileStructureSigner()
	if err != nil {
		return nil, fmt.Errorf("failed to create signer: %w", err)
	}

	return signer.SignFileStructure(nodes)
}

// GetKeyPair returns the underlying key pair for advanced usage
func (s *FileStructureSigner) GetKeyPair() *KeyPair {
	return s.keyPair
}

// GetPublicKey returns the public key for direct access
func (s *FileStructureSigner) GetPublicKey() *rsa.PublicKey {
	return s.keyPair.PublicKey
}

// GetPrivateKey returns the private key for direct access (use with caution)
func (s *FileStructureSigner) GetPrivateKey() *rsa.PrivateKey {
	return s.keyPair.PrivateKey
}

// GetPublicKeyBytes returns the public key as bytes for external use
func (s *FileStructureSigner) GetPublicKeyBytes() ([]byte, error) {
	return x509.MarshalPKIXPublicKey(s.keyPair.PublicKey)
}

// GetPrivateKeyBytes returns the private key as bytes (for testing/debugging only)
func (s *FileStructureSigner) GetPrivateKeyBytes() ([]byte, error) {
	return x509.MarshalPKCS1PrivateKey(s.keyPair.PrivateKey), nil
}

// NewFileStructureSignerFromKeyPair creates a signer from an existing key pair
func NewFileStructureSignerFromKeyPair(keyPair *KeyPair) *FileStructureSigner {
	return &FileStructureSigner{
		keyPair: keyPair,
	}
}
