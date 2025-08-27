# Transfer Status Management Design

## Overview

This document outlines the design for implementing comprehensive transfer status management in the lanFileSharer project. The system will provide real-time transfer monitoring, state management, and progress tracking capabilities that integrate seamlessly with the existing file transfer infrastructure.

## Architecture

### High-Level Architecture

```
┌─────────────────┐    ┌──────────────────┐    ┌─────────────────┐
│   UI Layer      │    │  Unified         │    │  Transfer       │
│                 │◄──►│  Transfer        │◄──►│  Status         │
│ - Progress Bars │    │  Manager         │    │  Manager        │
│ - Status Display│    │                  │    │                 │
│ - Controls      │    │ - File Queue     │    │ - Session Status│
└─────────────────┘    │ - Chunkers       │    │ - Event System  │
                       │ - Coordination   │    │ - State Mgmt    │
                       └──────────────────┘    └─────────────────┘
                                │                        │
                       ┌──────────────────┐    ┌─────────────────┐
                       │  Session         │    │  Transfer       │
                       │  Transfer        │    │  Status         │
                       │  Status          │    │                 │
                       │                  │    │ - File Progress │
                       │ - Overall Progress│    │ - State Info    │
                       │ - Current File   │    │ - Metrics       │
                       │ - Session State  │    │ - Timestamps    │
                       └──────────────────┘    └─────────────────┘
```

### Core Components

#### 1. UnifiedTransferManager

Central component responsible for:

- Managing file queue and chunkers
- Coordinating file transfers
- Integrating with TransferStatusManager
- Providing compatibility with existing code
- Managing transfer lifecycle

#### 2. TransferStatusManager

Simplified component focused on:

- Managing single SessionTransferStatus
- Tracking current file transfer progress
- Managing session state transitions
- Emitting status change events
- Providing session-level metrics

#### 3. SessionTransferStatus

Data structure representing the entire transfer session:

- Overall session progress aggregation
- Current file transfer status
- Session state management
- File counts (total, completed, failed, pending)
- Session-level timestamps and metrics

#### 4. TransferStatus

Data structure representing individual file transfer state:

- Progress information (bytes sent, total size, percentage)
- State information (pending, active, paused, completed, failed)
- Performance metrics (transfer rate, ETA)
- Error information and retry counts
- Timestamps for lifecycle events

#### 5. StatusListener Interface

Simplified event system for real-time status updates:

- Direct method-based notifications
- File status change events
- Session status change events
- Asynchronous event delivery

## Components and Interfaces

### UnifiedTransferManager Interface

```go
type UnifiedTransferManager interface {
    // File management
    AddFile(node *fileInfo.FileNode) error
    GetAllFiles() []*fileInfo.FileNode
    GetChunker(filePath string) (*Chunker, bool)
    GetTotalBytes() int64

    // Queue management
    GetNextPendingFile() (*fileInfo.FileNode, bool)
    MarkFileCompleted(filePath string) error
    MarkFileFailed(filePath string) error
    GetQueueStatus() (pending, completed, failed int)

    // Transfer control (integrated with status manager)
    StartTransfer(filePath string) error
    UpdateProgress(filePath string, bytesSent int64) error
    CompleteTransfer(filePath string) error
    FailTransfer(filePath string, err error) error
    PauseTransfer(filePath string) error
    ResumeTransfer(filePath string) error

    // Status querying
    GetSessionStatus() *SessionTransferStatus
    GetFileStatus(filePath string) (*TransferStatus, error)

    // Event management
    AddStatusListener(listener StatusListener)

    // Resource management
    Close() error
}

type TransferStatusManager interface {
    // Session management
    InitializeSession(sessionID string, totalFiles int, totalBytes int64) error
    GetSessionStatus() (*SessionTransferStatus, error)
    ResetSession()

    // File transfer management
    StartFileTransfer(filePath string, fileSize int64) (*TransferStatus, error)
    UpdateFileProgress(bytesSent int64) error
    CompleteCurrentFile() error
    FailCurrentFile(err error) error
    PauseCurrentFile() error
    ResumeCurrentFile() error

    // Status querying
    GetCurrentFile() (*TransferStatus, error)
    IsSessionActive() bool

    // Event management
    AddStatusListener(listener StatusListener)

    // Cleanup
    Clear()
}
```

