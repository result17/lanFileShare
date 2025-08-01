# Crypto Package

This package provides digital signature infrastructure for file structure verification in the lanFileSharer project.

## Overview

The crypto package implements RSA-based digital signatures to ensure the authenticity and integrity of file structures during transfer. It builds upon the existing `FileNode` structure from the `pkg/fileInfo` package and provides cryptographic utilities for secure key handling.

## Key Features

- **RSA Key Generation**: Generate secure 2048-bit RSA key pairs
- **Digital Signatures**: Sign and verify file structure authenticity
- **File Structure Integrity**: Prevent tampering with declared file structures
- **Cryptographic Utilities**: PEM encoding/decoding, key validation, secure comparison
- **Integration**: Seamless integration with existing `FileNode` and checksum infrastructure

## Core Components

### FileStructureSigner

The main component for creating and managing digital signatures:

```go
// Create a new signer with generated RSA keys
signer, err := NewFileStructureSigner()
if err != nil {
    log.Fatal(err)
}

// Sign a file structure
signedStructure, err := signer.SignFileStructure(fileNodes)
if err != nil {
    log.Fatal(err)
}
```

### SignedFileStructure

Contains the file structure with its digital signature:

```go
type SignedFileStructure struct {
    Files     []fileInfo.FileNode `json:"files"`     // File structure
    PublicKey []byte              `json:"public_key"` // RSA public key
    Signature []byte              `json:"signature"`  // Digital signature
}
```

### Verification

Verify the authenticity of a signed file structure:

```go
err := VerifyFileStructure(signedStructure)
if err != nil {
    log.Printf("Signature verification failed: %v", err)
    return
}
```

## Usage Examples

### Basic Usage

```go
// Create signed structure from file paths
filePaths := []string{"document.txt", "image.jpg"}
signedStructure, err := CreateSignedFileStructure(filePaths)
if err != nil {
    log.Fatal(err)
}

// Verify the signature
err = VerifyFileStructure(signedStructure)
if err != nil {
    log.Fatal("Verification failed:", err)
}
```

### Manual Key Management

```go
// Generate key pair
keyPair, err := GenerateKeyPair(2048)
if err != nil {
    log.Fatal(err)
}

// Convert to PEM format for storage
privateKeyPEM, err := PrivateKeyToPEM(keyPair.PrivateKey)
if err != nil {
    log.Fatal(err)
}

publicKeyPEM, err := PublicKeyToPEM(keyPair.PublicKey)
if err != nil {
    log.Fatal(err)
}

// Later, parse from PEM
privateKey, err := PrivateKeyFromPEM(privateKeyPEM)
if err != nil {
    log.Fatal(err)
}
```

## Security Considerations

- **Key Size**: Minimum 1024-bit RSA keys, 2048-bit recommended
- **Signature Algorithm**: PKCS#1 v1.5 with SHA-256
- **Key Validation**: Comprehensive validation of key pairs and components
- **Secure Comparison**: Constant-time comparison for sensitive data
- **Error Handling**: Proper error handling for all cryptographic operations

## Integration with FileInfo Package

The crypto package is designed to work seamlessly with the existing `pkg/fileInfo` infrastructure:

- Uses existing `FileNode` structure with checksums
- Leverages `CreateNode()` function for file tree building
- Integrates with existing checksum validation methods
- Preserves directory structure and metadata

## Requirements Addressed

This implementation addresses the following requirements from the secure file transfer specification:

- **3.1**: RSA key pair generation for session-based signing
- **3.2**: Cryptographic signing of FileNode tree structures
- **3.5**: Signature verification using provided public keys

## Testing

The package includes comprehensive tests covering:

- Key generation and validation
- Signature creation and verification
- PEM encoding/decoding
- Error handling scenarios
- Integration with FileNode structures
- Security edge cases

Run tests with:
```bash
go test ./pkg/crypto -v
```

## API Reference

### Functions

- `NewFileStructureSigner() (*FileStructureSigner, error)` - Create new signer
- `VerifyFileStructure(*SignedFileStructure) error` - Verify signature
- `CreateSignedFileStructure([]string) (*SignedFileStructure, error)` - Helper function
- `GenerateKeyPair(int) (*KeyPair, error)` - Generate RSA key pair
- `PrivateKeyToPEM(*rsa.PrivateKey) ([]byte, error)` - Convert to PEM
- `PublicKeyToPEM(*rsa.PublicKey) ([]byte, error)` - Convert to PEM
- `PrivateKeyFromPEM([]byte) (*rsa.PrivateKey, error)` - Parse from PEM
- `PublicKeyFromPEM([]byte) (*rsa.PublicKey, error)` - Parse from PEM
- `ValidateKeyPair(*rsa.PrivateKey, *rsa.PublicKey) error` - Validate key pair
- `ValidatePublicKey(*rsa.PublicKey) error` - Validate public key
- `ValidatePrivateKey(*rsa.PrivateKey) error` - Validate private key
- `SecureCompareBytes([]byte, []byte) bool` - Secure comparison