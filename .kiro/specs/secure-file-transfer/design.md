# Design Document

## Overview

This design implements secure file transfer functionality for the lanFileSharer project, building upon the existing WebRTC signaling and connection infrastructure. The solution leverages the current `FileNode` checksum system from `pkg/fileInfo` package and adds digital signature verification for file structure authenticity, chunked file transmission over WebRTC data channels, and robust progress tracking.

## Architecture

### High-Level Flow

1. **Preparation Phase**: Sender uses existing `CreateNode()` to build file tree with checksums, then signs the structure
2. **Signaling Phase**: Enhanced /ask endpoint includes signature verification using existing FileNode structure
3. **Transfer Phase**: Files transmitted in chunks over WebRTC data channels, leveraging existing Path field for file access
4. **Verification Phase**: Receiver validates file integrity using existing `VerifySHA256()` method

### Component Integration

The design integrates with existing components:

- `pkg/webrtc/connection.go`: Implements `SendFiles()` method in `SenderConn`
- `api/receiver.go`: Enhances `AskPayload` with signature fields
- `pkg/fileInfo/fileNode.go`: Utilizes existing `FileNode` struct, `CreateNode()`, and checksum functionality
- `pkg/fileInfo/checksum.go`: Leverages existing `CalcChecksum()` and `VerifySHA256()` methods
- UI event system: Leverages existing progress messaging patterns

### Existing FileInfo Package Utilization

The implementation maximally reuses the existing `pkg/fileInfo` infrastructure:

#### FileNode Structure Reuse

- **Path field**: Used by sender to read file contents during transmission
- **Checksum field**: Pre-calculated by `CreateNode()`, used for integrity verification
- **Size field**: Used for progress tracking and transfer validation
- **IsDir field**: Determines file vs directory handling during reconstruction
- **Children field**: Preserves directory structure during transfer
- **MimeType field**: Maintains file type information for proper reconstruction

#### Checksum System Integration

- **`calculateSHA256()`**: Core hashing function reused for chunk-level integrity
- **`CalcChecksum()`**: Hierarchical checksum calculation for directories preserved
- **`VerifySHA256()`**: Direct integration for received file validation
- **Sorted child processing**: Existing deterministic directory checksum method maintained

## Components and Interfaces

### 1. Digital Signature Component

```go
// pkg/crypto/signature.go
type FileStructureSigner struct {
    privateKey crypto.PrivateKey
    publicKey  crypto.PublicKey
}

type SignedFileStructure struct {
    Files     []fileInfo.FileNode `json:"files"`     // Reuses existing FileNode with Checksum field
    PublicKey []byte              `json:"public_key"`
    Signature []byte              `json:"signature"`
}

func NewFileStructureSigner() (*FileStructureSigner, error)
func (s *FileStructureSigner) SignFileStructure(files []fileInfo.FileNode) (*SignedFileStructure, error)
func VerifyFileStructure(signed *SignedFileStructure) error

// Helper function to create signed structure from file paths
func CreateSignedFileStructure(filePaths []string) (*SignedFileStructure, error) {
    var nodes []fileInfo.FileNode
    for _, path := range filePaths {
        node, err := fileInfo.CreateNode(path)  // Uses existing CreateNode function
        if err != nil {
            return nil, err
        }
        nodes = append(nodes, node)
    }

    signer, err := NewFileStructureSigner()
    if err != nil {
        return nil, err
    }

    return signer.SignFileStructure(nodes)
}
```

### 2. File Transfer Protocol

```go
// pkg/transfer/protocol.go
type ChunkMessage struct {
    Type       MessageType `json:"type"`
    FileID     string      `json:"file_id"`
    SequenceNo uint32      `json:"sequence_no"`
    Data       []byte      `json:"data,omitempty"`
    ChunkHash  string      `json:"chunk_hash,omitempty"`
    TotalSize  int64       `json:"total_size,omitempty"`
}

type MessageType string
const (
    ChunkData       MessageType = "chunk_data"
    FileComplete    MessageType = "file_complete"
    TransferCancel  MessageType = "transfer_cancel"
    ProgressUpdate  MessageType = "progress_update"
)
```

### 3. Enhanced WebRTC Connection