### TransferStatus Data Structure

```go
type TransferStatus struct {
    FilePath        string
    SessionID       string
    State           TransferState

    // Progress information
    BytesSent       int64
    TotalBytes      int64
    ChunksSent      int
    TotalChunks     int

    // Performance metrics
    TransferRate    float64  // bytes per second
    ETA             time.Duration

    // Lifecycle timestamps
    StartTime       time.Time
    LastUpdateTime  time.Time
    CompletionTime  *time.Time

    // Error handling
    LastError       error
    RetryCount      int
    MaxRetries      int

    // Metadata
    FileSize        int64
    FileChecksum    string
    Priority        int
}

type TransferState int
const (
    TransferStatePending TransferState = iota
    TransferStateActive
    TransferStatePaused
    TransferStateCompleted
    TransferStateFailed
    TransferStateCancelled
)
```

### SessionTransferStatus Management

```go
type SessionTransferStatus struct {
    // Session identification
    SessionID string

    // File counts
    TotalFiles     int
    CompletedFiles int
    FailedFiles    int
    PendingFiles   int

    // Byte progress
    TotalBytes      int64
    BytesCompleted  int64
    OverallProgress float64 // 0-100 percentage

    // Current file being transferred
    CurrentFile *TransferStatus

    // Session timing
    StartTime      time.Time
    LastUpdateTime time.Time
    CompletionTime *time.Time

    // Session state
    State StatusSessionState
}

type StatusSessionState int
const (
    StatusSessionStateActive StatusSessionState = iota
    StatusSessionStatePaused
    StatusSessionStateCompleted
    StatusSessionStateFailed
    StatusSessionStateCancelled
)
```

### Event System Design

```go
type StatusListener interface {
    // OnFileStatusChanged is called when an individual file's status changes
    OnFileStatusChanged(filePath string, oldStatus, newStatus *TransferStatus)

    // OnSessionStatusChanged is called when the overall session status changes
    OnSessionStatusChanged(oldStatus, newStatus *SessionTransferStatus)
}
```

### Simplified Event Flow

The event system has been simplified to use direct method calls instead of complex event structures:

1. **File Status Changes**: When a file's transfer status changes (start, progress, complete, fail), the `OnFileStatusChanged` method is called on all registered listeners.

2. **Session Status Changes**: When the overall session status changes (progress updates, completion, failure), the `OnSessionStatusChanged` method is called.

3. **Asynchronous Delivery**: Events are delivered asynchronously using goroutines to prevent blocking the main transfer operations.

4. **Error Handling**: Event delivery failures are handled gracefully without affecting transfer operations.

## Data Models

### Transfer History Schema

```go
type TransferRecord struct {
    ID              string
    SessionID       string
    FilePath        string
    FileName        string
    FileSize        int64
    FileChecksum    string

    // Transfer details
    StartTime       time.Time
    EndTime         time.Time
    Duration        time.Duration
    BytesTransferred int64
    AverageRate     float64

    // Final state
    FinalState      TransferState
    ErrorMessage    string
    RetryCount      int

    // Metadata
    RecipientID     string
    TransferMethod  string
    CreatedAt       time.Time
}
```

### Configuration Schema

```go
type TransferConfig struct {
    // Concurrency limits
    MaxConcurrentTransfers int
    MaxConcurrentChunks    int

    // Performance settings
    ChunkSize              int32
    BufferSize             int
    RateCalculationWindow  time.Duration

    // Retry policy
    DefaultRetryPolicy     *RetryPolicy

    // History settings
    HistoryRetentionDays   int
    MaxHistoryRecords      int

    // Event settings
    EventBufferSize        int
    EventDeliveryTimeout   time.Duration
}

type RetryPolicy struct {
    MaxRetries      int
    InitialDelay    time.Duration
    BackoffFactor   float64
    MaxDelay        time.Duration
    RetryableErrors []error
}
```

