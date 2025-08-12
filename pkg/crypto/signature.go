package crypto

import (
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"time"

	"github.com/rescp17/lanFileSharer/pkg/fileInfo"
	"github.com/rescp17/lanFileSharer/pkg/transfer"
)

// FileStructureSigner handles signing for FileNode structures using composition
type FileStructureSigner struct {
	keyPair *KeyPair
}

// SignedFileStructure contains the file structure with digital signature
type SignedFileStructure struct {
	Files       []fileInfo.FileNode `json:"files"`
	PublicKey   []byte              `json:"public_key"`
	Signature   []byte              `json:"signature"`
	
	// Enhanced structure information
	Directories []fileInfo.FileNode `json:"directories,omitempty"`
	RootNodes   []fileInfo.FileNode `json:"root_nodes,omitempty"`
	Metadata    *StructureMetadata  `json:"metadata,omitempty"`
}

// StructureMetadata contains additional information about the file structure
type StructureMetadata struct {
	TotalFiles    int   `json:"total_files"`
	TotalDirs     int   `json:"total_dirs"`
	TotalSize     int64 `json:"total_size"`
	CreatedAt     int64 `json:"created_at"`
	SignedAt      int64 `json:"signed_at"`
	Version       string `json:"version"`
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

// SignFileStructureManager creates a signed file structure from FileStructureManager
func (s *FileStructureSigner) SignFileStructureManager(fsm *transfer.FileStructureManager) (*SignedFileStructure, error) {
	if fsm == nil {
		return nil, fmt.Errorf("FileStructureManager cannot be nil")
	}
	
	// Convert pointer slices to value slices for consistent JSON serialization
	var files []fileInfo.FileNode
	for _, file := range fsm.GetAllFiles() {
		if file != nil {
			files = append(files, *file)
		}
	}
	
	var dirs []fileInfo.FileNode
	for _, dir := range fsm.GetAllDirs() {
		if dir != nil {
			dirs = append(dirs, *dir)
		}
	}
	
	var rootNodes []fileInfo.FileNode
	for _, root := range fsm.RootNodes {
		if root != nil {
			rootNodes = append(rootNodes, *root)
		}
	}

	// Create comprehensive signature data
	signatureData := struct {
		Files     []fileInfo.FileNode `json:"files"`
		Dirs      []fileInfo.FileNode `json:"directories"`
		RootNodes []fileInfo.FileNode `json:"root_nodes"`
		Stats     struct {
			FileCount int   `json:"file_count"`
			DirCount  int   `json:"dir_count"`
			TotalSize int64 `json:"total_size"`
		} `json:"stats"`
		Timestamp int64 `json:"timestamp"`
	}{
		Files:     files,
		Dirs:      dirs,
		RootNodes: rootNodes,
		Stats: struct {
			FileCount int   `json:"file_count"`
			DirCount  int   `json:"dir_count"`
			TotalSize int64 `json:"total_size"`
		}{
			FileCount: fsm.GetFileCount(),
			DirCount:  fsm.GetDirCount(),
			TotalSize: fsm.GetTotalSize(),
		},
		Timestamp: time.Now().Unix(),
	}
	
	// Serialize and sign
	dataJSON, err := json.Marshal(signatureData)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal signature data: %w", err)
	}
	
	hash := sha256.Sum256(dataJSON)
	signature, err := rsa.SignPKCS1v15(rand.Reader, s.keyPair.PrivateKey, crypto.SHA256, hash[:])
	if err != nil {
		return nil, fmt.Errorf("failed to sign data: %w", err)
	}
	
	publicKeyBytes, err := x509.MarshalPKIXPublicKey(s.keyPair.PublicKey)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal public key: %w", err)
	}
	
	// Create metadata
	metadata := &StructureMetadata{
		TotalFiles: fsm.GetFileCount(),
		TotalDirs:  fsm.GetDirCount(),
		TotalSize:  fsm.GetTotalSize(),
		CreatedAt:  time.Now().Unix(),
		SignedAt:   time.Now().Unix(),
		Version:    "1.0",
	}
	
	return &SignedFileStructure{
		Files:       files,
		PublicKey:   publicKeyBytes,
		Signature:   signature,
		Directories: dirs,
		RootNodes:   rootNodes,
		Metadata:    metadata,
	}, nil
}

// CreateSignedFileStructureFromManager creates a signed structure from FileStructureManager
func CreateSignedFileStructureFromManager(fsm *transfer.FileStructureManager) (*SignedFileStructure, error) {
	signer, err := NewFileStructureSigner()
	if err != nil {
		return nil, fmt.Errorf("failed to create signer: %w", err)
	}
	
	return signer.SignFileStructureManager(fsm)
}

// VerifyFileStructure verifies the digital signature of a SignedFileStructure
func VerifyFileStructure(signedStructure *SignedFileStructure) error {
	if signedStructure == nil {
		return fmt.Errorf("signed structure cannot be nil")
	}

	// Parse the public key
	publicKeyInterface, err := x509.ParsePKIXPublicKey(signedStructure.PublicKey)
	if err != nil {
		return fmt.Errorf("failed to parse public key: %w", err)
	}

	publicKey, ok := publicKeyInterface.(*rsa.PublicKey)
	if !ok {
		return fmt.Errorf("public key is not RSA")
	}

	// Recreate the signature data structure
	signatureData := struct {
		Files     []fileInfo.FileNode `json:"files"`
		Dirs      []fileInfo.FileNode `json:"directories"`
		RootNodes []fileInfo.FileNode `json:"root_nodes"`
		Stats     struct {
			FileCount int   `json:"file_count"`
			DirCount  int   `json:"dir_count"`
			TotalSize int64 `json:"total_size"`
		} `json:"stats"`
		Timestamp int64 `json:"timestamp"`
	}{
		Files:     signedStructure.Files,
		Dirs:      signedStructure.Directories,
		RootNodes: signedStructure.RootNodes,
		Stats: struct {
			FileCount int   `json:"file_count"`
			DirCount  int   `json:"dir_count"`
			TotalSize int64 `json:"total_size"`
		}{
			FileCount: len(signedStructure.Files),
			DirCount:  len(signedStructure.Directories),
			TotalSize: func() int64 {
				var total int64
				for _, file := range signedStructure.Files {
					total += file.Size
				}
				return total
			}(),
		},
		Timestamp: func() int64 {
			if signedStructure.Metadata != nil {
				return signedStructure.Metadata.SignedAt
			}
			return 0
		}(),
	}

	// Serialize and verify
	dataJSON, err := json.Marshal(signatureData)
	if err != nil {
		return fmt.Errorf("failed to marshal signature data: %w", err)
	}

	hash := sha256.Sum256(dataJSON)
	err = rsa.VerifyPKCS1v15(publicKey, crypto.SHA256, hash[:], signedStructure.Signature)
	if err != nil {
		return fmt.Errorf("signature verification failed: %w", err)
	}

	return nil
}

// CreateSignedFileStructure creates a signed file structure from file paths
func CreateSignedFileStructure(filePaths []string) (*SignedFileStructure, error) {
	if len(filePaths) == 0 {
		return nil, fmt.Errorf("file paths cannot be empty")
	}

	// Create FileStructureManager from paths
	fsm := transfer.NewFileStructureManager()
	
	for _, path := range filePaths {
		err := fsm.AddPath(path)
		if err != nil {
			return nil, fmt.Errorf("failed to add path %s: %w", path, err)
		}
	}

	// Create signed structure
	return CreateSignedFileStructureFromManager(fsm)
}