```go
// Extends existing SenderConn
func (c *SenderConn) SendFiles(ctx context.Context, files []fileInfo.FileNode) error {
    // Implementation will use FileNode.Path field to read files
    // and FileNode.Checksum field for integrity verification
}

// New receiver-side handler
type FileReceiver struct {
    dataChannel   *webrtc.DataChannel
    fileBuffers   map[string]*FileBuffer
    progressChan  chan<- ProgressUpdate
    downloadDir   string  // Base directory for reconstructed files
}

// File reconstruction using existing checksum validation
func (fr *FileReceiver) reconstructFile(fileID string, expectedNode fileInfo.FileNode) error {
    // Reconstruct file from chunks
    // Use expectedNode.Checksum for validation via VerifySHA256()
    // Preserve directory structure using expectedNode.IsDir and Children
}
```

### 4. Enhanced API Payload

```go
// Enhanced AskPayload in api/receiver.go
type AskPayload struct {
    SignedFiles *crypto.SignedFileStructure `json:"signed_files"`
    Offer       webrtc.SessionDescription   `json:"offer"`
}
```

## Data Models

### File Buffer Management

```go
type FileBuffer struct {
    FileID       string
    FileNode     fileInfo.FileNode     // Contains all metadata including Checksum, Size, MimeType
    Chunks       map[uint32][]byte
    ReceivedSize int64
    TempFilePath string
    IsComplete   bool
}

// Leverages existing FileNode structure for metadata
func (fb *FileBuffer) ValidateIntegrity() error {
    if fb.IsComplete {
        // Create temporary FileNode for validation
        tempNode := fileInfo.FileNode{
            Path:     fb.TempFilePath,
            Checksum: fb.FileNode.Checksum,
        }
        // Use existing VerifySHA256 method
        valid, err := tempNode.VerifySHA256(fb.FileNode.Checksum)
        if err != nil {
            return err
        }
        if !valid {
            return fmt.Errorf("file integrity check failed for %s", fb.FileID)
        }
    }
    return nil
}
```

### Progress Tracking

```go
type ProgressUpdate struct {
    FileID          string
    FileName        string
    BytesTransferred int64
    TotalBytes      int64
    Status          TransferStatus
}

type TransferStatus string
const (
    StatusStarted    TransferStatus = "started"
    StatusInProgress TransferStatus = "in_progress"
    StatusCompleted  TransferStatus = "completed"
    StatusFailed     TransferStatus = "failed"
    StatusCancelled  TransferStatus = "cancelled"
)
```

## Error Handling

### Signature Verification Errors

- Invalid signature format
- Public key verification failure
- File structure tampering detection

### Transfer Errors

- Chunk transmission failures with exponential backoff retry
- WebRTC connection drops with reconnection attempts
- File reconstruction errors with cleanup

### Checksum Validation Errors

- Individual file checksum mismatches using existing `VerifySHA256()` method
- Directory structure checksum failures leveraging existing hierarchical checksum calculation
- Partial file cleanup on validation failure with proper temp file management
- Integration with existing `CalcChecksum()` error handling patterns

## Testing Strategy

### Unit Tests

1. **Signature Component Tests**

   - Key generation and signing functionality
   - Signature verification with valid/invalid signatures
   - Edge cases with malformed data

2. **File Transfer Protocol Tests**

   - Chunk serialization/deserialization
   - Message ordering and sequencing
   - Buffer management and file reconstruction using existing FileNode structure

3. **Integration with Existing Systems**
   - WebRTC data channel integration
   - FileNode checksum validation using existing `CalcChecksum()` and `VerifySHA256()` methods
   - Integration with existing `CreateNode()` function for file tree building
   - UI event messaging

### Integration Tests

1. **End-to-End Transfer Tests**

   - Complete file transfer workflow
   - Multiple file transfers
   - Large file handling

2. **Error Scenario Tests**

   - Network interruption recovery
   - Signature verification failures
   - Checksum validation failures

3. **Performance Tests**
   - Transfer speed benchmarks
   - Memory usage during large transfers
   - Concurrent transfer handling

### Security Tests

1. **Signature Security**

   - Tampered file structure detection
   - Invalid public key handling
   - Replay attack prevention

2. **Data Integrity**
   - Corrupted chunk detection using SHA-256 hashing
   - File reconstruction accuracy with existing FileNode structure preservation
   - Checksum validation effectiveness using existing `calculateSHA256()` and hierarchical directory checksums
   - Directory structure integrity using existing sorted child checksum concatenation method