## Error Handling

### Error Categories

1. **Recoverable Errors**

   - Network timeouts
   - Temporary connection issues
   - Rate limiting
   - Temporary file access issues

2. **Non-Recoverable Errors**

   - File not found
   - Permission denied
   - Disk full
   - Invalid file format
   - Authentication failures

3. **System Errors**
   - Out of memory
   - Database connection failures
   - Configuration errors

### Error Handling Strategy

```go
type ErrorHandler interface {
    HandleError(filePath string, err error) ErrorAction
    IsRetryable(err error) bool
    GetRetryDelay(retryCount int, policy *RetryPolicy) time.Duration
}

type ErrorAction int
const (
    ErrorActionRetry ErrorAction = iota
    ErrorActionFail
    ErrorActionPause
    ErrorActionCancel
)
```

## Testing Strategy

### Unit Testing

- Test individual components in isolation
- Mock dependencies for focused testing
- Test error conditions and edge cases
- Verify state transitions and data consistency

### Integration Testing

- Test component interactions
- Verify event flow and delivery
- Test concurrent operations
- Validate persistence and recovery

### Performance Testing

- Test with large numbers of concurrent transfers
- Measure memory usage and resource consumption
- Test transfer rate calculations and accuracy
- Validate system behavior under load

### End-to-End Testing

- Test complete transfer workflows
- Verify UI integration and real-time updates
- Test error recovery and retry mechanisms
- Validate transfer history and persistence

## Implementation Phases

### Phase 1: Core Status Management (Week 1)

- Implement TransferStatus data structure
- Create basic TransferStatusManager
- Add progress tracking and state management
- Implement basic event emission

### Phase 2: Session Management (Week 1-2)

- Implement TransferSession management
- Add concurrent transfer coordination
- Implement overall progress calculation
- Add session-level state management

### Phase 3: Event System (Week 2)

- Implement robust event emission system
- Add event listener management
- Implement asynchronous event delivery
- Add error handling for event failures

### Phase 4: Persistence and History (Week 2-3)

- Implement transfer history storage
- Add history querying and filtering
- Implement history cleanup mechanisms
- Add configuration management

### Phase 5: Error Handling and Recovery (Week 3)

- Implement comprehensive error handling
- Add retry mechanisms with exponential backoff
- Implement transfer recovery after failures
- Add error categorization and routing

### Phase 6: Performance Optimization (Week 3-4)

- Optimize for high-throughput scenarios
- Implement efficient progress calculations
- Add memory usage optimization
- Implement performance monitoring

## Integration Points

### FileTransferManager Integration

- Modify FileTransferManager to use TransferStatusManager
- Add status update calls during chunk transmission
- Integrate error handling with status management
- Coordinate transfer lifecycle events

### WebRTC Connection Integration

- Add connection state monitoring
- Integrate network performance metrics
- Handle connection failures and recovery
- Monitor data channel statistics

### UI Integration

- Provide real-time status updates to UI components
- Support progress bar and status display updates
- Handle user control actions (pause, resume, cancel)
- Display transfer history and statistics

### API Integration

- Expose transfer status through REST API
- Support remote monitoring and control
- Integrate with authentication and authorization
- Provide transfer statistics and reporting

## Security Considerations

### Data Protection

- Ensure transfer status doesn't leak sensitive file information
- Protect transfer history from unauthorized access
- Implement secure event delivery mechanisms
- Validate all input parameters and file paths

### Resource Protection

- Implement rate limiting for status queries
- Prevent resource exhaustion from excessive transfers
- Protect against denial of service attacks
- Implement proper cleanup of abandoned transfers

### Privacy Considerations

- Allow users to control transfer history retention
- Provide options to disable transfer logging
- Ensure transfer metadata doesn't expose private information
- Implement secure deletion of transfer records
