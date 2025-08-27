# Crypto Package

This package provides digital signature infrastructure for file structure verification in the lanFileSharer project.

## Overview

The crypto package implements RSA-based digital signatures to ensure the authenticity and integrity of file structures during transfer. It integrates with the `FileStructureManager` from the `pkg/transfer` package and the `FileNode` structure from the `pkg/fileInfo` package, providing cryptographic utilities for secure key handling and file structure verification.

## Key Features

- **RSA Key Generation**: Generate secure 2048-bit RSA key pairs
- **Digital Signatures**: Sign and verify file structure authenticity using PKCS#1 v1.5 with SHA-256
- **File Structure Integrity**: Prevent tampering with declared file structures
- **Enhanced Structure Support**: Support for directories, root nodes, and metadata
- **Cryptographic Utilities**: PEM encoding/decoding, key validation, secure comparison
- **Integration**: Seamless integration with `FileStructureManager` and `FileNode` infrastructure

## Core Components

### FileStructureSigner

The main component for creating and managing digital signatures:

```go
// Create a new signer with generated RSA keys
signer, err := NewFileStructureSigner()
if err != nil {
    log.Fatal(err)
}

// Sign a FileStructureManager
signedStructure, err := signer.SignFileStructureManager(fsm)
if err != nil {
    log.Fatal(err)
}
```

### SignedFileStructure

Contains the comprehensive file structure with its digital signature:

```go
type SignedFileStructure struct {
    Files       []fileInfo.FileNode `json:"files"`                // Files in the structure
    PublicKey   []byte              `json:"public_key"`           // RSA public key
    Signature   []byte              `json:"signature"`            // Digital signature
    Directories []fileInfo.FileNode `json:"directories,omitempty"` // Directory nodes
    RootNodes   []fileInfo.FileNode `json:"root_nodes,omitempty"`  // Root nodes
    Metadata    *StructureMetadata  `json:"metadata,omitempty"`    // Additional metadata
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

### Basic Usage with File Paths

```go
// Create signed structure from file paths
filePaths := []string{"document.txt", "image.jpg", "/path/to/directory"}
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

### Using FileStructureManager

```go
// Create FileStructureManager
fsm := transfer.NewFileStructureManager()

// Add files and directories
err := fsm.AddPath("/path/to/file.txt")
if err != nil {
    log.Fatal(err)
}

err = fsm.AddPath("/path/to/directory")
if err != nil {
    log.Fatal(err)
}

// Create signed structure from manager
signedStructure, err := CreateSignedFileStructureFromManager(fsm)
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

- **Key Size**: Minimum 1024-bit RSA keys, 2048-bit recommended for production
- **Signature Algorithm**: PKCS#1 v1.5 with SHA-256 hash function
- **Key Validation**: Comprehensive validation of key pairs and components
- **Secure Comparison**: Constant-time comparison for sensitive data
- **Data Consistency**: Consistent JSON serialization for signature verification
- **Error Handling**: Proper error handling for all cryptographic operations
- **Thread Safety**: FileStructureManager operations are thread-safe

## Integration with Transfer Package

The crypto package is designed to work seamlessly with the `pkg/transfer` and `pkg/fileInfo` infrastructure:

- **FileStructureManager Integration**: Direct support for signing FileStructureManager instances
- **FileNode Compatibility**: Uses existing `FileNode` structure with checksums and directory support
- **Automatic File Discovery**: Leverages `CreateNode()` function for recursive file tree building
- **Checksum Integration**: Integrates with existing SHA-256 checksum validation methods
- **Directory Structure**: Preserves complete directory hierarchy and metadata
- **Concurrent Operations**: Thread-safe operations with proper mutex handling

## Architecture

The signing process creates a comprehensive data structure that includes:

1. **Files**: All individual files with their metadata and checksums
2. **Directories**: Directory nodes with their own checksums
3. **Root Nodes**: Top-level entries in the file structure
4. **Statistics**: File count, directory count, and total size
5. **Metadata**: Creation timestamp, signing timestamp, and version information

This structure is JSON-serialized, hashed with SHA-256, and signed using RSA PKCS#1 v1.5.

## Requirements Addressed

This implementation addresses the following requirements from the secure file transfer specification:

- **3.1**: RSA key pair generation for session-based signing
- **3.2**: Cryptographic signing of FileNode tree structures with enhanced metadata
- **3.3**: Support for complex directory structures and file hierarchies
- **3.4**: Thread-safe operations for concurrent file structure management
- **3.5**: Signature verification using provided public keys with tamper detection

## Testing

The package includes comprehensive tests covering:

- **Key Management**: Key generation, validation, and PEM encoding/decoding
- **Signature Operations**: Creation and verification of digital signatures
- **FileStructureManager Integration**: Signing and verifying complex file structures
- **Directory Support**: Handling of nested directories and file hierarchies
- **Error Handling**: Comprehensive error scenarios and edge cases
- **Security Testing**: Tamper detection and invalid signature handling
- **Thread Safety**: Concurrent operations and deadlock prevention
- **Performance**: Large file structure handling and timeout prevention

Run tests with:

```bash
go test ./pkg/crypto -v
```

For specific test categories:

```bash
# Test signature operations
go test ./pkg/crypto -run TestSign -v

