# Receiver Enhancement Design Document

## Overview

This design document details the enhanced implementation of receiver functionality in the lanFileSharer project. Building upon the existing FileReceiver implementation, it adds integrity verification, progress tracking, error handling, and user experience improvements to ensure reliable and user-friendly file reception processes.

## Architecture

### High-Level Architecture

```
┌─────────────────┐    ┌──────────────────┐    ┌─────────────────┐
│   UI Layer      │    │  Receiver App    │    │  File Receiver  │
│                 │◄──►│                  │◄──►│                 │
│ - Progress View │    │ - Request Mgmt   │    │ - Chunk Mgmt    │
│ - File List     │    │ - Connection     │    │ - File Writing  │
│ - Controls      │    │ - Event Handling │    │ - Verification  │
└─────────────────┘    └──────────────────┘    └─────────────────┘
                                │                        │
                       ┌──────────────────┐    ┌─────────────────┐
                       │  Transfer        │    │  File Reception │
                       │  Status Manager  │    │  State          │
                       │                  │    │                 │
                       │ - Progress Track │    │ - Chunk Buffer  │
                       │ - Event System   │    │ - File Handle   │
                       │ - Error Handling │    │ - Verification  │
                       └──────────────────┘    └─────────────────┘
```

### Core Components

#### 1. Enhanced FileReceiver

Core logic for file reception:

- Data chunk reception and buffering
- File reconstruction and writing
- Integrity verification
- Error handling and recovery

#### 2. FileReceptionManager

Multi-file reception state management:

- File reception queue management
- Progress aggregation and reporting
- Resource allocation and optimization
- State synchronization

#### 3. IntegrityVerifier

File integrity verification component:

- SHA-256 hash verification
- Digital signature verification
- Corruption detection and reporting
- Verification result management

#### 4. ProgressTracker

Reception progress tracking component:

- Real-time progress calculation
- Transfer speed statistics
- ETA estimation
- Performance metrics collection

## Components and Interfaces

### Enhanced FileReceiver Interface

```go
type FileReceiver struct {
    serializer       transfer.MessageSerializer
    currentFiles     map[string]*FileReception
    outputDir        string
    progressTracker  *ProgressTracker
    verifier         *IntegrityVerifier
    statusManager    *transfer.TransferStatusManager
    mu               sync.RWMutex

    // New fields
    maxConcurrentFiles int
    bufferSize         int
    tempDir           string
    eventChan         chan ReceptionEvent
}

type FileReception struct {
    FilePath        string
    FileName        string
    TotalSize       int64
    ReceivedSize    int64
    ExpectedHash    string
    File            *os.File
    TempFile        *os.File  // Temporary file for safe writing
    Chunks          map[uint32][]byte
    LastSequence    uint32
    IsComplete      bool

    // New fields
    StartTime       time.Time
    LastUpdateTime  time.Time
    TransferRate    float64
    ETA             time.Duration
    ErrorCount      int
    Status          ReceptionStatus
    Checksum        string
}

type ReceptionStatus int
const (
    StatusPending ReceptionStatus = iota
    StatusReceiving
    StatusVerifying
    StatusCompleted
    StatusFailed
    StatusCancelled
)
```

### File Reception Manager

```go
type FileReceptionManager struct {
    fileReceiver    *FileReceiver
    activeFiles     map[string]*FileReception
    completedFiles  []string
    failedFiles     []string
    totalBytes      int64
    receivedBytes   int64
    sessionID       string
    mu              sync.RWMutex
}

func NewFileReceptionManager(outputDir string) *FileReceptionManager
func (frm *FileReceptionManager) StartReception(fileList []FileInfo) error
func (frm *FileReceptionManager) ProcessChunk(data []byte) error
func (frm *FileReceptionManager) GetOverallProgress() ProgressInfo
func (frm *FileReceptionManager) PauseReception(fileID string) error
func (frm *FileReceptionManager) ResumeReception(fileID string) error
func (frm *FileReceptionManager) CancelReception(fileID string) error
```

### Integrity Verifier

```go
type IntegrityVerifier struct {
    signer *crypto.FileStructureSigner
}

func NewIntegrityVerifier() *IntegrityVerifier
func (iv *IntegrityVerifier) VerifyFileHash(filePath, expectedHash string) error
func (iv *IntegrityVerifier) VerifyFileStructure(signedStructure *crypto.SignedFileStructure) error
func (iv *IntegrityVerifier) VerifyChunkHash(data []byte, expectedHash string) error
```

### Progress Tracker

```go
type ProgressTracker struct {
    startTime       time.Time
    lastUpdateTime  time.Time
    bytesReceived   int64
    totalBytes      int64
    transferRates   []float64  // Sliding window for average speed calculation
    windowSize      int
    mu              sync.RWMutex
}

func NewProgressTracker(totalBytes int64) *ProgressTracker
func (pt *ProgressTracker) UpdateProgress(bytesReceived int64)
func (pt *ProgressTracker) GetCurrentRate() float64
func (pt *ProgressTracker) GetETA() time.Duration
func (pt *ProgressTracker) GetProgressPercentage() float64
```