# Test key management
go test ./pkg/crypto -run TestGenerate -v

# Test verification
go test ./pkg/crypto -run TestVerify -v
```

## API Reference

### Core Functions

#### Signature Operations

- `NewFileStructureSigner() (*FileStructureSigner, error)` - Create new signer with generated keys
- `NewFileStructureSignerFromKeyPair(*KeyPair) *FileStructureSigner` - Create signer from existing keys
- `SignFileStructureManager(*transfer.FileStructureManager) (*SignedFileStructure, error)` - Sign file structure
- `VerifyFileStructure(*SignedFileStructure) error` - Verify signature authenticity

#### Helper Functions

- `CreateSignedFileStructure([]string) (*SignedFileStructure, error)` - Create signed structure from paths
- `CreateSignedFileStructureFromManager(*transfer.FileStructureManager) (*SignedFileStructure, error)` - Create from manager

#### Key Management

- `GenerateKeyPair(int) (*KeyPair, error)` - Generate RSA key pair (minimum 1024 bits)
- `PrivateKeyToPEM(*rsa.PrivateKey) ([]byte, error)` - Convert private key to PEM format
- `PublicKeyToPEM(*rsa.PublicKey) ([]byte, error)` - Convert public key to PEM format
- `PrivateKeyFromPEM([]byte) (*rsa.PrivateKey, error)` - Parse private key from PEM
- `PublicKeyFromPEM([]byte) (*rsa.PublicKey, error)` - Parse public key from PEM

#### Validation

- `ValidateKeyPair(*rsa.PrivateKey, *rsa.PublicKey) error` - Validate key pair consistency
- `ValidatePublicKey(*rsa.PublicKey) error` - Validate public key properties
- `ValidatePrivateKey(*rsa.PrivateKey) error` - Validate private key properties
- `SecureCompareBytes([]byte, []byte) bool` - Constant-time byte comparison

### Types

#### SignedFileStructure

```go
type SignedFileStructure struct {
    Files       []fileInfo.FileNode `json:"files"`
    PublicKey   []byte              `json:"public_key"`
    Signature   []byte              `json:"signature"`
    Directories []fileInfo.FileNode `json:"directories,omitempty"`
    RootNodes   []fileInfo.FileNode `json:"root_nodes,omitempty"`
    Metadata    *StructureMetadata  `json:"metadata,omitempty"`
}
```

#### StructureMetadata

```go
type StructureMetadata struct {
    TotalFiles int   `json:"total_files"`
    TotalDirs  int   `json:"total_dirs"`
    TotalSize  int64 `json:"total_size"`
    CreatedAt  int64 `json:"created_at"`
    SignedAt   int64 `json:"signed_at"`
    Version    string `json:"version"`
}
```

#### KeyPair

```go
type KeyPair struct {
    PrivateKey *rsa.PrivateKey
    PublicKey  *rsa.PublicKey
}
```

## Common Issues and Solutions

### Signature Verification Failures

**Problem**: `signature verification failed: crypto/rsa: verification error`

**Causes and Solutions**:

1. **Data Structure Mismatch**: Ensure signing and verification use consistent data structures
2. **JSON Serialization Issues**: Avoid mixing pointer and value types in JSON serialization
3. **Timestamp Differences**: Verification recreates the exact signing timestamp from metadata
4. **File Content Changes**: Files modified after signing will cause verification failure

### Thread Safety Issues

**Problem**: Deadlocks or race conditions with FileStructureManager

**Solution**: The current implementation uses proper mutex locking. Avoid calling `AddFileNode()` from within `AddPath()` as it can cause deadlocks.

### Performance Considerations

- **Large File Structures**: RSA signing time increases with data size
- **Directory Recursion**: Deep directory structures may impact performance
- **Concurrent Operations**: Use separate FileStructureManager instances for concurrent operations

## Migration Guide

### From Previous Versions

If upgrading from earlier versions that used pointer slices:

```go
// Old (problematic)
signatureData := struct {
    Files []*fileInfo.FileNode `json:"files"`
}{
    Files: pointerSlice,
}

// New (correct)
signatureData := struct {
    Files []fileInfo.FileNode `json:"files"`
}{
    Files: valueSlice,
}
```

### Integration with WebRTC

The crypto package integrates seamlessly with the WebRTC signaling process:

```go
// In sender
fsm := transfer.NewFileStructureManager()
// ... add files to fsm ...

signedStructure, err := crypto.CreateSignedFileStructureFromManager(fsm)
if err != nil {
    return err
}

// Send signedStructure in WebRTC offer
offer := webrtc.SessionDescription{...}
signaler.SendOffer(ctx, offer, signedStructure)
```

## Best Practices

1. **Key Management**: Generate new key pairs for each session
2. **Error Handling**: Always check errors from cryptographic operations
3. **Validation**: Validate file structures before and after signing
4. **Testing**: Test with various file structures and edge cases
5. **Security**: Never log or expose private keys
6. **Performance**: Consider file structure size when designing the application

## Contributing

When contributing to the crypto package:

1. Ensure all tests pass: `go test ./pkg/crypto -v`
2. Add tests for new functionality
3. Follow Go cryptography best practices
4. Document security considerations
5. Test with various file structures and sizes