## Data Models

### Reception Event System

```go
type ReceptionEvent struct {
    Type      ReceptionEventType
    FileID    string
    FileName  string
    Progress  ProgressInfo
    Error     error
    Timestamp time.Time
}

type ReceptionEventType int
const (
    EventFileStarted ReceptionEventType = iota
    EventFileProgress
    EventFileCompleted
    EventFileFailed
    EventFileVerified
    EventSessionCompleted
)

type ProgressInfo struct {
    FileID           string
    FileName         string
    BytesReceived    int64
    TotalBytes       int64
    Percentage       float64
    TransferRate     float64
    ETA              time.Duration
    Status           ReceptionStatus
}
```

### Configuration Management

```go
type ReceiverConfig struct {
    OutputDirectory     string
    TempDirectory       string
    MaxConcurrentFiles  int
    BufferSize          int
    VerifyIntegrity     bool
    AutoAcceptFiles     bool
    MaxFileSize         int64
    AllowedFileTypes    []string

    // Performance settings
    WriteBufferSize     int
    ProgressUpdateInterval time.Duration

    // Security settings
    RequireSignature    bool
    TrustedSenders      []string
}
```

## Error Handling

### Error Classification

```go
type ReceptionError struct {
    Type        ErrorType
    FileID      string
    Message     string
    Recoverable bool
    RetryCount  int
    Timestamp   time.Time
}

type ErrorType int
const (
    ErrorNetworkTimeout ErrorType = iota
    ErrorDiskFull
    ErrorPermissionDenied
    ErrorCorruptedData
    ErrorVerificationFailed
    ErrorInvalidSignature
    ErrorFileTooBig
    ErrorUnsupportedType
)
```

### Error Handling Strategy

```go
type ErrorHandler struct {
    maxRetries      int
    retryDelay      time.Duration
    backoffFactor   float64
}

func (eh *ErrorHandler) HandleError(err *ReceptionError) ErrorAction
func (eh *ErrorHandler) ShouldRetry(err *ReceptionError) bool
func (eh *ErrorHandler) GetRetryDelay(retryCount int) time.Duration

type ErrorAction int
const (
    ActionRetry ErrorAction = iota
    ActionSkip
    ActionCancel
    ActionPause
)
```

## Testing Strategy

### Unit Tests

1. **FileReceiver Tests**

   - Data chunk reception and ordering
   - File reconstruction accuracy
   - Error handling mechanisms
   - Concurrent safety

2. **IntegrityVerifier Tests**

   - Hash verification accuracy
   - Signature verification functionality
   - Corrupted data detection
   - Performance benchmarks

3. **ProgressTracker Tests**
   - Progress calculation accuracy
   - Speed statistics precision
   - ETA estimation reasonableness
   - Concurrent update safety

### Integration Tests

1. **End-to-End Reception Tests**

   - Complete file transfer workflow
   - Multi-file concurrent reception
   - Network interruption recovery
   - Error scenario handling

2. **Performance Tests**

   - Large file reception performance
   - Memory usage optimization
   - Disk I/O efficiency
   - Concurrent processing capability

3. **Security Tests**
   - Malicious file detection
   - Signature verification strength
   - Permission control effectiveness
   - Temporary file cleanup

## Implementation Phases

### Phase 1: Core Enhancement (Week 1)

- Enhance FileReceiver implementation
- Add integrity verification
- Implement basic progress tracking
- Improve error handling

### Phase 2: Manager Implementation (Week 1-2)

- Implement FileReceptionManager
- Add multi-file management
- Implement state synchronization
- Integrate event system

### Phase 3: User Experience (Week 2)

- UI integration and improvements
- Real-time progress display
- User control functionality
- Error message presentation

### Phase 4: Performance Optimization (Week 2-3)

- Memory usage optimization
- Disk I/O optimization
- Concurrent processing optimization
- Performance monitoring

### Phase 5: Security Enhancement (Week 3)

- Signature verification integration
- Secure file handling
- Permission control
- Threat detection

### Phase 6: Testing and Documentation (Week 3-4)

- Comprehensive test coverage
- Performance benchmarking
- Security testing
- Documentation completion

## Integration Points

### Integration with Existing Systems

1. **WebRTC Connection Integration**

   - Data channel message handling
   - Connection state monitoring
   - Network performance metrics

2. **Transfer Status Management Integration**

   - Status update synchronization
   - Event notification mechanisms
   - Progress aggregation

3. **UI System Integration**

   - Real-time status updates
   - User interaction handling
   - Error message display

4. **Encryption System Integration**
   - Encrypted data processing
   - Key management
   - Security verification

## Security Considerations

### Data Protection

- Secure temporary file handling
- Sensitive data cleanup in memory
- File permission control
- Access logging

### Threat Protection

- Malicious file detection
- Resource exhaustion protection
- Denial of service attack protection
- Data integrity protection

### Privacy Protection

- File metadata protection
- Transfer history privacy
- User choice control
- Data cleanup mechanisms
